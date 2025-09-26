package main

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
	urlPattern = regexp.MustCompile(`https?://[^\s'"\` + "`" + ` <>(),\n\r]+`)
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

// Localizer handles the resource finding, downloading, and replacement logic.
type Localizer struct {
	cardData         map[string]interface{}
	outputDir        string
	safeCharName     string
	proxy            *url.URL
	successfulURLMap sync.Map // map[string]string, from original URL to new web path
	textContentQueue chan map[string]string
	processedURLs    sync.Map // map[string]bool
	wg               sync.WaitGroup
	stopOnce         sync.Once
	stopChan         chan struct{}
	progressCallback func(message string, level string)
}

// NewLocalizer creates a new Localizer instance.
func NewLocalizer(cardData map[string]interface{}, outputDir string, proxyStr string, progressCallback func(message string, level string)) (*Localizer, error) {
	var proxyURL *url.URL
	var err error
	if proxyStr != "" {
		proxyURL, err = url.Parse(proxyStr)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
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

// Stop gracefully stops the localization process.
func (l *Localizer) Stop() {
	l.stopOnce.Do(func() {
		close(l.stopChan)
	})
}

// getResourcePaths generates the physical storage path and web access path for a URL.
func (l *Localizer) getResourcePaths(rawURL, context string) (string, string) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", ""
	}
	parsedPath := parsedURL.Path

	var filename, subDirName string

	// Logic from html/css context
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
		subDirName = "" // HTML/CSS resources are placed directly in the character directory
	} else { // Logic from js or other contexts
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

// findAndQueueURLs finds URLs in text content and queues them for download.
func (l *Localizer) findAndQueueURLs(textContent, context string) []downloadTask {
	rawURLs := make(map[string]bool)

	var patterns []*regexp.Regexp
	switch context {
	case "css":
		patterns = append(patterns, cssUrlPattern, urlPattern)
	case "js":
		patterns = append(patterns, jsUrlPattern)
	case "html":
		// Extract style tags and queue them for processing
		for _, styleContent := range styleTagPattern.FindAllStringSubmatch(textContent, -1) {
			if len(styleContent) > 1 {
				l.textContentQueue <- map[string]string{"content": styleContent[1], "context": "css"}
			}
		}
		patterns = append(patterns, urlPattern)
	default: // json or other
		patterns = append(patterns, urlPattern)
	}

	for _, p := range patterns {
		matches := p.FindAllString(textContent, -1)
		for _, match := range matches {
			// Clean up matches from specific patterns
			if p == cssUrlPattern {
				match = strings.TrimPrefix(match, "url(")
				match = strings.TrimSuffix(match, ")")
				match = strings.Trim(match, `'"`)
			} else if p == jsUrlPattern {
				match = strings.Trim(match, `'"\`+"`")
			}

			// Handle multiline URLs in JSON
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

// downloadResource downloads a single resource.
func downloadResource(task downloadTask) ([]byte, error) {
	if _, err := os.Stat(task.LocalPath); err == nil {
		// File exists, read it
		content, err := os.ReadFile(task.LocalPath)
		if err == nil {
			return content, nil // Return existing content
		}
		// If reading fails, proceed to download
	}

	if err := os.MkdirAll(filepath.Dir(task.LocalPath), os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	client := &http.Client{}
	if task.Proxy != nil {
		client.Transport = &http.Transport{Proxy: http.ProxyURL(task.Proxy)}
	}

	resp, err := client.Get(task.URL)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if err := os.WriteFile(task.LocalPath, content, 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return content, nil
}

// Localize starts the localization process.
func (l *Localizer) Localize() (map[string]interface{}, error) {
	// Initial queue
	cardDataBytes, _ := json.Marshal(l.cardData)
	l.textContentQueue <- map[string]string{"content": string(cardDataBytes), "context": "json"}
	l.wg.Add(1) // Add one for the initial content

	taskChan := make(chan downloadTask, 100)
	resultChan := make(chan downloadResult, 100)

	// Start download workers
	for i := 0; i < maxWorkers; i++ {
		go func() {
			for task := range taskChan {
				content, err := downloadResource(task)
				resultChan <- downloadResult{Task: task, Content: content, Err: err}
			}
		}()
	}

	// Main processing loop
	go func() {
		for {
			select {
			case item := <-l.textContentQueue:
				tasks := l.findAndQueueURLs(item["content"], item["context"])
				if len(tasks) > 0 {
					l.progressCallback(fmt.Sprintf("Found %d new URLs in %s context", len(tasks)), "info")
					l.wg.Add(len(tasks))
					for _, task := range tasks {
						taskChan <- task
					}
				}
				l.wg.Done()
			case <-l.stopChan:
				return
			}
		}
	}()

	// Result handling loop
	go func() {
		for result := range resultChan {
			l.handleDownloadResult(result)
			l.wg.Done()
		}
	}()

	l.wg.Wait()
	close(taskChan)
	close(resultChan)
	close(l.textContentQueue)

	select {
	case <-l.stopChan:
		l.progressCallback("Localization stopped.", "warning")
		return nil, fmt.Errorf("localization stopped by user")
	default:
	}

	l.progressCallback("All resources downloaded, replacing paths...", "info")
	finalData, ok := l.replaceURLsRecursive(l.cardData).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to cast final data to map[string]interface{}")
	}
	return finalData, nil
}

func (l *Localizer) handleDownloadResult(result downloadResult) {
	if result.Err != nil {
		l.progressCallback(fmt.Sprintf("[Failure] %s - %v", result.Task.URL, result.Err), "failure")
		return
	}

	l.progressCallback(fmt.Sprintf("[Success] %s", result.Task.URL), "success")

	_, webPath := l.getResourcePaths(result.Task.URL, "") // Context doesn't matter for web path generation
	if webPath != "" {
		l.successfulURLMap.Store(result.Task.URL, webPath)
	}

	// If the downloaded file is text, add it to the queue for recursive processing
	ext := strings.ToLower(filepath.Ext(result.Task.LocalPath))
	if ext == ".css" || ext == ".js" || ext == ".html" || ext == ".htm" {
		l.wg.Add(1)
		l.textContentQueue <- map[string]string{"content": string(result.Content), "context": strings.TrimPrefix(ext, ".")}
	}
}

// replaceURLsRecursive recursively replaces URLs in the data structure.
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
		// This is a simple replacement. For more complex scenarios where a single string
		// might contain multiple URLs or text around them, a more sophisticated approach
		// using regex replacement would be needed.
		l.successfulURLMap.Range(func(key, value interface{}) bool {
			v = strings.ReplaceAll(v, key.(string), value.(string))
			return true
		})
		return v
	default:
		return data
	}
}
