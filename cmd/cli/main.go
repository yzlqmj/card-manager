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

	if err := localizer.Run(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
