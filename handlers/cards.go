package handlers

import (
	"encoding/json"
	"fmt"
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
	config *Config
	cache  *CacheManager
}

// NewCardsHandler 创建新的卡片处理器
func NewCardsHandler(config *Config, cache *CacheManager) *CardsHandler {
	return &CardsHandler{
		config: config,
		cache:  cache,
	}
}

// GetCards 获取所有卡片数据
func (h *CardsHandler) GetCards(w http.ResponseWriter, r *http.Request) {
	response, err := h.fetchCardsData()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "获取卡片数据失败", err)
		return
	}
	writeSuccessResponse(w, "获取卡片数据成功", response)
}

// fetchCardsData 获取卡片数据的核心逻辑
func (h *CardsHandler) fetchCardsData() (CardsResponse, error) {
	response := CardsResponse{
		Categories: make(map[string][]Character),
		StrayCards: make([]StrayCard, 0),
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
		response.Categories[categoryName] = make([]Character, 0)
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

// processCharacterDirectory 处理单个角色目录
func (h *CardsHandler) processCharacterDirectory(itemPath string) *Character {
	characterName := filepath.Base(itemPath)
	versions := make([]CardVersion, 0)
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

	// 处理导入信息和本地化状态
	character := &Character{
		Name:              characterName,
		InternalName:      versions[0].InternalName,
		FolderPath:        itemPath,
		LatestVersionPath: versions[0].Path,
		VersionCount:      len(versions),
		Versions:          versions,
		HasNote:           hasNote,
		HasFaceFolder:     hasFaceFolder,
	}

	return character
}

// getCardMetadata 获取卡片元数据
func (h *CardsHandler) getCardMetadata(filePath string) (CacheEntry, error) {
	// 这里应该调用缓存管理器的方法
	// 为了简化，暂时返回空结构
	return CacheEntry{}, nil
}