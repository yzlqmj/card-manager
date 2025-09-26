package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	checkFlag := flag.Bool("check", false, "Only check if localization is needed and return True or False")
	basePathFlag := flag.String("base-path", "", "SillyTavern's public folder path")
	proxyFlag := flag.String("proxy", "", "Proxy address, e.g., http://127.0.0.1:7890")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: Missing card path argument")
		os.Exit(1)
	}
	cardPath := args[0]

	// 1. Load character data from PNG
	base64Data, err := GetCharacterData(cardPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading character data from %s: %v\n", cardPath, err)
		os.Exit(1)
	}

	jsonData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding base64 data: %v\n", err)
		os.Exit(1)
	}

	var cardData map[string]interface{}
	if err := json.Unmarshal(jsonData, &cardData); err != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshalling json data: %v\n", err)
		os.Exit(1)
	}

	// 2. Create a temporary localizer just to find URLs
	// The output path is a placeholder as we only need to find URLs.
	tempLocalizer, err := NewLocalizer(cardData, "./temp_output", *proxyFlag, func(message, level string) {})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temporary localizer: %v\n", err)
		os.Exit(1)
	}

	cardDataBytes, _ := json.Marshal(cardData)
	tasks := tempLocalizer.findAndQueueURLs(string(cardDataBytes), "json")
	needsLocalization := len(tasks) > 0

	// 3. Execute requested function
	if *checkFlag {
		fmt.Println(strings.Title(fmt.Sprintf("%v", needsLocalization)))
		os.Exit(0)
	}

	// --- Full Localization ---
	if !needsLocalization {
		fmt.Println("Analysis complete: No links found that require localization.")
		os.Exit(0)
	}

	if *basePathFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: Please provide a valid SillyTavern public directory path with --base-path")
		os.Exit(1)
	}

	fmt.Println("Starting localization process...")

	charName, _ := cardData["name"].(string)
	if charName == "" {
		charName = strings.TrimSuffix(filepath.Base(cardPath), filepath.Ext(cardPath))
	}
	// Sanitize character name for folder
	reg := regexp.MustCompile(`[^a-zA-Z0-9_ -]`)
	safeCharName := reg.ReplaceAllString(charName, "")

	resourceOutputDir := filepath.Join(*basePathFlag, "niko", safeCharName)
	if err := os.MkdirAll(resourceOutputDir, os.ModePerm); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating resource output directory: %v\n", err)
		os.Exit(1)
	}

	// Create the real localizer
	progressCallback := func(message string, level string) {
		fmt.Printf("[%s] %s\n", strings.ToUpper(level), message)
	}
	localizer, err := NewLocalizer(cardData, resourceOutputDir, *proxyFlag, progressCallback)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create localizer: %v\n", err)
		os.Exit(1)
	}

	updatedCardData, err := localizer.Localize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Localization process failed: %v\n", err)
		os.Exit(1)
	}

	// 4. Prepare V2 and V3 data for writing
	// V2 data (spec and spec_version removed)
	v2CardData := make(map[string]interface{})
	for k, v := range updatedCardData {
		if k != "spec" && k != "spec_version" {
			v2CardData[k] = v
		}
	}
	v2Bytes, err := json.Marshal(v2CardData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshalling V2 data: %v\n", err)
		os.Exit(1)
	}
	v2Base64 := base64.StdEncoding.EncodeToString(v2Bytes)

	// V3 data (spec and spec_version added/updated)
	v3CardData := updatedCardData
	v3CardData["spec"] = "chara_card_v3"
	v3CardData["spec_version"] = "3.0"
	v3Bytes, err := json.Marshal(v3CardData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshalling V3 data: %v\n", err)
		os.Exit(1)
	}
	v3Base64 := base64.StdEncoding.EncodeToString(v3Bytes)

	// 5. Write to new card
	cardOutputDir := filepath.Join(filepath.Dir(cardPath), "本地化")
	if err := os.MkdirAll(cardOutputDir, os.ModePerm); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating localized card directory: %v\n", err)
		os.Exit(1)
	}
	finalCardPath := filepath.Join(cardOutputDir, filepath.Base(cardPath))

	err = WriteCharacterData(cardPath, finalCardPath, v2Base64, v3Base64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write new character card: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Localization successful! New card saved to: %s\n", finalCardPath)
}
