package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

// getFileHash calculates the SHA256 hash of a file.
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

// getCardMetadata efficiently retrieves card metadata, utilizing a cache.
func getCardMetadata(filePath string) (CacheEntry, error) {
	stats, err := os.Stat(filePath)
	if err != nil {
		return CacheEntry{}, err
	}
	mtime := stats.ModTime().Format(time.RFC3339Nano)

	cachedData, found := getCache(filePath)
	if found && cachedData.Mtime == mtime {
		return cachedData, nil
	}

	hash, err := getFileHash(filePath)
	if err != nil {
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

	setCache(filePath, metadata)
	return metadata, nil
}

// fetchCardsData is the core logic for fetching card data.
func fetchCardsData() (CardsResponse, error) {
	response := CardsResponse{
		Categories: make(map[string][]Character),
		StrayCards: make([]StrayCard, 0),
	}
	var wg sync.WaitGroup
	var mu sync.Mutex

	rootDirents, err := os.ReadDir(config.CharactersRootPath)
	if err != nil {
		slog.Error("无法读取角色根目录", "path", config.CharactersRootPath, "error", err)
		return response, fmt.Errorf("无法读取角色根目录: %w", err)
	}

	for _, dirent := range rootDirents {
		if !dirent.IsDir() {
			continue
		}

		categoryName := dirent.Name()
		categoryPath := filepath.Join(config.CharactersRootPath, categoryName)
		mu.Lock()
		response.Categories[categoryName] = make([]Character, 0)
		mu.Unlock()

		itemDirents, err := os.ReadDir(categoryPath)
		if err != nil {
			slog.Warn("无法读取分类目录", "path", categoryPath, "error", err)
			continue
		}

		for _, item := range itemDirents {
			itemPath := filepath.Join(categoryPath, item.Name())
			if item.IsDir() {
				wg.Add(1)
				go func(itemPath, categoryName string) {
					defer wg.Done()
					character := processCharacterDirectory(itemPath)
					if character != nil {
						mu.Lock()
						response.Categories[categoryName] = append(response.Categories[categoryName], *character)
						mu.Unlock()
					}
				}(itemPath, categoryName)
			} else if strings.HasSuffix(strings.ToLower(item.Name()), ".png") {
				mu.Lock()
				response.StrayCards = append(response.StrayCards, StrayCard{
					FileName: item.Name(),
					Path:     itemPath,
				})
				mu.Unlock()
			}
		}
	}

	wg.Wait()
	return response, nil
}

// processCharacterDirectory processes a single character directory.
func processCharacterDirectory(itemPath string) *Character {
	characterName := filepath.Base(itemPath)
	versions := make([]CardVersion, 0)
	hasNote := false
	hasFaceFolder := false

	versionFiles, err := os.ReadDir(itemPath)
	if err != nil {
		slog.Warn("无法读取角色版本目录", "path", itemPath, "error", err)
		return nil
	}

	faceDirPath := filepath.Join(itemPath, "卡面")
	if _, err := os.Stat(faceDirPath); err == nil {
		hasFaceFolder = true
	}

	for _, verFile := range versionFiles {
		if !verFile.IsDir() && strings.HasSuffix(strings.ToLower(verFile.Name()), ".png") {
			verPath := filepath.Join(itemPath, verFile.Name())
			metadata, _ := getCardMetadata(verPath)
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

	if len(versions) == 0 {
		return nil
	}

	sort.Slice(versions, func(i, j int) bool {
		t1, _ := time.Parse(time.RFC3339Nano, versions[i].Mtime)
		t2, _ := time.Parse(time.RFC3339Nano, versions[j].Mtime)
		return t1.After(t2)
	})

	importInfo := ImportInfo{}
	TavernScanMutex.Lock()
	for i, version := range versions {
		metadata, found := getCache(version.Path)
		if !found {
			continue
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
	TavernScanMutex.Unlock()

	metadata, _ := getCardMetadata(versions[0].Path)
	var localizationNeeded *bool
	if metadata.LocalizationNeeded != nil {
		localizationNeeded = metadata.LocalizationNeeded
	} else {
		needed, err := checkLocalizationNeeded(versions[0].Path)
		if err != nil {
			slog.Warn("自动本地化检查失败", "card", versions[0].Path, "error", err)
		} else {
			if needed {
				cachedData, _ := getCache(versions[0].Path)
				cachedData.LocalizationNeeded = &needed
				setCache(versions[0].Path, cachedData)
			}
			localizationNeeded = &needed
		}
	}

	nameToCheck := versions[0].InternalName
	if nameToCheck == "" {
		nameToCheck = characterName
	}
	isLocalized, _ := isLocalized(nameToCheck)

	return &Character{
		Name:               characterName,
		InternalName:       versions[0].InternalName,
		FolderPath:         itemPath,
		LatestVersionPath:  versions[0].Path,
		VersionCount:       len(versions),
		Versions:           versions,
		HasNote:            hasNote,
		HasFaceFolder:      hasFaceFolder,
		ImportInfo:         importInfo,
		LocalizationNeeded: localizationNeeded,
		IsLocalized:        isLocalized,
	}
}

// getCardsHandler handles the request to get all cards.
func getCardsHandler(w http.ResponseWriter, r *http.Request) {
	defer saveCache()
	response, err := fetchCardsData()
	if err != nil {
		http.Error(w, "获取卡片数据失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// scanChangesHandler handles the request to scan for changes.
func scanChangesHandler(w http.ResponseWriter, r *http.Request) {
	defer saveCache()
	scanTavernHashes()
	response, err := fetchCardsData()
	if err != nil {
		http.Error(w, "扫描变更时获取卡片数据失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getImageHandler serves image files.
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

// openFolderHandler opens a folder in the system's file explorer.
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
		slog.Error("无法打开文件夹", "path", body.FolderPath, "error", err)
		http.Error(w, "无法打开文件夹", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// downloadCardHandler handles downloading a card or a face image.
func downloadCardHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL           string `json:"url"`
		Category      string `json:"category"`
		CharacterName string `json:"characterName"`
		FileName      string `json:"fileName"`
		IsFace        bool   `json:"isFace"`
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
		slog.Error("下载文件失败", "url", body.URL, "error", err)
		http.Error(w, fmt.Sprintf("下载失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var targetFolderPath string
	var finalFileName string
	var successMessage string

	characterFolderPath := filepath.Join(config.CharactersRootPath, body.Category, body.CharacterName)

	if body.IsFace {
		targetFolderPath = filepath.Join(characterFolderPath, "卡面")
		successMessage = "卡面已保存"
		parsedURL, err := url.Parse(body.URL)
		if err != nil {
			http.Error(w, "无效的URL", http.StatusBadRequest)
			return
		}
		finalFileName = filepath.Base(parsedURL.Path)
	} else {
		targetFolderPath = characterFolderPath
		successMessage = "角色卡下载成功"
		finalFileName = body.FileName
		if !strings.HasSuffix(strings.ToLower(finalFileName), ".png") {
			finalFileName += ".png"
		}
	}

	if err := os.MkdirAll(targetFolderPath, os.ModePerm); err != nil {
		slog.Error("创建目录失败", "path", targetFolderPath, "error", err)
		http.Error(w, "创建目录失败", http.StatusInternalServerError)
		return
	}

	filePath := filepath.Join(targetFolderPath, finalFileName)
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
		slog.Error("创建文件失败", "path", filePath, "error", err)
		http.Error(w, "创建文件失败", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		slog.Error("保存文件失败", "path", filePath, "error", err)
		http.Error(w, "保存文件失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": fmt.Sprintf("%s: %s", successMessage, filepath.Base(filePath))})
}

// deleteVersionHandler handles deleting a card version.
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
		slog.Error("删除文件失败", "path", body.FilePath, "error", err)
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

// moveCharacterHandler handles moving a character to a different category.
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
		slog.Error("移动角色失败", "from", body.OldFolderPath, "to", newFolderPath, "error", err)
		http.Error(w, "移动失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "已移动到分类 " + body.NewCategory})
}

// organizeStrayHandler handles organizing a stray card.
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
		slog.Error("创建目录失败", "path", newFolderPath, "error", err)
		http.Error(w, "创建目录失败", http.StatusInternalServerError)
		return
	}
	newFilePath := filepath.Join(newFolderPath, filepath.Base(body.StrayPath))
	if err := os.Rename(body.StrayPath, newFilePath); err != nil {
		slog.Error("整理文件失败", "from", body.StrayPath, "to", newFilePath, "error", err)
		http.Error(w, "整理失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "文件已整理"})
}

// deleteStrayHandler handles deleting a stray card.
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
		slog.Error("删除文件失败", "path", body.FilePath, "error", err)
		http.Error(w, "删除文件失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "文件删除成功"})
}

// getNoteHandler handles getting a note for a character.
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
		slog.Warn("读取备注失败", "path", notePath, "error", err)
		http.Error(w, "读取备注失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "content": string(content)})
}

// saveNoteHandler handles saving a note for a character.
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
		slog.Error("保存备注失败", "path", notePath, "error", err)
		http.Error(w, "保存备注失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "备注已保存"})
}

// getFacesHandler handles getting face images for a character.
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
		slog.Warn("获取卡面失败", "path", faceDir, "error", err)
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

// submitUrlHandler handles submitting a URL to the queue.
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

// getSubmittedUrlHandler gets a URL from the queue.
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

// clearCacheHandler handles clearing the cache.
func clearCacheHandler(w http.ResponseWriter, r *http.Request) {
	if err := clearCache(); err != nil {
		slog.Error("清除缓存失败", "error", err)
		http.Error(w, "清除缓存失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "缓存已清除"})
}

// localizeCardHandler handles the request to localize a card.
func localizeCardHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CardPath string `json:"cardPath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(body.CardPath, config.CharactersRootPath) {
		http.Error(w, "路径非法", http.StatusForbidden)
		return
	}

	cardPath := body.CardPath
	metadata, found := getCache(cardPath)
	// 强制重新检查：清除旧的本地化状态
	if found && metadata.LocalizationNeeded != nil {
		slog.Info("发现旧的本地化缓存，清除以强制重新检查", "card", cardPath)
		metadata.LocalizationNeeded = nil
		setCache(cardPath, metadata)
	}

	slog.Info("开始本地化检查/执行流程", "card", cardPath)
	needed, err := checkLocalizationNeeded(cardPath)
	if err != nil {
		slog.Error("本地化检查失败", "card", cardPath, "error", err)
		http.Error(w, fmt.Sprintf("本地化检查失败: %v", err), http.StatusInternalServerError)
		return
	}
	slog.Info("本地化检查完成", "card", cardPath, "needed", needed)

	// 更新缓存
	metadata, _ = getCardMetadata(cardPath) // 重新获取以包含mtime等最新信息
	metadata.LocalizationNeeded = &needed
	setCache(cardPath, metadata)

	if !needed {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "log": "检查完成：此卡无需本地化。"})
		return
	}

	slog.Info("开始执行本地化", "card", cardPath)
	output, err := runLocalization(cardPath)
	cleanOutput := strings.ToValidUTF8(output, "")

	if err != nil {
		slog.Error("本地化过程失败", "card", cardPath, "error", err, "output", cleanOutput)
		http.Error(w, "本地化失败: "+cleanOutput, http.StatusInternalServerError)
		return
	}

	slog.Info("本地化过程成功", "card", cardPath)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "log": cleanOutput})
}

// getStatsHandler handles getting statistics.
func getStatsHandler(w http.ResponseWriter, r *http.Request) {
	cardsData, err := fetchCardsData()
	if err != nil {
		http.Error(w, "无法获取卡片数据", http.StatusInternalServerError)
		return
	}
	stats := StatsResponse{}
	for _, category := range cardsData.Categories {
		for _, character := range category {
			stats.TotalCharacters++
			if character.LocalizationNeeded != nil && *character.LocalizationNeeded {
				stats.NeedsLocalization++
				if !character.IsLocalized {
					stats.NotLocalized++
				}
			}
			if !character.ImportInfo.IsImported {
				stats.NotImported++
			} else if !character.ImportInfo.IsLatestImported {
				stats.NotLatestImported++
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func listFilesInFolderHandler(w http.ResponseWriter, r *http.Request) {
	folderPath := r.URL.Query().Get("folderPath")
	if folderPath == "" {
		http.Error(w, "缺少文件夹路径", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(folderPath, config.CharactersRootPath) {
		http.Error(w, "路径非法", http.StatusForbidden)
		return
	}

	files, err := os.ReadDir(folderPath)
	if err != nil {
		slog.Error("无法读取文件夹内容", "path", folderPath, "error", err)
		http.Error(w, "无法读取文件夹内容", http.StatusInternalServerError)
		return
	}

	var jsonFiles []string
	var pngFiles []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileName := file.Name()
		if strings.HasSuffix(strings.ToLower(fileName), ".json") {
			jsonFiles = append(jsonFiles, fileName)
		} else if strings.HasSuffix(strings.ToLower(fileName), ".png") {
			pngFiles = append(pngFiles, fileName)
		}
	}

	response := map[string][]string{
		"jsonFiles": jsonFiles,
		"pngFiles":  pngFiles,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func mergeJsonToPngHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FolderPath   string `json:"folderPath"`
		JsonFileName string `json:"jsonFileName"`
		PngFileName  string `json:"pngFileName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	jsonPath := filepath.Join(body.FolderPath, body.JsonFileName)
	pngPath := filepath.Join(body.FolderPath, body.PngFileName)

	// 简单的安全检查
	if !strings.HasPrefix(jsonPath, config.CharactersRootPath) || !strings.HasPrefix(pngPath, config.CharactersRootPath) {
		http.Error(w, "路径非法", http.StatusForbidden)
		return
	}

	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		slog.Error("读取 JSON 文件失败", "path", jsonPath, "error", err)
		http.Error(w, "读取 JSON 文件失败", http.StatusInternalServerError)
		return
	}

	// 将 JSON 数据编码为 Base64
	charaData := base64.StdEncoding.EncodeToString(jsonData)

	// 定义输出文件名
	outputFileName := strings.TrimSuffix(body.PngFileName, filepath.Ext(body.PngFileName)) + "_merged.png"
	outputPath := filepath.Join(body.FolderPath, outputFileName)

	// 调用一个通用的写入函数 (我们将在 png_utils.go 中创建)
	err = WriteCharaToPNG(pngPath, outputPath, charaData)
	if err != nil {
		slog.Error("合并 JSON 到 PNG 失败", "error", err)
		http.Error(w, fmt.Sprintf("合并失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "合并成功！新文件已保存为: " + outputFileName})
}
