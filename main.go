package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
)

//go:embed all:public
var publicFiles embed.FS

func main() {
	// 加载配置
	if err := loadConfig(); err != nil {
		fmt.Printf("无法加载配置: %v\n", err)
		os.Exit(1)
	}

	// 加载缓存
	if err := loadCache(); err != nil {
		fmt.Printf("警告: 无法加载缓存文件: %v\n", err)
	}

	// 首次启动时扫描Tavern哈希，以确保初始加载时导入状态正确
	if err := scanTavernHashes(); err != nil {
		fmt.Printf("警告: 启动时扫描Tavern目录失败: %v\n", err)
	}

	// 使用 embed.FS 提供静态文件服务
	staticFS, err := fs.Sub(publicFiles, "public")
	if err != nil {
		fmt.Printf("无法创建静态文件子系统: %v\n", err)
		os.Exit(1)
	}
	http.Handle("/", http.FileServer(http.FS(staticFS)))

	// API 路由
	http.HandleFunc("/api/cards", getCardsHandler)
	http.HandleFunc("/api/scan-changes", scanChangesHandler) // 恢复旧的扫描端点
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

	// 启动服务器
	port := strconv.Itoa(config.Port)
	if port == "0" {
		port = "3000" // 默认端口
	}
	fmt.Printf("Go server running at http://localhost:%s\n", port)
	fmt.Printf("访问 http://localhost:%s/index.html 来使用管理器。\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("启动服务器失败: %v\n", err)
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
