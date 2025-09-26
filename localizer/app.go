package localizer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Options defines the parameters for the Run function.
type Options struct {
	CardPath    string
	BasePath    string
	Proxy       string
	IsCheckMode bool
}

// Run executes the localization process based on the provided options.
func Run(opts Options) error {
	// 加载配置文件
	cliConfig, err := LoadCliConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载 config.json 出错: %v\n", err)
		cliConfig = &CliConfig{}
	}

	// 如果命令行没有提供，则使用配置文件中的值
	if opts.BasePath == "" {
		opts.BasePath = cliConfig.BasePath
	}
	if opts.Proxy == "" {
		opts.Proxy = cliConfig.Proxy
	}

	// 1. 从 PNG 加载角色卡数据
	base64Data, err := GetCharacterData(opts.CardPath)
	if err != nil {
		return fmt.Errorf("从 %s 读取角色卡数据时出错: %v", opts.CardPath, err)
	}

	jsonData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("解码 base64 数据时出错: %v", err)
	}

	var cardData map[string]interface{}
	if err := json.Unmarshal(jsonData, &cardData); err != nil {
		return fmt.Errorf("解析 json 数据时出错: %v", err)
	}

	// 2. 创建一个临时本地化工具仅用于查找 URL
	tempLocalizer, err := NewLocalizer(cardData, "./temp_output", opts.Proxy, func(message, level string) {})
	if err != nil {
		return fmt.Errorf("创建临时本地化工具失败: %v", err)
	}

	cardDataBytes, _ := json.Marshal(cardData)
	tasks := tempLocalizer.findAndQueueURLs(string(cardDataBytes), "json")
	needsLocalization := len(tasks) > 0

	// 3. 执行请求的功能
	if opts.IsCheckMode {
		if needsLocalization {
			fmt.Println("True")
		} else {
			fmt.Println("False")
		}
		return nil
	}

	// --- 完整本地化流程 ---
	if !needsLocalization {
		fmt.Println("分析完成: 未发现任何需要本地化的链接。")
		return nil
	}

	if opts.BasePath == "" {
		return fmt.Errorf("错误: 请使用 --base-path 提供一个有效的 SillyTavern public 目录路径")
	}

	fmt.Println("开始本地化处理...")

	charName, _ := cardData["name"].(string)
	if charName == "" {
		charName = strings.TrimSuffix(filepath.Base(opts.CardPath), filepath.Ext(opts.CardPath))
	}
	r := strings.NewReplacer(`\`, " ", `/`, " ", `:`, "：", `*`, " ", `?`, "？", `"`, "”", `<`, " ", `>`, " ", `|`, " ")
	safeCharName := r.Replace(charName)

	resourceOutputDir := filepath.Join(opts.BasePath, "niko", safeCharName)
	if err := os.MkdirAll(resourceOutputDir, os.ModePerm); err != nil {
		return fmt.Errorf("创建资源输出目录失败: %v", err)
	}

	progressCallback := func(message string, level string) {
		fmt.Printf("[%s] %s\n", strings.ToUpper(level), message)
	}
	localizer, err := NewLocalizer(cardData, resourceOutputDir, opts.Proxy, progressCallback)
	if err != nil {
		return fmt.Errorf("创建本地化工具失败: %v", err)
	}

	updatedCardData, err := localizer.Localize()
	if err != nil {
		return fmt.Errorf("本地化过程失败: %v", err)
	}

	v2CardData := make(map[string]interface{})
	for k, v := range updatedCardData {
		if k != "spec" && k != "spec_version" {
			v2CardData[k] = v
		}
	}
	v2Bytes, err := json.Marshal(v2CardData)
	if err != nil {
		return fmt.Errorf("序列化 V2 数据失败: %v", err)
	}
	v2Base64 := base64.StdEncoding.EncodeToString(v2Bytes)

	v3CardData := updatedCardData
	v3CardData["spec"] = "chara_card_v3"
	v3CardData["spec_version"] = "3.0"
	v3Bytes, err := json.Marshal(v3CardData)
	if err != nil {
		return fmt.Errorf("序列化 V3 数据失败: %v", err)
	}
	v3Base64 := base64.StdEncoding.EncodeToString(v3Bytes)

	cardOutputDir := filepath.Join(filepath.Dir(opts.CardPath), "本地化")
	if err := os.MkdirAll(cardOutputDir, os.ModePerm); err != nil {
		return fmt.Errorf("创建本地化角色卡目录失败: %v", err)
	}
	finalCardPath := filepath.Join(cardOutputDir, filepath.Base(opts.CardPath))

	err = WriteCharacterData(opts.CardPath, finalCardPath, v2Base64, v3Base64)
	if err != nil {
		return fmt.Errorf("写入新角色卡失败: %v", err)
	}

	fmt.Printf("本地化成功！新卡保存至: %s\n", finalCardPath)
	return nil
}
