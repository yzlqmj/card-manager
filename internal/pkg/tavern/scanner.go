package tavern

import (
	"card-manager/internal/pkg/png"
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

// Scanner Tavern目录扫描器
type Scanner struct {
	tavernCharactersPath string
	importedHashes       map[string]bool
	importedInternalNames map[string]bool
	mutex                sync.RWMutex
	isScanning           bool
}

// NewScanner 创建新的Tavern扫描器
func NewScanner(tavernCharactersPath string) *Scanner {
	return &Scanner{
		tavernCharactersPath:  tavernCharactersPath,
		importedHashes:        make(map[string]bool),
		importedInternalNames: make(map[string]bool),
	}
}

// ScanHashes 扫描 tavern 目录并填充 importedHashes 和 importedInternalNames
func (s *Scanner) ScanHashes() error {
	s.mutex.Lock()
	if s.isScanning {
		s.mutex.Unlock()
		return nil // 如果已经在扫描，则直接返回
	}
	s.isScanning = true
	s.mutex.Unlock()

	defer func() {
		s.mutex.Lock()
		s.isScanning = false
		s.mutex.Unlock()
	}()

	localHashes := make(map[string]bool)
	localInternalNames := make(map[string]bool)

	if s.tavernCharactersPath == "" {
		return nil
	}

	err := filepath.Walk(s.tavernCharactersPath, func(path string, info os.FileInfo, err error) error {
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
			charaData, err := png.GetInternalCharNameFromPNG(path)
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
	s.mutex.Lock()
	s.importedHashes = localHashes
	s.importedInternalNames = localInternalNames
	s.mutex.Unlock()

	return err
}

// IsHashImported 检查哈希是否已导入
func (s *Scanner) IsHashImported(hash string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.importedHashes[hash]
}

// IsInternalNameImported 检查内部名称是否已导入
func (s *Scanner) IsInternalNameImported(name string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.importedInternalNames[name]
}

// GetImportedHashes 获取所有已导入的哈希
func (s *Scanner) GetImportedHashes() map[string]bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	result := make(map[string]bool)
	for k, v := range s.importedHashes {
		result[k] = v
	}
	return result
}

// GetImportedInternalNames 获取所有已导入的内部名称
func (s *Scanner) GetImportedInternalNames() map[string]bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	result := make(map[string]bool)
	for k, v := range s.importedInternalNames {
		result[k] = v
	}
	return result
}