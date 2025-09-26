package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"card-manager/localizer"
)

// checkLocalizationNeeded 调用 localizer.Run 来判断角色卡是否需要本地化。
func checkLocalizationNeeded(cardPath string) (bool, error) {
	var out bytes.Buffer
	// 模拟 cli.exe 的输出捕获
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := localizer.Options{
		CardPath:    cardPath,
		IsCheckMode: true,
	}
	err := localizer.Run(opts)

	w.Close()
	os.Stdout = oldStdout
	out.ReadFrom(r)

	if err != nil {
		return false, fmt.Errorf("执行本地化检查失败: %v, output: %s", err, out.String())
	}

	output := strings.TrimSpace(out.String())
	if strings.Contains(output, "True") {
		return true, nil
	} else if strings.Contains(output, "False") {
		return false, nil
	}

	return false, fmt.Errorf("未知的本地化检查输出: %s", output)
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
	var out bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := localizer.Options{
		CardPath: cardPath,
		BasePath: config.TavernPublicPath,
		Proxy:    config.Proxy,
	}
	err := localizer.Run(opts)

	w.Close()
	os.Stdout = oldStdout
	out.ReadFrom(r)

	output := out.String()
	if err != nil {
		// 即使有错误，也返回输出，以便前端显示
		return output, err
	}

	return output, nil
}
