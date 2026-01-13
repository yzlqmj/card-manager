package handlers

import (
	"card-manager/internal/config"
	"card-manager/internal/models"
	"card-manager/internal/pkg/cache"
	"card-manager/internal/pkg/tavern"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Handlers 包含所有处理器
type Handlers struct {
	Cards  *CardsHandler
	Files  *FilesHandler
	Tavern *TavernHandler
	System *SystemHandler
}

// NewHandlers 创建新的处理器集合
func NewHandlers(config *config.Config, cacheManager *cache.Manager) *Handlers {
	return &Handlers{
		Cards:  NewCardsHandler(config, cacheManager, nil), // 暂时传nil，稍后更新
		Files:  NewFilesHandler(config, cacheManager),
		Tavern: NewTavernHandler(config, cacheManager),
		System: NewSystemHandler(config, cacheManager),
	}
}

// SetTavernScanner 设置Tavern扫描器（在创建后调用）
func (h *Handlers) SetTavernScanner(scanner *tavern.Scanner) {
	h.Cards.tavernScanner = scanner
}

// writeSuccessResponse 写入成功响应
func writeSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	response := models.APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("编码响应失败", "error", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
	}
}

// writeErrorResponse 写入错误响应
func writeErrorResponse(w http.ResponseWriter, code int, message string, err error) {
	response := models.APIResponse{
		Success: false,
		Message: message,
	}
	
	if err != nil {
		response.Error = err.Error()
		slog.Error(message, "error", err)
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		slog.Error("编码错误响应失败", "error", encodeErr)
	}
}

// handleAppError 处理应用错误
func handleAppError(w http.ResponseWriter, appErr *models.AppError) {
	writeErrorResponse(w, appErr.Code, appErr.Message, appErr.Err)
}

// decodeJSONRequest 解码JSON请求
func decodeJSONRequest(r *http.Request, v interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return models.NewBadRequestError("请求格式无效", err)
	}
	return nil
}