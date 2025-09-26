package main

import (
	"flag"
	"fmt"
	"os"

	"card-manager/localizer"
)

func main() {
	checkFlag := flag.Bool("check", false, "Check if localization is needed")
	basePathFlag := flag.String("base-path", "", "SillyTavern's public folder path")
	proxyFlag := flag.String("proxy", "", "Proxy address, e.g., http://127.0.0.1:7890")
	flag.Parse()

	cardPath := flag.Arg(0)
	if cardPath == "" {
		fmt.Fprintln(os.Stderr, "Error: Missing character card path parameter")
		os.Exit(1)
	}

	opts := localizer.Options{
		CardPath:    cardPath,
		BasePath:    *basePathFlag,
		Proxy:       *proxyFlag,
		IsCheckMode: *checkFlag,
	}

	needed, logOutput, err := localizer.Run(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "发生错误:", err)
		fmt.Fprintln(os.Stderr, "--- 日志 ---")
		fmt.Fprintln(os.Stderr, logOutput)
		os.Exit(1)
	}

	// 打印日志
	fmt.Println(logOutput)

	// 在检查模式下，根据需要退出
	if opts.IsCheckMode {
		if needed {
			// 可以选择用特定的退出码表示需要本地化
			// os.Exit(10)
		}
	}
}
