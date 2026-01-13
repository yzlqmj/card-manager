package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"net/http"
	"os"
	"strconv"

	"github.com/lmittmann/tint"
)

//go:embed all:public
var publicFiles embed.FS

func main() {
	// 设置简洁的中文日志系统
	logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level:      slog.LevelInfo,
		TimeFormat: "15:04:05",
		NoColor:    false,
	}))
	slog.SetDefault(logger)

	// 加载配置
	if err := loadConfig(); err != nil {
		slog.Error("配置加载失败", "error", err)
		os.Exit(1)
	}
	slog.Info("✓ 配置加载完成")

	// 加载缓存
	if err := loadCache(); err != nil {
		slog.Warn("缓存文件加载失败，将使用空缓存", "error", err)
	} else {
		slog.Info("✓ 缓存加载完成")
	}

	// 首次启动时扫描Tavern哈希，以确保初始加载时导入状态正确
	if err := scanTavernHashes(); err != nil {
		slog.Warn("Tavern目录扫描失败", "error", err)
	} else {
		slog.Info("✓ Tavern目录扫描完成")
	}

	// 使用 embed.FS 提供静态文件服务
	staticFS, err := fs.Sub(publicFiles, "public")
	if err != nil {
		slog.Error("无法创建静态文件子系统", "error", err)
		os.Exit(1)
	}
	http.Handle("/", http.FileServer(http.FS(staticFS)))

	// API 路由
	http.HandleFunc("/api/cards", getCardsHandler)
	http.HandleFunc("/api/scan-changes", scanChangesHandler)
	http.HandleFunc("/api/image", getImageHandler)
	http.HandleFunc("/api/open-folder", openFolderHandler)
	http.HandleFunc("/api/download-card", downloadCardHandler)
	http.HandleFunc("/api/delete-version", deleteVersionHandler)
	http.HandleFunc("/api/move-character", moveCharacterHandler)
	http.HandleFunc("/api/organize-stray", organizeStrayHandler)
	http.HandleFunc("/api/delete-stray", deleteStrayHandler)
	http.HandleFunc("/api/note", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			getNoteHandler(w, r)
		} else if r.Method == http.MethodPost {
			saveNoteHandler(w, r)
		} else {
			http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/api/faces", getFacesHandler)
	http.HandleFunc("/api/submit-url", submitUrlHandler)
	http.HandleFunc("/api/get-submitted-url", getSubmittedUrlHandler)
	http.HandleFunc("/api/clear-cache", clearCacheHandler)
	http.HandleFunc("/api/toggle-clipboard", toggleClipboardHandler)
	http.HandleFunc("/api/localize-card", localizeCardHandler)
	http.HandleFunc("/api/stats", getStatsHandler)
	http.HandleFunc("/api/list-files", listFilesInFolderHandler)
	http.HandleFunc("/api/merge-json-to-png", mergeJsonToPngHandler)

	// 启动服务器
	port := strconv.Itoa(config.Port)
	if port == "0" {
		port = "3000" // 默认端口
	}
	slog.Info("🚀 服务器启动", "地址", fmt.Sprintf("http://localhost:%s", port))
	slog.Info("📋 管理页面", "地址", fmt.Sprintf("http://localhost:%s/index.html", port))
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		slog.Error("启动服务器失败", "error", err)
		os.Exit(1)
	}
}

func toggleClipboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	enableStr := r.URL.Query().Get("enable")
	enable, err := strconv.ParseBool(enableStr)
	if err != nil {
		slog.Warn("无效的 'enable' 参数", "value", enableStr, "error", err)
		http.Error(w, "无效的 'enable' 参数", http.StatusBadRequest)
		return
	}

	if enable {
		startClipboardListener()
	} else {
		stopClipboardListener()
	}

	status := "stopped"
	if enable {
		status = "started"
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "Clipboard listener %s"}`, status)
}
