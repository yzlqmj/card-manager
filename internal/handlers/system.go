package handlers

import (
	"card-manager/internal/config"
	"card-manager/internal/models"
	"card-manager/internal/pkg/cache"
	"card-manager/internal/pkg/clipboard"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
)

// SystemHandler 处理系统功能相关的API请求
type SystemHandler struct {
	config             *config.Config
	cacheManager       *cache.Manager
	submittedUrlQueue  []string
	queueMutex         sync.Mutex
	clipboardListener  *clipboard.Listener
}

// NewSystemHandler 创建新的系统处理器
func NewSystemHandler(config *config.Config, cacheManager *cache.Manager) *SystemHandler {
	handler := &SystemHandler{
		config:            config,
		cacheManager:      cacheManager,
		submittedUrlQueue: make([]string, 0),
	}
	
	// 创建剪贴板监听器，当发现URL时添加到队列
	handler.clipboardListener = clipboard.NewListener(func(url string) {
		handler.queueMutex.Lock()
		handler.submittedUrlQueue = append(handler.submittedUrlQueue, url)
		handler.queueMutex.Unlock()
		slog.Info("📎 从剪贴板捕获URL", "url", url)
	})
	
	return handler
}

// ClearCache 清除缓存
func (h *SystemHandler) ClearCache(w http.ResponseWriter, r *http.Request) {
	if err := h.cacheManager.Clear(); err != nil {
		slog.Error("清除缓存失败", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "清除缓存失败", err)
		return
	}
	
	slog.Info("🗑️ 缓存已清除")
	writeSuccessResponse(w, "缓存已清除", nil)
}

// ToggleClipboard 切换剪贴板监听状态
func (h *SystemHandler) ToggleClipboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "方法不允许", nil)
		return
	}

	enableStr := r.URL.Query().Get("enable")
	enable, err := strconv.ParseBool(enableStr)
	if err != nil {
		slog.Warn("无效的 'enable' 参数", "value", enableStr, "error", err)
		writeErrorResponse(w, http.StatusBadRequest, "无效的 'enable' 参数", err)
		return
	}

	if enable {
		h.startClipboardListener()
	} else {
		h.stopClipboardListener()
	}

	status := "stopped"
	if enable {
		status = "started"
	}

	slog.Info("📋 剪贴板监听状态已更改", "状态", status)
	writeSuccessResponse(w, "Clipboard listener "+status, nil)
}

// SubmitUrl 提交URL到队列
func (h *SystemHandler) SubmitUrl(w http.ResponseWriter, r *http.Request) {
	var req models.SubmitUrlRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		handleAppError(w, err.(*models.AppError))
		return
	}
	
	if req.URL != "" {
		h.queueMutex.Lock()
		h.submittedUrlQueue = append(h.submittedUrlQueue, req.URL)
		h.queueMutex.Unlock()
		
		slog.Info("📎 URL已添加到队列", "url", req.URL)
		writeSuccessResponse(w, "URL received.", nil)
	} else {
		writeErrorResponse(w, http.StatusBadRequest, "No URL provided.", nil)
	}
}

// GetSubmittedUrl 从队列获取URL
func (h *SystemHandler) GetSubmittedUrl(w http.ResponseWriter, r *http.Request) {
	h.queueMutex.Lock()
	defer h.queueMutex.Unlock()
	
	if len(h.submittedUrlQueue) > 0 {
		url := h.submittedUrlQueue[0]
		h.submittedUrlQueue = h.submittedUrlQueue[1:]
		
		slog.Info("📎 从队列获取URL", "url", url)
		writeSuccessResponse(w, "URL retrieved from queue", map[string]interface{}{
			"success": true,
			"url":     url,
		})
	} else {
		writeSuccessResponse(w, "No URL in queue", map[string]interface{}{
			"success": false,
			"url":     nil,
		})
	}
}

// startClipboardListener 启动剪贴板监听
func (h *SystemHandler) startClipboardListener() {
	h.clipboardListener.Start()
}

// stopClipboardListener 停止剪贴板监听
func (h *SystemHandler) stopClipboardListener() {
	h.clipboardListener.Stop()
}

// IsClipboardListening 检查剪贴板监听状态
func (h *SystemHandler) IsClipboardListening() bool {
	return h.clipboardListener.IsRunning()
}

// GetQueueLength 获取URL队列长度
func (h *SystemHandler) GetQueueLength() int {
	h.queueMutex.Lock()
	defer h.queueMutex.Unlock()
	return len(h.submittedUrlQueue)
}