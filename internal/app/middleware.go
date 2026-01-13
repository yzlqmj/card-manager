package app

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// withMiddleware 应用中间件链
func (a *App) withMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return a.loggingMiddleware(
		a.corsMiddleware(
			a.panicRecoveryMiddleware(
				a.pathValidationMiddleware(handler),
			),
		),
	)
}

// loggingMiddleware 请求日志中间件
func (a *App) loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// 创建响应写入器包装器来捕获状态码
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(wrapped, r)
		
		duration := time.Since(start)
		slog.Info("HTTP请求",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration", duration.String(),
		)
	}
}

// panicRecoveryMiddleware panic恢复中间件
func (a *App) panicRecoveryMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("HTTP处理器发生panic", "error", err, "path", r.URL.Path, "method", r.Method)
				http.Error(w, "内部服务器错误", http.StatusInternalServerError)
			}
		}()
		
		next.ServeHTTP(w, r)
	}
}

// corsMiddleware CORS中间件
func (a *App) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	}
}

// pathValidationMiddleware 路径验证中间件
func (a *App) pathValidationMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 对需要路径验证的端点进行检查
		if needsPathValidation(r.URL.Path) {
			if err := a.validateRequestPath(r); err != nil {
				http.Error(w, "路径验证失败: "+err.Error(), http.StatusForbidden)
				return
			}
		}
		
		next.ServeHTTP(w, r)
	}
}

// validateRequestPath 验证请求中的路径参数
func (a *App) validateRequestPath(r *http.Request) error {
	// 从查询参数或请求体中提取路径进行验证
	path := r.URL.Query().Get("path")
	if path == "" {
		path = r.URL.Query().Get("folderPath")
	}
	if path == "" {
		path = r.URL.Query().Get("characterFolderPath")
	}
	
	if path != "" {
		return a.ValidatePath(path)
	}
	
	return nil
}

// ValidatePath 验证路径是否安全
func (a *App) ValidatePath(path string) error {
	if path == "" {
		return nil
	}
	
	// 清理路径并转换为标准格式
	cleanPath := filepath.Clean(path)
	
	// 检查是否包含危险的路径遍历
	if strings.Contains(cleanPath, "..") {
		slog.Warn("❌ 路径验证失败", "原因", "包含非法字符", "路径", cleanPath)
		return fmt.Errorf("路径包含非法字符")
	}
	
	// 将配置中的根目录路径也转换为标准格式进行比较
	rootPath := filepath.Clean(a.Config.CharactersRootPath)
	
	// 检查是否在允许的根目录下
	if !strings.HasPrefix(cleanPath, rootPath) {
		slog.Warn("❌ 路径验证失败", "原因", "不在允许目录", "请求路径", cleanPath, "根目录", rootPath)
		return fmt.Errorf("路径不在允许的目录范围内")
	}
	
	return nil
}

// needsPathValidation 判断端点是否需要路径验证
func needsPathValidation(path string) bool {
	pathValidationEndpoints := []string{
		"/api/image",
		"/api/open-folder",
		"/api/delete-version",
		"/api/move-character",
		"/api/organize-stray",
		"/api/delete-stray",
		"/api/faces",
		"/api/note",
		"/api/list-files",
		"/api/merge-json-to-png",
	}
	
	for _, endpoint := range pathValidationEndpoints {
		if strings.HasPrefix(path, endpoint) {
			return true
		}
	}
	
	return false
}

// responseWriter 包装器用于捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// 实现 Flusher 接口
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}