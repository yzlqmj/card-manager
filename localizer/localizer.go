package localizer

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const maxWorkers = 8

var (
	// General URL pattern, excludes newlines
	urlPattern = regexp.MustCompile(`https?://(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,6}(?:[/?#][^\s'"\` + "`" + `<>()]*)?`)
	// CSS url() pattern
	cssUrlPattern = regexp.MustCompile(`url\((?:['"]?)(https?://.*?)(?:['"]?)\)`)
	// JS string pattern
	jsUrlPattern = regexp.MustCompile(`['"\` + "`" + `](https?://[^\'"` + "`" + `\s\n\r]+)['"\` + "`" + `]`)
	// HTML style tag pattern
	styleTagPattern = regexp.MustCompile(`<style[^>]*>([\s\S]*?)</style>`)
)

// resourceExtensions is a whitelist of acceptable resource file extensions.
var resourceExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true, ".svg": true,
	".mp3": true, ".wav": true, ".ogg": true, ".m4a": true, ".flac": true, ".mid": true,
	".mp4": true, ".webm": true, ".mov": true, ".avi": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true,
	".css": true, ".js": true,
	".json": true, ".txt": true,
}

type downloadTask struct {
	URL       string
	LocalPath string
	Proxy     *url.URL
}

type downloadResult struct {
	Task    downloadTask
	Content []byte
	Err     error
}

// Localizer 处理资源查找、下载和替换的逻辑
type Localizer struct {
	cardData         map[string]interface{}
	outputDir        string
	safeCharName     string
	proxy            *url.URL
	successfulURLMap sync.Map // map[string]string, 从原始 URL 到新的 web 路径
	textContentQueue chan map[string]string
	processedURLs    sync.Map // map[string]bool
	wg               sync.WaitGroup
	stopOnce         sync.Once
	stopChan         chan struct{}
	progressCallback func(message string, level string)
}

// NewLocalizer 创建一个新的 Localizer 实例
func NewLocalizer(cardData map[string]interface{}, outputDir string, proxyStr string, progressCallback func(message string, level string)) (*Localizer, error) {
	var proxyURL *url.URL
	var err error
	if proxyStr != "" {
		proxyURL, err = url.Parse(proxyStr)
		if err != nil {
			return nil, fmt.Errorf("无效的代理 URL: %w", err)
		}
	}

	return &Localizer{
		cardData:         cardData,
		outputDir:        outputDir,
		safeCharName:     filepath.Base(outputDir),
		proxy:            proxyURL,
		textContentQueue: make(chan map[string]string, 100),
		stopChan:         make(chan struct{}),
		progressCallback: progressCallback,
	}, nil
}

// Stop 优雅地停止本地化进程
func (l *Localizer) Stop() {
	l.stopOnce.Do(func() {
		close(l.stopChan)
	})
}

// getResourcePaths 为 URL 生成物理存储路径和 Web 访问路径
func (l *Localizer) getResourcePaths(rawURL, context string) (string, string) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", ""
	}
	parsedPath := parsedURL.Path

	var filename, subDirName string

	// HTML/CSS 上下文逻辑
	if context == "html" || context == "css" {
		hash := sha1.Sum([]byte(rawURL))
		urlHash := hex.EncodeToString(hash[:])[:12]
		fileExt := filepath.Ext(parsedPath)
		if fileExt == "" {
			fileExt = ".dat"
		}
		if strings.Contains(rawURL, "googleapis.com/css") {
			fileExt = ".css"
		}
		filename = urlHash + fileExt
		subDirName = "" // HTML/CSS 资源直接放在角色目录下
	} else { // JS 或其他上下文逻辑
		filename = filepath.Base(parsedPath)
		if filename == "" || filename == "." {
			hash := sha1.Sum([]byte(rawURL))
			fileExt := filepath.Ext(parsedPath)
			if fileExt == "" {
				fileExt = ".dat"
			}
			filename = hex.EncodeToString(hash[:])[:12] + fileExt
		}

		fileExt := strings.ToLower(filepath.Ext(filename))
		switch {
		case fileExt == ".png" || fileExt == ".jpg" || fileExt == ".jpeg" || fileExt == ".webp" || fileExt == ".gif" || fileExt == ".svg" || fileExt == ".bmp":
			subDirName = "images"
		case fileExt == ".mp3" || fileExt == ".wav" || fileExt == ".ogg" || fileExt == ".m4a" || fileExt == ".flac" || fileExt == ".mid" || fileExt == ".mp4" || fileExt == ".webm" || fileExt == ".mov" || fileExt == ".avi":
			subDirName = "media"
		default:
			subDirName = "assets"
		}
	}

	localPath := filepath.Join(l.outputDir, subDirName, filename)
	webPath := "/niko/" + l.safeCharName + "/"
	if subDirName != "" {
		webPath += subDirName + "/"
	}
	webPath += filename

	return localPath, webPath
}

// findAndQueueURLs 在文本内容中查找 URL 并将其加入下载队列
func (l *Localizer) findAndQueueURLs(textContent, context string) []downloadTask {
	rawURLs := make(map[string]bool)

	var patterns []*regexp.Regexp
	switch context {
	case "css":
		patterns = append(patterns, cssUrlPattern, urlPattern)
	case "js":
		patterns = append(patterns, jsUrlPattern)
	case "html":
		// 提取 style 标签内容并加入处理队列
		for _, styleContent := range styleTagPattern.FindAllStringSubmatch(textContent, -1) {
			if len(styleContent) > 1 {
				l.textContentQueue <- map[string]string{"content": styleContent[1], "context": "css"}
			}
		}
		patterns = append(patterns, urlPattern)
	default: // json 或其他
		patterns = append(patterns, urlPattern)
	}

	for _, p := range patterns {
		matches := p.FindAllString(textContent, -1)
		for _, match := range matches {
			// 清理特定模式的匹配结果
			if p == cssUrlPattern {
				match = strings.TrimPrefix(match, "url(")
				match = strings.TrimSuffix(match, ")")
				match = strings.Trim(match, `'"`)
			} else if p == jsUrlPattern {
				match = strings.Trim(match, `'"\`+"`")
			}

			// 处理 JSON 中的多行 URL
			subUrls := regexp.MustCompile(`[\n\r]+|\\n`).Split(match, -1)
			for _, subUrl := range subUrls {
				if strings.TrimSpace(subUrl) != "" {
					rawURLs[subUrl] = true
				}
			}
		}
	}

	var tasks []downloadTask
	for urlStr := range rawURLs {
		cleanedURL := strings.TrimSpace(strings.TrimRight(urlStr, `\`))
		if cleanedURL == "" {
			continue
		}

		if _, loaded := l.processedURLs.LoadOrStore(cleanedURL, true); loaded {
			continue
		}

		parsedURL, err := url.Parse(cleanedURL)
		if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
			continue
		}

		fileExt := strings.ToLower(filepath.Ext(parsedURL.Path))
		if !resourceExtensions[fileExt] {
			if !(strings.Contains(cleanedURL, "googleapis.com/css") && (context == "html" || context == "css")) {
				continue
			}
		}

		localPath, _ := l.getResourcePaths(cleanedURL, context)
		if localPath == "" {
			continue
		}

		tasks = append(tasks, downloadTask{
			URL:       cleanedURL,
			LocalPath: localPath,
			Proxy:     l.proxy,
		})
	}
	return tasks
}

// downloadResource 下载单个资源
func downloadResource(task downloadTask) ([]byte, error) {
	if _, err := os.Stat(task.LocalPath); err == nil {
		// 文件已存在，读取它
		content, err := os.ReadFile(task.LocalPath)
		if err == nil {
			return content, nil // 返回已存在的内容
		}
		// 如果读取失败，则继续下载
	}

	if err := os.MkdirAll(filepath.Dir(task.LocalPath), os.ModePerm); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	client := &http.Client{}
	if task.Proxy != nil {
		client.Transport = &http.Transport{Proxy: http.ProxyURL(task.Proxy)}
	}

	resp, err := client.Get(task.URL)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("错误的响应状态: %s", resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	if err := os.WriteFile(task.LocalPath, content, 0644); err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	return content, nil
}

// Localize 开始本地化进程
func (l *Localizer) Localize() (map[string]interface{}, error) {
	// 初始队列
	cardDataBytes, _ := json.Marshal(l.cardData)
	l.textContentQueue <- map[string]string{"content": string(cardDataBytes), "context": "json"}
	l.wg.Add(1) // 为初始内容添加一个计数

	taskChan := make(chan downloadTask, 100)
	resultChan := make(chan downloadResult, 100)

	// 启动下载工作线程
	for i := 0; i < maxWorkers; i++ {
		go func() {
			for task := range taskChan {
				content, err := downloadResource(task)
				resultChan <- downloadResult{Task: task, Content: content, Err: err}
			}
		}()
	}

	// 主处理循环
	go func() {
		for item := range l.textContentQueue {
			select {
			case <-l.stopChan:
				l.wg.Done() // 如果停止，确保计数器递减
				continue
			default:
				context := item["context"]
				tasks := l.findAndQueueURLs(item["content"], context)
				if len(tasks) > 0 {
					l.progressCallback(fmt.Sprintf("在 %s 上下文中发现 %d 个新URL", context, len(tasks)), "info")
					l.wg.Add(len(tasks))
					for _, task := range tasks {
						taskChan <- task
					}
				}
				l.wg.Done()
			}
		}
	}()

	// 结果处理循环
	go func() {
		for result := range resultChan {
			l.handleDownloadResult(result)
			// 下载任务的 wg.Done() 现在在这里
			l.wg.Done()
		}
	}()

	l.wg.Wait()
	close(taskChan)
	close(resultChan)
	close(l.textContentQueue)

	select {
	case <-l.stopChan:
		l.progressCallback("本地化进程已停止。", "warning")
		return nil, fmt.Errorf("本地化被用户停止")
	default:
	}

	l.progressCallback("所有资源下载完毕，正在替换路径...", "info")
	finalData, ok := l.replaceURLsRecursive(l.cardData).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无法将最终数据转换为 map[string]interface{}")
	}
	return finalData, nil
}

func (l *Localizer) handleDownloadResult(result downloadResult) {
	if result.Err != nil {
		l.progressCallback(fmt.Sprintf("[失败] %s - %v", result.Task.URL, result.Err), "failure")
		return
	}

	l.progressCallback(fmt.Sprintf("[成功] %s", result.Task.URL), "success")

	_, webPath := l.getResourcePaths(result.Task.URL, "") // 上下文对于 web 路径生成不重要
	if webPath != "" {
		l.successfulURLMap.Store(result.Task.URL, webPath)
	}

	// 如果下载的文件是文本，则将其添加到队列中进行递归处理
	ext := strings.ToLower(filepath.Ext(result.Task.LocalPath))
	if ext == ".css" || ext == ".js" || ext == ".html" || ext == ".htm" {
		l.wg.Add(1)
		l.textContentQueue <- map[string]string{"content": string(result.Content), "context": strings.TrimPrefix(ext, ".")}
	}
}

// replaceURLsRecursive 递归地替换数据结构中的 URL
func (l *Localizer) replaceURLsRecursive(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		newMap := make(map[string]interface{})
		for key, val := range v {
			newMap[key] = l.replaceURLsRecursive(val)
		}
		return newMap
	case []interface{}:
		newSlice := make([]interface{}, len(v))
		for i, val := range v {
			newSlice[i] = l.replaceURLsRecursive(val)
		}
		return newSlice
	case string:
		// 这是一个简单的替换。对于单个字符串可能包含多个 URL 或其周围有文本的更复杂场景，
		// 需要使用更复杂的、基于正则表达式的替换方法。
		l.successfulURLMap.Range(func(key, value interface{}) bool {
			v = strings.ReplaceAll(v, key.(string), value.(string))
			return true
		})
		return v
	default:
		return data
	}
}
