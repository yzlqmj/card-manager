package localization

import (
	"card-manager/localizer"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// 本地化服务 - 负责处理角色卡的本地化操作
type Service struct {
	// 酒馆公共目录路径
	tavernPublicPath string
	// 代理服务器地址
	proxy            string
}

// 创建新的本地化服务
func NewService(tavernPublicPath, proxy string) *Service {
	return &Service{
		tavernPublicPath: tavernPublicPath,
		proxy:            proxy,
	}
}

// 构建niko目录路径
func (s *Service) buildNikoPath() string {
	return filepath.Join(s.tavernPublicPath, "niko")
}

// 调用 localizer.Run 来判断角色卡是否需要本地化
func (s *Service) CheckLocalizationNeeded(cardPath string) (bool, error) {
	opts := localizer.Options{
		CardPath:    cardPath,
		IsCheckMode: true,
	}
	// 直接调用新的 Run 函数
	needed, logOutput, err := localizer.Run(opts)
	if err != nil {
		// 即使有错，也记录日志
		return false, fmt.Errorf("执行本地化检查失败: %v, output: %s", err, logOutput)
	}
	// 忽略日志输出，只返回结果
	return needed, nil
}

// 检查角色卡是否已经被本地化
func (s *Service) IsLocalized(characterName string) (bool, error) {
	nikoBasePath := s.buildNikoPath()
	if nikoBasePath == "" {
		return false, fmt.Errorf("无法构建Niko路径：酒馆公共目录未配置")
	}

	// 检查原始名称
	nikoPath := filepath.Join(nikoBasePath, characterName)
	info, err := os.Stat(nikoPath)
	if err == nil && info.IsDir() {
		return true, nil
	}

	// 使用正则表达式移除所有非字母数字字符
	reg := regexp.MustCompile(`[.();【】《》？！，、——：:\[\]]`)
	sanitizedName := reg.ReplaceAllString(characterName, "")
	nikoPathSanitized := filepath.Join(nikoBasePath, sanitizedName)
	info, err = os.Stat(nikoPathSanitized)
	if err == nil && info.IsDir() {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

// 调用 localizer.Run 来执行本地化操作
func (s *Service) RunLocalization(cardPath string) (string, error) {
	opts := localizer.Options{
		CardPath: cardPath,
		BasePath: s.tavernPublicPath,
		Proxy:    s.proxy,
	}
	// 直接调用新的 Run 函数
	_, logOutput, err := localizer.Run(opts)
	if err != nil {
		// 即使有错误，也返回输出，以便前端显示
		return logOutput, err
	}

	return logOutput, nil
}

// 调用 localizer.Run 来执行本地化操作，支持流式输出
func (s *Service) RunLocalizationWithStreaming(cardPath string, sendMessage func(msgType, content string)) (string, error) {
	opts := localizer.Options{
		CardPath: cardPath,
		BasePath: s.tavernPublicPath,
		Proxy:    s.proxy,
	}
	
	// 调用带流式输出的 Run 函数
	_, logOutput, err := localizer.RunWithStreaming(opts, sendMessage)
	if err != nil {
		// 即使有错误，也返回输出，以便前端显示
		return logOutput, err
	}

	return logOutput, nil
}