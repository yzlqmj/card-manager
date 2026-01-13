package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// APIResponse 统一的API响应结构
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// writeJSONResponse 写入JSON响应
func writeJSONResponse(w http.ResponseWriter, response APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("写入响应失败", "error", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
	}
}

// writeSuccessResponse 写入成功响应
func writeSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	writeJSONResponse(w, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// writeErrorResponse 写入错误响应
func writeErrorResponse(w http.ResponseWriter, statusCode int, message string, err error) {
	errorMsg := message
	if err != nil {
		slog.Error(message, "error", err)
		errorMsg = message + ": " + err.Error()
	} else {
		slog.Warn(message)
	}
	
	w.WriteHeader(statusCode)
	writeJSONResponse(w, APIResponse{
		Success: false,
		Error:   errorMsg,
	})
}

// validatePath 验证路径安全性
func validatePath(path string) error {
	if path == "" {
		return ErrEmptyPath
	}
	if !strings.HasPrefix(path, config.CharactersRootPath) {
		return ErrPathNotAllowed
	}
	return nil
}