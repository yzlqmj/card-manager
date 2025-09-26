package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"card-manager/localizer"
)

// checkLocalizationNeeded 调用 localizer.Run 来判断角色卡是否需要本地化。
func checkLocalizationNeeded(cardPath string) (bool, error) {
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

// isLocalized 检查角色卡是否已经被本地化。
func isLocalized(characterName string) (bool, error) {
	if config.TavernPublicPath == "" {
		return false, fmt.Errorf("SillyTavern public path not configured")
	}

	// 检查原始名称
	nikoPath := filepath.Join(config.TavernPublicPath, "niko", characterName)
	info, err := os.Stat(nikoPath)
	if err == nil && info.IsDir() {
		return true, nil
	}

	// 使用正则表达式移除所有非字母数字字符
	reg := regexp.MustCompile(`[.();【】《》？！，、——：:\[\]]`)
	sanitizedName := reg.ReplaceAllString(characterName, "")
	nikoPathSanitized := filepath.Join(config.TavernPublicPath, "niko", sanitizedName)
	info, err = os.Stat(nikoPathSanitized)
	if err == nil && info.IsDir() {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

// runLocalization 调用 localizer.Run 来执行本地化操作。
func runLocalization(cardPath string) (string, error) {
	opts := localizer.Options{
		CardPath: cardPath,
		BasePath: config.TavernPublicPath,
		Proxy:    config.Proxy,
	}
	// 直接调用新的 Run 函数
	_, logOutput, err := localizer.Run(opts)
	if err != nil {
		// 即使有错误，也返回输出，以便前端显示
		return logOutput, err
	}

	return logOutput, nil
}
