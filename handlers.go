package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	submittedUrlQueue []string
	queueMutex        sync.Mutex
)

// getFileHash 计算文件的 SHA256 哈希
func getFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// getCardMetadata 高效获取卡片元数据，利用缓存
func getCardMetadata(filePath string) (CacheEntry, error) {
	stats, err := os.Stat(filePath)
	if err != nil {
		return CacheEntry{}, err
	}
	mtime := stats.ModTime().Format(time.RFC3339Nano) // 使用更高精度的时间戳

	// 检查缓存
	cachedData, found := getCache(filePath)
	if found && cachedData.Mtime == mtime {
		return cachedData, nil
	}

	// 缓存未命中或文件已更新，重新解析
	hash, err := getFileHash(filePath)
	if err != nil {
		// 如果无法计算哈希，仍然尝试返回部分数据
		return CacheEntry{Mtime: mtime}, err
	}

	var internalName string
	charaData, err := getInternalCharNameFromPNG(filePath)
	if err == nil {
		decoded, err := base64.StdEncoding.DecodeString(charaData)
		if err == nil {
			var charDataJSON map[string]interface{}
			if json.Unmarshal(decoded, &charDataJSON) == nil {
				if name, ok := charDataJSON["name"].(string); ok && name != "" {
					internalName = name
				} else if name, ok := charDataJSON["char_name"].(string); ok && name != "" {
					internalName = name
				}
			}
		}
	}

	metadata := CacheEntry{
		Hash:         hash,
		InternalName: internalName,
		Mtime:        mtime,
	}
	setCache(filePath, metadata) // 更新缓存
	return metadata, nil
}

// fetchCardsData 是获取卡片数据的核心逻辑
func fetchCardsData() (CardsResponse, error) {
	response := CardsResponse{
		Categories: make(map[string][]Character),
		StrayCards: make([]StrayCard, 0),
	}

	rootDirents, err := os.ReadDir(config.CharactersRootPath)
	if err != nil {
		return response, fmt.Errorf("无法读取角色根目录: %w", err)
	}

	for _, dirent := range rootDirents {
		if !dirent.IsDir() {
			continue
		}

		categoryName := dirent.Name()
		categoryPath := filepath.Join(config.CharactersRootPath, categoryName)
		response.Categories[categoryName] = make([]Character, 0)

		itemDirents, err := os.ReadDir(categoryPath)
		if err != nil {
			continue
		}

		for _, item := range itemDirents {
			itemPath := filepath.Join(categoryPath, item.Name())
			if item.IsDir() {
				characterName := item.Name()
				versions := make([]CardVersion, 0)
				hasNote := false
				hasFaceFolder := false

				versionFiles, err := os.ReadDir(itemPath)
				if err != nil {
					continue
				}

				faceDirPath := filepath.Join(itemPath, "卡面")
				if _, err := os.Stat(faceDirPath); err == nil {
					hasFaceFolder = true
				}

				for _, verFile := range versionFiles {
					if !verFile.IsDir() && strings.HasSuffix(strings.ToLower(verFile.Name()), ".png") {
						verPath := filepath.Join(itemPath, verFile.Name())
						metadata, _ := getCardMetadata(verPath) // 忽略错误，尽力而为
						versions = append(versions, CardVersion{
							Path:         verPath,
							FileName:     verFile.Name(),
							Mtime:        metadata.Mtime,
							InternalName: metadata.InternalName,
						})
					} else if !verFile.IsDir() && strings.ToLower(verFile.Name()) == "note.md" {
						hasNote = true
					}
				}

				if len(versions) > 0 {
					sort.Slice(versions, func(i, j int) bool {
						t1, _ := time.Parse(time.RFC3339Nano, versions[i].Mtime)
						t2, _ := time.Parse(time.RFC3339Nano, versions[j].Mtime)
						return t1.After(t2)
					})

					importInfo := ImportInfo{}
					for i, version := range versions {
						metadata, found := getCache(version.Path)
						if !found {
							continue // 如果没有缓存，无法判断导入状态
						}
						isImported := false
						if metadata.Hash != "" && importedHashes[metadata.Hash] {
							isImported = true
						}
						if !isImported && version.InternalName != "" && importedInternalNames[version.InternalName] {
							isImported = true
						}

						if isImported {
							importInfo.IsImported = true
							importInfo.ImportedVersionPath = version.Path
							importInfo.IsLatestImported = i == 0
							break
						}
					}

					character := Character{
						Name:              characterName,
						InternalName:      versions[0].InternalName,
						FolderPath:        itemPath,
						LatestVersionPath: versions[0].Path,
						VersionCount:      len(versions),
						Versions:          versions,
						HasNote:           hasNote,
						HasFaceFolder:     hasFaceFolder,
						ImportInfo:        importInfo,
					}
					response.Categories[categoryName] = append(response.Categories[categoryName], character)
				}

			} else if strings.HasSuffix(strings.ToLower(item.Name()), ".png") {
				response.StrayCards = append(response.StrayCards, StrayCard{
					FileName: item.Name(),
					Path:     itemPath,
				})
			}
		}
	}
	return response, nil
}

// getCardsHandler 仅从缓存和文件系统获取数据，不执行全量扫描
func getCardsHandler(w http.ResponseWriter, r *http.Request) {
	defer saveCache()
	response, err := fetchCardsData()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// scanChangesHandler 执行全量扫描并返回新数据
func scanChangesHandler(w http.ResponseWriter, r *http.Request) {
	defer saveCache()

	// 只有当缓存为空时，才执行完整的Tavern哈希扫描
	if isCacheEmpty() {
		if err := scanTavernHashes(); err != nil {
			http.Error(w, "扫描 Tavern 目录失败", http.StatusInternalServerError)
			return
		}
	}

	// 重新获取卡片数据（这将利用现有缓存）
	response, err := fetchCardsData()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getImageHandler(w http.ResponseWriter, r *http.Request) {
	imagePath := r.URL.Query().Get("path")
	if imagePath == "" {
		http.Error(w, "缺少路径参数", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(imagePath, config.CharactersRootPath) {
		http.Error(w, "路径非法", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, imagePath)
}

func openFolderHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FolderPath string `json:"folderPath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(body.FolderPath, config.CharactersRootPath) {
		http.Error(w, "路径非法", http.StatusForbidden)
		return
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", body.FolderPath)
	case "darwin":
		cmd = exec.Command("open", body.FolderPath)
	default:
		cmd = exec.Command("xdg-open", body.FolderPath)
	}

	if err := cmd.Run(); err != nil {
		http.Error(w, "无法打开文件夹", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func downloadCardHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL           string `json:"url"`
		Category      string `json:"category"`
		CharacterName string `json:"characterName"`
		FileName      string `json:"fileName"`
		IsFace        bool   `json:"isFace"` // 新增字段
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	client := &http.Client{}
	if config.Proxy != "" {
		proxyURL, err := url.Parse(config.Proxy)
		if err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}

	resp, err := client.Get(body.URL)
	if err != nil {
		http.Error(w, fmt.Sprintf("下载失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var targetFolderPath string
	var finalFileName string
	var successMessage string

	characterFolderPath := filepath.Join(config.CharactersRootPath, body.Category, body.CharacterName)

	if body.IsFace {
		// --- 卡面下载逻辑 ---
		targetFolderPath = filepath.Join(characterFolderPath, "卡面")
		successMessage = "卡面已保存"

		// 从URL中提取原始文件名
		parsedURL, err := url.Parse(body.URL)
		if err != nil {
			http.Error(w, "无效的URL", http.StatusBadRequest)
			return
		}
		originalFileName := filepath.Base(parsedURL.Path)
		finalFileName = originalFileName

	} else {
		// --- 角色卡下载逻辑 ---
		targetFolderPath = characterFolderPath
		successMessage = "角色卡下载成功"
		finalFileName = body.FileName
		if !strings.HasSuffix(strings.ToLower(finalFileName), ".png") {
			finalFileName += ".png"
		}
	}

	if err := os.MkdirAll(targetFolderPath, os.ModePerm); err != nil {
		http.Error(w, "创建目录失败", http.StatusInternalServerError)
		return
	}

	filePath := filepath.Join(targetFolderPath, finalFileName)

	// 处理文件名冲突
	counter := 1
	baseName := strings.TrimSuffix(finalFileName, filepath.Ext(finalFileName))
	extension := filepath.Ext(finalFileName)
	for {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			break
		}
		filePath = filepath.Join(targetFolderPath, fmt.Sprintf("%s_%d%s", baseName, counter, extension))
		counter++
	}

	file, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "创建文件失败", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		http.Error(w, "保存文件失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": fmt.Sprintf("%s: %s", successMessage, filepath.Base(filePath))})
}

func deleteVersionHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FilePath string `json:"filePath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(body.FilePath, config.CharactersRootPath) {
		http.Error(w, "路径非法", http.StatusForbidden)
		return
	}

	if err := os.Remove(body.FilePath); err != nil {
		http.Error(w, "删除文件失败", http.StatusInternalServerError)
		return
	}

	parentDir := filepath.Dir(body.FilePath)
	files, err := os.ReadDir(parentDir)
	if err == nil && len(files) == 0 {
		os.Remove(parentDir)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "文件删除成功"})
}

func moveCharacterHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OldFolderPath string `json:"oldFolderPath"`
		NewCategory   string `json:"newCategory"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(body.OldFolderPath, config.CharactersRootPath) {
		http.Error(w, "路径非法", http.StatusForbidden)
		return
	}

	characterName := filepath.Base(body.OldFolderPath)
	newFolderPath := filepath.Join(config.CharactersRootPath, body.NewCategory, characterName)

	if err := os.Rename(body.OldFolderPath, newFolderPath); err != nil {
		http.Error(w, "移动失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "已移动到分类 " + body.NewCategory})
}

func organizeStrayHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		StrayPath     string `json:"strayPath"`
		Category      string `json:"category"`
		CharacterName string `json:"characterName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(body.StrayPath, config.CharactersRootPath) {
		http.Error(w, "路径非法", http.StatusForbidden)
		return
	}

	newFolderPath := filepath.Join(config.CharactersRootPath, body.Category, body.CharacterName)
	if err := os.MkdirAll(newFolderPath, os.ModePerm); err != nil {
		http.Error(w, "创建目录失败", http.StatusInternalServerError)
		return
	}

	newFilePath := filepath.Join(newFolderPath, filepath.Base(body.StrayPath))
	if err := os.Rename(body.StrayPath, newFilePath); err != nil {
		http.Error(w, "整理失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "文件已整理"})
}

func deleteStrayHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FilePath string `json:"filePath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(body.FilePath, config.CharactersRootPath) {
		http.Error(w, "路径非法", http.StatusForbidden)
		return
	}

	rel, err := filepath.Rel(config.CharactersRootPath, body.FilePath)
	if err != nil || len(strings.Split(rel, string(filepath.Separator))) != 2 {
		http.Error(w, "只能删除待整理目录中的文件", http.StatusForbidden)
		return
	}

	if err := os.Remove(body.FilePath); err != nil {
		http.Error(w, "删除文件失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "文件删除成功"})
}

func getNoteHandler(w http.ResponseWriter, r *http.Request) {
	folderPath := r.URL.Query().Get("folderPath")
	if folderPath == "" {
		http.Error(w, "缺少文件夹路径", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(folderPath, config.CharactersRootPath) {
		http.Error(w, "路径非法", http.StatusForbidden)
		return
	}

	notePath := filepath.Join(folderPath, "note.md")
	content, err := os.ReadFile(notePath)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "content": ""})
			return
		}
		http.Error(w, "读取备注失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "content": string(content)})
}

func saveNoteHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FolderPath string `json:"folderPath"`
		Content    string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(body.FolderPath, config.CharactersRootPath) {
		http.Error(w, "路径非法", http.StatusForbidden)
		return
	}

	notePath := filepath.Join(body.FolderPath, "note.md")
	if err := os.WriteFile(notePath, []byte(body.Content), 0644); err != nil {
		http.Error(w, "保存备注失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "备注已保存"})
}

func getFacesHandler(w http.ResponseWriter, r *http.Request) {
	characterFolderPath := r.URL.Query().Get("characterFolderPath")
	if characterFolderPath == "" {
		http.Error(w, "缺少角色文件夹路径", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(characterFolderPath, config.CharactersRootPath) {
		http.Error(w, "非法的文件夹路径", http.StatusForbidden)
		return
	}

	faceDir := filepath.Join(characterFolderPath, "卡面")
	files, err := os.ReadDir(faceDir)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "faces": []string{}})
			return
		}
		http.Error(w, "获取卡面失败", http.StatusInternalServerError)
		return
	}

	imageFiles := make([]string, 0)
	for _, file := range files {
		if !file.IsDir() {
			fileName := file.Name()
			if strings.HasSuffix(strings.ToLower(fileName), ".jpg") ||
				strings.HasSuffix(strings.ToLower(fileName), ".jpeg") ||
				strings.HasSuffix(strings.ToLower(fileName), ".png") ||
				strings.HasSuffix(strings.ToLower(fileName), ".gif") {
				imageFiles = append(imageFiles, filepath.Join(faceDir, fileName))
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "faces": imageFiles})
}

func submitUrlHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if body.URL != "" {
		queueMutex.Lock()
		submittedUrlQueue = append(submittedUrlQueue, body.URL)
		queueMutex.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "URL received."})
	} else {
		http.Error(w, "No URL provided.", http.StatusBadRequest)
	}
}

func getSubmittedUrlHandler(w http.ResponseWriter, r *http.Request) {
	queueMutex.Lock()
	defer queueMutex.Unlock()

	if len(submittedUrlQueue) > 0 {
		url := submittedUrlQueue[0]
		submittedUrlQueue = submittedUrlQueue[1:]
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "url": url})
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "url": nil})
	}
}

func clearCacheHandler(w http.ResponseWriter, r *http.Request) {
	if err := clearCache(); err != nil {
		http.Error(w, "清除缓存失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "缓存已清除"})
}
