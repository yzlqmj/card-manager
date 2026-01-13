package clipboard

import (
	"bytes"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

var (
	urlRegex            = regexp.MustCompile(`(https?://cdn\.discordapp\.com/attachments/[^\s]+)`)
	isListenerRunning   atomic.Bool
	stopListenerChannel chan struct{}
)

// Listener 剪贴板监听器
type Listener struct {
	onURLFound func(url string)
}

// NewListener 创建新的剪贴板监听器
func NewListener(onURLFound func(url string)) *Listener {
	return &Listener{
		onURLFound: onURLFound,
	}
}

// Start 启动剪贴板监听
func (l *Listener) Start() {
	if isListenerRunning.CompareAndSwap(false, true) {
		slog.Info("📋 启动剪贴板监听器")
		stopListenerChannel = make(chan struct{})
		go l.runClipboardListener()
		slog.Info("👂 正在监听 Discord 附件链接")
	}
}

// Stop 停止剪贴板监听
func (l *Listener) Stop() {
	if isListenerRunning.CompareAndSwap(true, false) {
		slog.Info("⏹️ 停止剪贴板监听器")
		if stopListenerChannel != nil {
			close(stopListenerChannel)
		}
	}
}

// IsRunning 检查监听器是否正在运行
func (l *Listener) IsRunning() bool {
	return isListenerRunning.Load()
}

// getClipboardContent 获取剪贴板内容
func getClipboardContent() (string, error) {
	cmd := exec.Command("powershell", "-Command", "Get-Clipboard")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// runClipboardListener 运行剪贴板监听循环
func (l *Listener) runClipboardListener() {
	var lastContent string
	ticker := time.NewTicker(200 * time.Millisecond) // Poll every 200ms
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			content, err := getClipboardContent()
			if err != nil {
				continue
			}

			if content != "" && content != lastContent {
				lastContent = content
				matches := urlRegex.FindStringSubmatch(content)
				if len(matches) > 1 && l.onURLFound != nil {
					l.onURLFound(matches[1])
				}
			}
		case <-stopListenerChannel:
			slog.Info("⏹️ 剪贴板监听器已停止")
			return
		}
	}
}