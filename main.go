package main

import (
	"card-manager/internal/app"
	"card-manager/internal/config"
	"embed"
	"io/fs"
	"log/slog"
	"os"

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
	cfg, err := config.Load()
	if err != nil {
		slog.Error("配置加载失败", "error", err)
		os.Exit(1)
	}
	slog.Info("✓ 配置加载完成")

	// 创建应用实例
	application := app.NewApp(cfg)

	// 初始化应用
	if err := application.Initialize(); err != nil {
		slog.Error("应用初始化失败", "error", err)
		os.Exit(1)
	}

	// 首次启动时扫描Tavern哈希，以确保初始加载时导入状态正确
	// TODO: 从原代码迁移scanTavernHashes函数
	// if err := scanTavernHashes(); err != nil {
	// 	slog.Warn("Tavern目录扫描失败", "error", err)
	// } else {
	// 	slog.Info("✓ Tavern目录扫描完成")
	// }

	// 使用 embed.FS 提供静态文件服务
	staticFS, err := fs.Sub(publicFiles, "public")
	if err != nil {
		slog.Error("无法创建静态文件子系统", "error", err)
		os.Exit(1)
	}

	// 设置路由
	application.SetupRoutes(staticFS)

	// 启动服务器
	if err := application.Run(); err != nil {
		slog.Error("启动服务器失败", "error", err)
		os.Exit(1)
	}
}