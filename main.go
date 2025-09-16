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

	// 启动剪贴板监听器
	startClipboardListener()

	// 使用 embed.FS 提供静态文件服务
	staticFS, err := fs.Sub(publicFiles, "public")
	if err != nil {
		fmt.Printf("无法创建静态文件子系统: %v\n", err)
		os.Exit(1)
	}
	http.Handle("/", http.FileServer(http.FS(staticFS)))

	// API 路由
	http.HandleFunc("/api/cards", getCardsHandler)
	http.HandleFunc("/api/scan-changes", scanChangesHandler) // 新的扫描端点
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
	http.HandleFunc("/api/download-face", downloadFaceHandler)
	http.HandleFunc("/api/submit-url", submitUrlHandler)
	http.HandleFunc("/api/get-submitted-url", getSubmittedUrlHandler)
	http.HandleFunc("/api/clear-cache", clearCacheHandler)

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
