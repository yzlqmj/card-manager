package handlers

import (
	"card-manager/internal/config"
	"card-manager/internal/models"
	"card-manager/internal/pkg/cache"
	"card-manager/internal/pkg/localization"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// TavernHandler 处理Tavern集成相关的API请求
type TavernHandler struct {
	config              *config.Config
	cacheManager        *cache.Manager
	localizationService *localization.Service
}

// NewTavernHandler 创建新的Tavern处理器
func NewTavernHandler(config *config.Config, cacheManager *cache.Manager) *TavernHandler {
	localizationService := localization.NewService(config.TavernPublicPath, config.Proxy)
	return &TavernHandler{
		config:              config,
		cacheManager:        cacheManager,
		localizationService: localizationService,
	}
}

// LocalizeCard 本地化卡片
func (h *TavernHandler) LocalizeCard(w http.ResponseWriter, r *http.Request) {
	var req models.LocalizeCardRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		handleAppError(w, err.(*models.AppError))
		return
	}
	
	if !strings.HasPrefix(req.CardPath, h.config.CharactersRootPath) {
		writeErrorResponse(w, http.StatusForbidden, "路径非法", nil)
		return
	}

	cardPath := req.CardPath
	metadata, found := h.cacheManager.Get(cardPath)
	// 强制重新检查：清除旧的本地化状态
	if found && metadata.LocalizationNeeded != nil {
		slog.Info("发现旧的本地化缓存，清除以强制重新检查", "card", cardPath)
		metadata.LocalizationNeeded = nil
		h.cacheManager.Set(cardPath, metadata)
	}

	// 设置SSE头部
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErrorResponse(w, http.StatusInternalServerError, "流式传输不支持", nil)
		return
	}

	// 发送消息的辅助函数
	sendMessage := func(msgType, content string) {
		fmt.Fprintf(w, "data: {\"type\":\"%s\",\"content\":%q}\n\n", msgType, content)
		flusher.Flush()
	}

	sendMessage("info", "开始本地化检查...")
	slog.Info("开始本地化检查/执行流程", "card", cardPath)
	
	needed, err := h.checkLocalizationNeeded(cardPath)
	if err != nil {
		slog.Error("本地化检查失败", "card", cardPath, "error", err)
		sendMessage("error", fmt.Sprintf("本地化检查失败: %v", err))
		return
	}
	slog.Info("本地化检查完成", "card", cardPath, "needed", needed)

	// 更新缓存
	metadata.LocalizationNeeded = &needed
	h.cacheManager.Set(cardPath, metadata)

	if !needed {
		sendMessage("success", "检查完成：此卡无需本地化。")
		sendMessage("complete", "")
		return
	}

	sendMessage("info", "发现需要本地化的内容，开始执行本地化...")
	slog.Info("开始执行本地化", "card", cardPath)
	
	output, err := h.runLocalizationWithStreaming(cardPath, sendMessage)
	if err != nil {
		slog.Error("本地化过程失败", "card", cardPath, "error", err, "output", output)
		sendMessage("error", fmt.Sprintf("本地化失败: %v", err))
		return
	}

	slog.Info("本地化过程成功", "card", cardPath)
	sendMessage("success", "本地化完成！")
	sendMessage("complete", "")
}

// GetFaces 获取角色的卡面图片
func (h *TavernHandler) GetFaces(w http.ResponseWriter, r *http.Request) {
	characterFolderPath := r.URL.Query().Get("characterFolderPath")
	if characterFolderPath == "" {
		writeErrorResponse(w, http.StatusBadRequest, "缺少角色文件夹路径", nil)
		return
	}
	
	faceDir := filepath.Join(characterFolderPath, "卡面")
	files, err := os.ReadDir(faceDir)
	if err != nil {
		if os.IsNotExist(err) {
			writeSuccessResponse(w, "该角色没有卡面目录", map[string][]string{"faces": {}})
			return
		}
		writeErrorResponse(w, http.StatusInternalServerError, "获取卡面失败", err)
		return
	}
	
	imageFiles := make([]string, 0)
	for _, file := range files {
		if !file.IsDir() {
			fileName := file.Name()
			if h.isImageFile(fileName) {
				imageFiles = append(imageFiles, filepath.Join(faceDir, fileName))
			}
		}
	}
	
	slog.Info("🖼️ 获取卡面列表", "角色", filepath.Base(characterFolderPath), "数量", len(imageFiles))
	writeSuccessResponse(w, fmt.Sprintf("找到 %d 张卡面图片", len(imageFiles)), map[string][]string{"faces": imageFiles})
}

// HandleNote 处理备注的GET和POST请求
func (h *TavernHandler) HandleNote(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.getNote(w, r)
	} else if r.Method == http.MethodPost {
		h.saveNote(w, r)
	} else {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "方法不允许", nil)
	}
}

// getNote 获取角色备注
func (h *TavernHandler) getNote(w http.ResponseWriter, r *http.Request) {
	folderPath := r.URL.Query().Get("folderPath")
	if folderPath == "" {
		writeErrorResponse(w, http.StatusBadRequest, "缺少文件夹路径", nil)
		return
	}
	
	notePath := filepath.Join(folderPath, "note.md")
	content, err := os.ReadFile(notePath)
	if err != nil {
		if os.IsNotExist(err) {
			writeSuccessResponse(w, "备注文件不存在", map[string]string{"content": ""})
			return
		}
		writeErrorResponse(w, http.StatusInternalServerError, "读取备注失败", err)
		return
	}
	
	writeSuccessResponse(w, "备注读取成功", map[string]string{"content": string(content)})
}

// saveNote 保存角色备注
func (h *TavernHandler) saveNote(w http.ResponseWriter, r *http.Request) {
	var req models.SaveNoteRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		handleAppError(w, err.(*models.AppError))
		return
	}
	
	notePath := filepath.Join(req.FolderPath, "note.md")
	if err := os.WriteFile(notePath, []byte(req.Content), 0644); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "保存备注失败", err)
		return
	}
	
	slog.Info("📝 备注已保存", "路径", notePath)
	writeSuccessResponse(w, "备注已保存", nil)
}

// checkLocalizationNeeded 检查是否需要本地化
func (h *TavernHandler) checkLocalizationNeeded(cardPath string) (bool, error) {
	return h.localizationService.CheckLocalizationNeeded(cardPath)
}

// runLocalizationWithStreaming 执行本地化并支持流式输出
func (h *TavernHandler) runLocalizationWithStreaming(cardPath string, sendMessage func(string, string)) (string, error) {
	return h.localizationService.RunLocalizationWithStreaming(cardPath, sendMessage)
}

// isImageFile 检查文件是否为图片文件
func (h *TavernHandler) isImageFile(fileName string) bool {
	lowerName := strings.ToLower(fileName)
	return strings.HasSuffix(lowerName, ".jpg") ||
		strings.HasSuffix(lowerName, ".jpeg") ||
		strings.HasSuffix(lowerName, ".png") ||
		strings.HasSuffix(lowerName, ".gif") ||
		strings.HasSuffix(lowerName, ".webp")
}