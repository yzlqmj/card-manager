package app

import (
	"card-manager/internal/config"
	"card-manager/internal/handlers"
	"card-manager/internal/pkg/cache"
	"card-manager/internal/pkg/tavern"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
)

// App 应用程序结构体，包含所有依赖
type App struct {
	Config        *config.Config
	CacheManager  *cache.Manager
	Handlers      *handlers.Handlers
	TavernScanner *tavern.Scanner
}

// NewApp 创建新的应用实例
func NewApp(cfg *config.Config) *App {
	// 初始化缓存管理器
	cacheManager := cache.NewManager("cache.json")

	// 初始化Tavern扫描器
	tavernScanner := tavern.NewScanner(cfg.TavernCharactersPath)

	// 初始化处理器
	handlers := handlers.NewHandlers(cfg, cacheManager)
	
	// 设置Tavern扫描器
	handlers.SetTavernScanner(tavernScanner)

	return &App{
		Config:        cfg,
		CacheManager:  cacheManager,
		Handlers:      handlers,
		TavernScanner: tavernScanner,
	}
}

// Initialize 初始化应用
func (a *App) Initialize() error {
	// 加载缓存
	if err := a.CacheManager.Load(); err != nil {
		slog.Warn("缓存文件加载失败，将使用空缓存", "error", err)
	} else {
		slog.Info("✓ 缓存加载完成")
	}

	// 扫描Tavern哈希
	if err := a.TavernScanner.ScanHashes(); err != nil {
		slog.Warn("Tavern目录扫描失败", "error", err)
	} else {
		slog.Info("✓ Tavern目录扫描完成")
	}

	return nil
}

// SetupRoutes 设置路由
func (a *App) SetupRoutes(staticFS fs.FS) {
	// 静态文件服务
	http.Handle("/", http.FileServer(http.FS(staticFS)))

	// 直接注册API路由到默认ServeMux
	// 卡片管理相关路由
	http.HandleFunc("/api/cards", a.withMiddleware(a.Handlers.Cards.GetCards))
	http.HandleFunc("/api/scan-changes", a.withMiddleware(a.Handlers.Cards.ScanChanges))
	http.HandleFunc("/api/stats", a.withMiddleware(a.Handlers.Cards.GetStats))
	
	// 文件操作相关路由
	http.HandleFunc("/api/image", a.withMiddleware(a.Handlers.Files.GetImage))
	http.HandleFunc("/api/open-folder", a.withMiddleware(a.Handlers.Files.OpenFolder))
	http.HandleFunc("/api/download-card", a.withMiddleware(a.Handlers.Files.DownloadCard))
	http.HandleFunc("/api/delete-version", a.withMiddleware(a.Handlers.Files.DeleteVersion))
	http.HandleFunc("/api/move-character", a.withMiddleware(a.Handlers.Files.MoveCharacter))
	http.HandleFunc("/api/organize-stray", a.withMiddleware(a.Handlers.Files.OrganizeStray))
	http.HandleFunc("/api/delete-stray", a.withMiddleware(a.Handlers.Files.DeleteStray))
	http.HandleFunc("/api/list-files", a.withMiddleware(a.Handlers.Files.ListFiles))
	http.HandleFunc("/api/merge-json-to-png", a.withMiddleware(a.Handlers.Files.MergeJsonToPng))
	
	// Tavern集成相关路由
	http.HandleFunc("/api/localize-card", a.withMiddleware(a.Handlers.Tavern.LocalizeCard))
	http.HandleFunc("/api/faces", a.withMiddleware(a.Handlers.Tavern.GetFaces))
	http.HandleFunc("/api/note", a.withMiddleware(a.Handlers.Tavern.HandleNote))
	
	// 系统功能相关路由
	http.HandleFunc("/api/clear-cache", a.withMiddleware(a.Handlers.System.ClearCache))
	http.HandleFunc("/api/toggle-clipboard", a.withMiddleware(a.Handlers.System.ToggleClipboard))
	http.HandleFunc("/api/submit-url", a.withMiddleware(a.Handlers.System.SubmitUrl))
	http.HandleFunc("/api/get-submitted-url", a.withMiddleware(a.Handlers.System.GetSubmittedUrl))
}

// Run 启动应用
func (a *App) Run() error {
	port := strconv.Itoa(a.Config.Port)
	if port == "0" {
		port = "3000" // 默认端口
	}
	
	slog.Info("🚀 服务器启动", "地址", fmt.Sprintf("http://localhost:%s", port))
	slog.Info("📋 管理页面", "地址", fmt.Sprintf("http://localhost:%s/index.html", port))
	
	return http.ListenAndServe(":"+port, nil)
}