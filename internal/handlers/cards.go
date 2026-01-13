package handlers

import (
	"card-manager/internal/config"
	"card-manager/internal/models"
	"card-manager/internal/pkg/cache"
	"card-manager/internal/pkg/localization"
	"card-manager/internal/pkg/png"
	"card-manager/internal/pkg/tavern"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// CardsHandler 处理卡片相关的API请求
type CardsHandler struct {
	config        *config.Config
	cacheManager  *cache.Manager
	tavernScanner *tavern.Scanner
}

// NewCardsHandler 创建新的卡片处理器
func NewCardsHandler(config *config.Config, cacheManager *cache.Manager, tavernScanner *tavern.Scanner) *CardsHandler {
	return &CardsHandler{
		config:        config,
		cacheManager:  cacheManager,
		tavernScanner: tavernScanner,
	}
}

// GetCards 获取所有卡片数据
func (h *CardsHandler) GetCards(w http.ResponseWriter, r *http.Request) {
	defer h.cacheManager.Save()
	
	response, err := h.fetchCardsData()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "获取卡片数据失败", err)
		return
	}
	
	writeSuccessResponse(w, "获取卡片数据成功", response)
}

// ScanChanges 扫描变更并获取卡片数据
func (h *CardsHandler) ScanChanges(w http.ResponseWriter, r *http.Request) {
	defer h.cacheManager.Save()
	
	// 扫描Tavern哈希
	if h.tavernScanner != nil {
		if err := h.tavernScanner.ScanHashes(); err != nil {
			slog.Warn("Tavern目录扫描失败", "error", err)
		}
	}
	
	response, err := h.fetchCardsData()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "扫描变更时获取卡片数据失败", err)
		return
	}
	
	writeSuccessResponse(w, "扫描变更完成", response)
}

// GetStats 获取统计信息
func (h *CardsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	cardsData, err := h.fetchCardsData()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "无法获取卡片数据", err)
		return
	}
	
	stats := models.StatsResponse{}
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
	
	writeSuccessResponse(w, "获取统计信息成功", stats)
}

// fetchCardsData 获取卡片数据的核心逻辑
func (h *CardsHandler) fetchCardsData() (models.CardsResponse, error) {
	response := models.CardsResponse{
		Categories: make(map[string][]models.Character),
		StrayCards: make([]models.StrayCard, 0),
	}
	var wg sync.WaitGroup
	var mu sync.Mutex

	rootDirents, err := os.ReadDir(h.config.CharactersRootPath)
	if err != nil {
		slog.Error("📂 无法读取角色根目录", "路径", h.config.CharactersRootPath, "error", err)
		return response, fmt.Errorf("无法读取角色根目录: %w", err)
	}

	for _, dirent := range rootDirents {
		if !dirent.IsDir() {
			continue
		}

		categoryName := dirent.Name()
		categoryPath := filepath.Join(h.config.CharactersRootPath, categoryName)
		mu.Lock()
		response.Categories[categoryName] = make([]models.Character, 0)
		mu.Unlock()

		itemDirents, err := os.ReadDir(categoryPath)
		if err != nil {
			slog.Warn("📂 无法读取分类目录", "路径", categoryPath, "error", err)
			continue
		}

		for _, item := range itemDirents {
			itemPath := filepath.Join(categoryPath, item.Name())
			if item.IsDir() {
				wg.Add(1)
				go func(itemPath, categoryName string) {
					defer wg.Done()
					character := h.processCharacterDirectory(itemPath)
					if character != nil {
						mu.Lock()
						response.Categories[categoryName] = append(response.Categories[categoryName], *character)
						mu.Unlock()
					}
				}(itemPath, categoryName)
			} else if strings.HasSuffix(strings.ToLower(item.Name()), ".png") {
				mu.Lock()
				response.StrayCards = append(response.StrayCards, models.StrayCard{
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

// processCharacterDirectory 处理单个角色目录
func (h *CardsHandler) processCharacterDirectory(itemPath string) *models.Character {
	characterName := filepath.Base(itemPath)
	versions := make([]models.CardVersion, 0)
	hasNote := false
	hasFaceFolder := false

	versionFiles, err := os.ReadDir(itemPath)
	if err != nil {
		slog.Warn("📂 无法读取角色版本目录", "路径", itemPath, "error", err)
		return nil
	}

	faceDirPath := filepath.Join(itemPath, "卡面")
	if _, err := os.Stat(faceDirPath); err == nil {
		hasFaceFolder = true
	}

	for _, verFile := range versionFiles {
		if !verFile.IsDir() && strings.HasSuffix(strings.ToLower(verFile.Name()), ".png") {
			verPath := filepath.Join(itemPath, verFile.Name())
			metadata, _ := h.getCardMetadata(verPath)
			versions = append(versions, models.CardVersion{
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

	// 处理导入信息和本地化状态
	importInfo := models.ImportInfo{}
	if h.tavernScanner != nil {
		for i, version := range versions {
			metadata, found := h.cacheManager.Get(version.Path)
			if !found {
				continue
			}
			isImported := false
			if metadata.Hash != "" && h.tavernScanner.IsHashImported(metadata.Hash) {
				isImported = true
			}
			if !isImported && version.InternalName != "" && h.tavernScanner.IsInternalNameImported(version.InternalName) {
				isImported = true
			}

			if isImported {
				importInfo.IsImported = true
				importInfo.ImportedVersionPath = version.Path
				importInfo.IsLatestImported = i == 0
				break
			}
		}
	}
	
	metadata, _ := h.getCardMetadata(versions[0].Path)
	var localizationNeeded *bool
	if metadata.LocalizationNeeded != nil {
		localizationNeeded = metadata.LocalizationNeeded
	} else {
		// 如果缓存中没有本地化状态，进行检查
		needed, err := h.checkLocalizationNeeded(versions[0].Path)
		if err != nil {
			slog.Warn("检查本地化状态失败", "path", versions[0].Path, "error", err)
			// 如果检查失败，设置为不需要本地化
			needed = false
		}
		localizationNeeded = &needed
		// 更新缓存
		metadata.LocalizationNeeded = localizationNeeded
		h.cacheManager.Set(versions[0].Path, metadata)
	}

	nameToCheck := versions[0].InternalName
	if nameToCheck == "" {
		nameToCheck = characterName
	}
	
	// 检查是否已经本地化
	localizationService := localization.NewService(h.config.TavernPublicPath, h.config.NikoPath, h.config.Proxy)
	isLocalized, err := localizationService.IsLocalized(nameToCheck)
	if err != nil {
		slog.Warn("检查本地化完成状态失败", "character", nameToCheck, "error", err)
		isLocalized = false
	}

	return &models.Character{
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

// getCardMetadata 获取卡片元数据
func (h *CardsHandler) getCardMetadata(filePath string) (cache.Entry, error) {
	stats, err := os.Stat(filePath)
	if err != nil {
		return cache.Entry{}, err
	}
	mtime := stats.ModTime().Format(time.RFC3339Nano)

	cachedData, found := h.cacheManager.Get(filePath)
	if found && cachedData.Mtime == mtime {
		return cachedData, nil
	}

	hash, err := h.getFileHash(filePath)
	if err != nil {
		return cache.Entry{Mtime: mtime}, err
	}

	var internalName string
	charaData, err := h.getInternalCharNameFromPNG(filePath)
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

	metadata := cache.Entry{
		Hash:         hash,
		InternalName: internalName,
		Mtime:        mtime,
	}

	h.cacheManager.Set(filePath, metadata)
	return metadata, nil
}

// getFileHash 计算文件的SHA256哈希
func (h *CardsHandler) getFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// getInternalCharNameFromPNG 从PNG文件中提取角色数据
func (h *CardsHandler) getInternalCharNameFromPNG(filePath string) (string, error) {
	return png.GetInternalCharNameFromPNG(filePath)
}

// checkLocalizationNeeded 检查是否需要本地化
func (h *CardsHandler) checkLocalizationNeeded(cardPath string) (bool, error) {
	// 创建一个临时的本地化服务来检查
	localizationService := localization.NewService(h.config.TavernPublicPath, h.config.NikoPath, h.config.Proxy)
	return localizationService.CheckLocalizationNeeded(cardPath)
}