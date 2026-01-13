package handlers

import (
	"card-manager/internal/config"
	"card-manager/internal/models"
	"card-manager/internal/pkg/cache"
	"card-manager/internal/pkg/png"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// FilesHandler 处理文件操作相关的API请求
type FilesHandler struct {
	config       *config.Config
	cacheManager *cache.Manager
}

// NewFilesHandler 创建新的文件处理器
func NewFilesHandler(config *config.Config, cacheManager *cache.Manager) *FilesHandler {
	return &FilesHandler{
		config:       config,
		cacheManager: cacheManager,
	}
}

// 提供图片文件服务
func (h *FilesHandler) GetImage(w http.ResponseWriter, r *http.Request) {
	imagePath := r.URL.Query().Get("path")
	if imagePath == "" {
		writeErrorResponse(w, http.StatusBadRequest, "缺少路径参数", nil)
		return
	}
	
	// 路径验证已经在中间件中完成，这里不需要重复检查
	// 但为了安全起见，我们仍然进行标准化比较
	cleanImagePath := filepath.Clean(imagePath)
	cleanRootPath := filepath.Clean(h.config.CharactersRootPath)
	
	if !strings.HasPrefix(cleanImagePath, cleanRootPath) {
		slog.Warn("图片路径验证失败", "请求路径", cleanImagePath, "根目录", cleanRootPath)
		writeErrorResponse(w, http.StatusForbidden, "路径非法", nil)
		return
	}
	
	http.ServeFile(w, r, imagePath)
}

// OpenFolder 在系统文件管理器中打开文件夹
func (h *FilesHandler) OpenFolder(w http.ResponseWriter, r *http.Request) {
	var req models.OpenFolderRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		handleAppError(w, err.(*models.AppError))
		return
	}
	
	// 检查路径是否存在
	if _, err := os.Stat(req.FolderPath); err != nil {
		writeErrorResponse(w, http.StatusNotFound, "文件夹不存在", err)
		return
	}
	
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", req.FolderPath)
	case "darwin":
		cmd = exec.Command("open", req.FolderPath)
	default:
		cmd = exec.Command("xdg-open", req.FolderPath)
	}
	
	if err := cmd.Start(); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "启动文件管理器失败", err)
		return
	}
	
	slog.Info("📁 文件夹已打开", "路径", req.FolderPath)
	writeSuccessResponse(w, "文件夹已成功打开", nil)
}

// DownloadCard 下载角色卡或卡面图片
func (h *FilesHandler) DownloadCard(w http.ResponseWriter, r *http.Request) {
	var req models.DownloadCardRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		handleAppError(w, err.(*models.AppError))
		return
	}

	client := &http.Client{}
	if h.config.Proxy != "" {
		proxyURL, err := url.Parse(h.config.Proxy)
		if err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}

	resp, err := client.Get(req.URL)
	if err != nil {
		slog.Error("下载文件失败", "url", req.URL, "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("下载失败: %v", err), err)
		return
	}
	defer resp.Body.Close()

	var targetFolderPath string
	var finalFileName string
	var successMessage string

	characterFolderPath := filepath.Join(h.config.CharactersRootPath, req.Category, req.CharacterName)

	if req.IsFace {
		targetFolderPath = filepath.Join(characterFolderPath, "卡面")
		successMessage = "卡面已保存"
		parsedURL, err := url.Parse(req.URL)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "无效的URL", err)
			return
		}
		finalFileName = filepath.Base(parsedURL.Path)
	} else {
		targetFolderPath = characterFolderPath
		successMessage = "角色卡下载成功"
		finalFileName = req.FileName
		if !strings.HasSuffix(strings.ToLower(finalFileName), ".png") {
			finalFileName += ".png"
		}
	}

	if err := os.MkdirAll(targetFolderPath, os.ModePerm); err != nil {
		slog.Error("创建目录失败", "path", targetFolderPath, "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "创建目录失败", err)
		return
	}

	filePath := filepath.Join(targetFolderPath, finalFileName)
	counter := 1
	baseName := strings.TrimSuffix(finalFileName, filepath.Ext(finalFileName))
	extension := filepath.Ext(finalFileName)
	for {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			break
		}
		filePath = filepath.Join(targetFolderPath, fmt.Sprintf("%s_%d%s", baseName, counter, extension))
		counter++
	}

	file, err := os.Create(filePath)
	if err != nil {
		slog.Error("创建文件失败", "path", filePath, "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "创建文件失败", err)
		return
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		slog.Error("保存文件失败", "path", filePath, "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "保存文件失败", err)
		return
	}

	slog.Info("📥 文件下载完成", "文件", filepath.Base(filePath), "大小", fmt.Sprintf("%.2f KB", float64(resp.ContentLength)/1024))
	writeSuccessResponse(w, fmt.Sprintf("%s: %s", successMessage, filepath.Base(filePath)), nil)
}

// DeleteVersion 删除卡片版本
func (h *FilesHandler) DeleteVersion(w http.ResponseWriter, r *http.Request) {
	var req models.DeleteVersionRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		handleAppError(w, err.(*models.AppError))
		return
	}
	
	fileName := filepath.Base(req.FilePath)
	if err := os.Remove(req.FilePath); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "删除文件失败", err)
		return
	}
	
	// 检查父目录是否为空，如果为空则删除
	parentDir := filepath.Dir(req.FilePath)
	files, err := os.ReadDir(parentDir)
	if err == nil && len(files) == 0 {
		if err := os.Remove(parentDir); err != nil {
			slog.Warn("删除空目录失败", "目录", parentDir, "error", err)
		} else {
			slog.Info("🗑️ 空目录已清理", "目录", filepath.Base(parentDir))
		}
	}
	
	slog.Info("🗑️ 文件已删除", "文件", fileName)
	writeSuccessResponse(w, fmt.Sprintf("文件 %s 已成功删除", fileName), nil)
}

// MoveCharacter 移动角色到不同分类
func (h *FilesHandler) MoveCharacter(w http.ResponseWriter, r *http.Request) {
	var req models.MoveCharacterRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		handleAppError(w, err.(*models.AppError))
		return
	}
	
	characterName := filepath.Base(req.OldFolderPath)
	newFolderPath := filepath.Join(h.config.CharactersRootPath, req.NewCategory, characterName)
	
	// 确保目标分类目录存在
	categoryPath := filepath.Join(h.config.CharactersRootPath, req.NewCategory)
	if err := os.MkdirAll(categoryPath, 0755); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "创建分类目录失败", err)
		return
	}
	
	if err := os.Rename(req.OldFolderPath, newFolderPath); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "移动角色失败", err)
		return
	}
	
	slog.Info("📦 角色已移动", "角色", characterName, "从", filepath.Base(filepath.Dir(req.OldFolderPath)), "到", req.NewCategory)
	writeSuccessResponse(w, fmt.Sprintf("角色 %s 已成功移动到 %s 分类", characterName, req.NewCategory), nil)
}

// OrganizeStray 整理待整理的卡片
func (h *FilesHandler) OrganizeStray(w http.ResponseWriter, r *http.Request) {
	var req models.OrganizeStrayRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		handleAppError(w, err.(*models.AppError))
		return
	}
	
	newFolderPath := filepath.Join(h.config.CharactersRootPath, req.Category, req.CharacterName)
	if err := os.MkdirAll(newFolderPath, 0755); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "创建角色目录失败", err)
		return
	}
	
	newFilePath := filepath.Join(newFolderPath, filepath.Base(req.StrayPath))
	if err := os.Rename(req.StrayPath, newFilePath); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "整理文件失败", err)
		return
	}
	
	slog.Info("📋 卡片已整理", "文件", filepath.Base(req.StrayPath), "角色", req.CharacterName, "分类", req.Category)
	writeSuccessResponse(w, fmt.Sprintf("卡片已成功整理到 %s/%s", req.Category, req.CharacterName), nil)
}

// DeleteStray 删除待整理的卡片
func (h *FilesHandler) DeleteStray(w http.ResponseWriter, r *http.Request) {
	var req models.DeleteVersionRequest // 复用相同的结构
	if err := decodeJSONRequest(r, &req); err != nil {
		handleAppError(w, err.(*models.AppError))
		return
	}
	
	// 验证是否为待整理目录中的文件
	rel, err := filepath.Rel(h.config.CharactersRootPath, req.FilePath)
	if err != nil || len(strings.Split(rel, string(filepath.Separator))) != 2 {
		writeErrorResponse(w, http.StatusForbidden, "只能删除待整理目录中的文件", nil)
		return
	}
	
	fileName := filepath.Base(req.FilePath)
	if err := os.Remove(req.FilePath); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "删除文件失败", err)
		return
	}
	
	slog.Info("🗑️ 待整理文件已删除", "文件", fileName)
	writeSuccessResponse(w, fmt.Sprintf("待整理文件 %s 已成功删除", fileName), nil)
}

// ListFiles 列出文件夹中的文件
func (h *FilesHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	folderPath := r.URL.Query().Get("folderPath")
	if folderPath == "" {
		writeErrorResponse(w, http.StatusBadRequest, "缺少文件夹路径", nil)
		return
	}
	
	if !strings.HasPrefix(folderPath, h.config.CharactersRootPath) {
		writeErrorResponse(w, http.StatusForbidden, "路径非法", nil)
		return
	}

	files, err := os.ReadDir(folderPath)
	if err != nil {
		slog.Error("无法读取文件夹内容", "path", folderPath, "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "无法读取文件夹内容", err)
		return
	}

	var jsonFiles []string
	var pngFiles []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileName := file.Name()
		if strings.HasSuffix(strings.ToLower(fileName), ".json") {
			jsonFiles = append(jsonFiles, fileName)
		} else if strings.HasSuffix(strings.ToLower(fileName), ".png") {
			pngFiles = append(pngFiles, fileName)
		}
	}

	response := map[string][]string{
		"jsonFiles": jsonFiles,
		"pngFiles":  pngFiles,
	}
	
	writeSuccessResponse(w, "文件列表获取成功", response)
}

// MergeJsonToPng 合并JSON到PNG文件
func (h *FilesHandler) MergeJsonToPng(w http.ResponseWriter, r *http.Request) {
	var req models.MergeJsonToPngRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		handleAppError(w, err.(*models.AppError))
		return
	}

	jsonPath := filepath.Join(req.FolderPath, req.JsonFileName)
	pngPath := filepath.Join(req.FolderPath, req.PngFileName)

	// 安全检查
	if !strings.HasPrefix(jsonPath, h.config.CharactersRootPath) || !strings.HasPrefix(pngPath, h.config.CharactersRootPath) {
		writeErrorResponse(w, http.StatusForbidden, "路径非法", nil)
		return
	}

	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		slog.Error("读取 JSON 文件失败", "path", jsonPath, "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "读取 JSON 文件失败", err)
		return
	}

	// 将 JSON 数据编码为 Base64
	charaData := base64.StdEncoding.EncodeToString(jsonData)

	// 定义输出文件名
	outputFileName := strings.TrimSuffix(req.PngFileName, filepath.Ext(req.PngFileName)) + "_merged.png"
	outputPath := filepath.Join(req.FolderPath, outputFileName)

	// 调用PNG工具函数合并数据
	err = png.WriteCharaToPNG(pngPath, outputPath, charaData)
	if err != nil {
		slog.Error("合并 JSON 到 PNG 失败", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("合并失败: %v", err), err)
		return
	}

	writeSuccessResponse(w, "合并成功！新文件已保存为: "+outputFileName, nil)
}