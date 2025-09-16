package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// checkLocalizationNeeded 调用 cli.exe --check 来判断角色卡是否需要本地化。
// 它返回一个布尔值和任何可能发生的错误。
func checkLocalizationNeeded(cardPath string) (bool, error) {
	cmd := exec.Command("./cli/cli.exe", cardPath, "--check")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out // 将标准错误也重定向到 out，以便调试

	err := cmd.Run()
	if err != nil {
		// 如果命令执行失败，返回错误信息，包括命令的输出
		return false, fmt.Errorf("执行本地化检查失败: %v, output: %s", err, out.String())
	}

	// 去除输出中的空格和换行符
	output := strings.TrimSpace(out.String())

	// 根据输出判断结果
	if output == "True" {
		return true, nil
	} else if output == "False" {
		return false, nil
	}

	// 如果输出不是预期的 "True" 或 "False"，则认为是一个错误
	return false, fmt.Errorf("未知的本地化检查输出: %s", output)
}

// isLocalized 检查角色卡是否已经被本地化。
// 通过检查在 SillyTavern 的 public/niko 目录下是否存在与角色名同名的文件夹来判断。
func isLocalized(characterName string) (bool, error) {
	// 从配置中获取 SillyTavern 的 public 目录
	// 注意：cli 的 config.json 中的 base_path 是 public 目录，我们需要的是 niko 目录
	// 我们假设 niko 目录就在 public 目录下
	if config.TavernPublicPath == "" {
		return false, fmt.Errorf("SillyTavern public path not configured")
	}
	nikoPath := filepath.Join(config.TavernPublicPath, "niko", characterName)

	// 检查与角色名同名的文件夹是否存在
	info, err := os.Stat(nikoPath)
	if os.IsNotExist(err) {
		// 文件夹不存在，说明未本地化
		return false, nil
	}
	if err != nil {
		// 其他类型的错误
		return false, err
	}

	// 文件夹存在且是一个目录，说明已本地化
	return info.IsDir(), nil
}

// runLocalization 调用 cli.exe 来执行本地化操作。
// 它返回命令的实时输出。
func runLocalization(cardPath string) (string, error) {
	cmd := exec.Command("./cli/cli.exe", cardPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return out.String(), fmt.Errorf("执行本地化失败: %v", err)
	}

	return out.String(), nil
}
