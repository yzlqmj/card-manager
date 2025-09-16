package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	importedHashes        = make(map[string]bool)
	importedInternalNames = make(map[string]bool)
	TavernScanMutex       sync.Mutex
	isScanningTavern      bool
)

// scanTavernHashes 扫描 tavern 目录并填充 importedHashes 和 importedInternalNames
func scanTavernHashes() error {
	TavernScanMutex.Lock()
	if isScanningTavern {
		TavernScanMutex.Unlock()
		return nil // 如果已经在扫描，则直接返回
	}
	isScanningTavern = true
	TavernScanMutex.Unlock()

	defer func() {
		TavernScanMutex.Lock()
		isScanningTavern = false
		TavernScanMutex.Unlock()
	}()

	localHashes := make(map[string]bool)
	localInternalNames := make(map[string]bool)

	if config.TavernCharactersPath == "" {
		return nil
	}

	err := filepath.Walk(config.TavernCharactersPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".png") {
			// 计算文件哈希
			file, err := os.Open(path)
			if err != nil {
				return nil // 忽略无法打开的文件
			}
			defer file.Close()

			hash := sha256.New()
			if _, err := io.Copy(hash, file); err != nil {
				return nil // 忽略无法计算哈希的文件
			}
			localHashes[hex.EncodeToString(hash.Sum(nil))] = true

			// 提取内部名称
			charaData, err := getInternalCharNameFromPNG(path)
			if err == nil {
				decoded, err := base64.StdEncoding.DecodeString(charaData)
				if err == nil {
					var charDataJSON map[string]interface{}
					if json.Unmarshal(decoded, &charDataJSON) == nil {
						if name, ok := charDataJSON["name"].(string); ok && name != "" {
							localInternalNames[name] = true
						} else if name, ok := charDataJSON["char_name"].(string); ok && name != "" {
							localInternalNames[name] = true
						}
					}
				}
			}
		}
		return nil
	})

	// 一次性更新全局 map
	TavernScanMutex.Lock()
	importedHashes = localHashes
	importedInternalNames = localInternalNames
	TavernScanMutex.Unlock()

	return err
}
