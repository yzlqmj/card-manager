package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// 手动检查 --check 标志，绕过 flag 包的顺序限制
	isCheckMode := false
	cardPath := ""
	otherArgs := []string{}
	for _, arg := range os.Args[1:] {
		if arg == "--check" {
			isCheckMode = true
		} else if !strings.HasPrefix(arg, "-") && cardPath == "" {
			cardPath = arg
		} else {
			otherArgs = append(otherArgs, arg)
		}
	}

	if cardPath == "" {
		fmt.Fprintln(os.Stderr, "错误: 缺少角色卡路径参数")
		os.Exit(1)
	}

	// 加载配置文件
	cliConfig, err := loadCliConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载 config.json 出错: %v\n", err)
		cliConfig = &CliConfig{}
	}

	// 正常解析其他标志
	fs := flag.NewFlagSet("cli", flag.ExitOnError)
	basePathFlag := fs.String("base-path", cliConfig.BasePath, "SillyTavern 的 public 文件夹路径")
	proxyFlag := fs.String("proxy", cliConfig.Proxy, "代理地址, 例如: http://127.0.0.1:7890")
	fs.Parse(otherArgs)

	// 1. 从 PNG 加载角色卡数据
	base64Data, err := GetCharacterData(cardPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "从 %s 读取角色卡数据时出错: %v\n", cardPath, err)
		os.Exit(1)
	}

	jsonData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解码 base64 数据时出错: %v\n", err)
		os.Exit(1)
	}

	var cardData map[string]interface{}
	if err := json.Unmarshal(jsonData, &cardData); err != nil {
		fmt.Fprintf(os.Stderr, "解析 json 数据时出错: %v\n", err)
		os.Exit(1)
	}

	// 2. 创建一个临时本地化工具仅用于查找 URL
	// 此处的输出路径仅为占位符
	tempLocalizer, err := NewLocalizer(cardData, "./temp_output", *proxyFlag, func(message, level string) {})
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建临时本地化工具失败: %v\n", err)
		os.Exit(1)
	}

	cardDataBytes, _ := json.Marshal(cardData)
	tasks := tempLocalizer.findAndQueueURLs(string(cardDataBytes), "json")
	needsLocalization := len(tasks) > 0

	// 3. 执行请求的功能
	if isCheckMode {
		if needsLocalization {
			fmt.Println("True")
		} else {
			fmt.Println("False")
		}
		os.Exit(0)
	}

	// --- 完整本地化流程 ---
	if !needsLocalization {
		fmt.Println("分析完成: 未发现任何需要本地化的链接。")
		os.Exit(0)
	}

	if *basePathFlag == "" {
		fmt.Fprintln(os.Stderr, "错误: 请使用 --base-path 提供一个有效的 SillyTavern public 目录路径")
		os.Exit(1)
	}

	fmt.Println("开始本地化处理...")

	charName, _ := cardData["name"].(string)
	if charName == "" {
		charName = strings.TrimSuffix(filepath.Base(cardPath), filepath.Ext(cardPath))
	}
	// 移除 Windows 文件名中的非法字符，保留其他符号
	r := strings.NewReplacer(`\`, " ", `/`, " ", `:`, "：", `*`, " ", `?`, "？", `"`, "”", `<`, " ", `>`, " ", `|`, " ")
	safeCharName := r.Replace(charName)

	resourceOutputDir := filepath.Join(*basePathFlag, "niko", safeCharName)
	if err := os.MkdirAll(resourceOutputDir, os.ModePerm); err != nil {
		fmt.Fprintf(os.Stderr, "创建资源输出目录失败: %v\n", err)
		os.Exit(1)
	}

	// 创建真正的本地化工具
	progressCallback := func(message string, level string) {
		fmt.Printf("[%s] %s\n", strings.ToUpper(level), message)
	}
	localizer, err := NewLocalizer(cardData, resourceOutputDir, *proxyFlag, progressCallback)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建本地化工具失败: %v\n", err)
		os.Exit(1)
	}

	updatedCardData, err := localizer.Localize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "本地化过程失败: %v\n", err)
		os.Exit(1)
	}

	// 4. 准备 V2 和 V3 数据用于写入
	// V2 数据 (移除 spec 和 spec_version)
	v2CardData := make(map[string]interface{})
	for k, v := range updatedCardData {
		if k != "spec" && k != "spec_version" {
			v2CardData[k] = v
		}
	}
	v2Bytes, err := json.Marshal(v2CardData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "序列化 V2 数据失败: %v\n", err)
		os.Exit(1)
	}
	v2Base64 := base64.StdEncoding.EncodeToString(v2Bytes)

	// V3 数据 (添加/更新 spec 和 spec_version)
	v3CardData := updatedCardData
	v3CardData["spec"] = "chara_card_v3"
	v3CardData["spec_version"] = "3.0"
	v3Bytes, err := json.Marshal(v3CardData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "序列化 V3 数据失败: %v\n", err)
		os.Exit(1)
	}
	v3Base64 := base64.StdEncoding.EncodeToString(v3Bytes)

	// 5. 写入新的角色卡
	cardOutputDir := filepath.Join(filepath.Dir(cardPath), "本地化")
	if err := os.MkdirAll(cardOutputDir, os.ModePerm); err != nil {
		fmt.Fprintf(os.Stderr, "创建本地化角色卡目录失败: %v\n", err)
		os.Exit(1)
	}
	finalCardPath := filepath.Join(cardOutputDir, filepath.Base(cardPath))

	err = WriteCharacterData(cardPath, finalCardPath, v2Base64, v3Base64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "写入新角色卡失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("本地化成功！新卡保存至: %s\n", finalCardPath)
}
