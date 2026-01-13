package cache

import (
	"encoding/json"
	"os"
	"sync"
)

// Entry 缓存条目
type Entry struct {
	Hash               string `json:"hash"`
	InternalName       string `json:"internalName"`
	Mtime              string `json:"mtime"`
	LocalizationNeeded *bool  `json:"localizationNeeded,omitempty"`
}

// Manager 缓存管理器
type Manager struct {
	cache     map[string]Entry
	mutex     sync.RWMutex
	cachePath string
}

// NewManager 创建新的缓存管理器
func NewManager(cachePath string) *Manager {
	return &Manager{
		cache:     make(map[string]Entry),
		cachePath: cachePath,
	}
}

// Load 从文件加载缓存
func (m *Manager) Load() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, err := os.Stat(m.cachePath); os.IsNotExist(err) {
		m.cache = make(map[string]Entry)
		return nil
	}

	file, err := os.ReadFile(m.cachePath)
	if err != nil {
		m.cache = make(map[string]Entry)
		return err
	}

	return json.Unmarshal(file, &m.cache)
}

// Save 保存缓存到文件
func (m *Manager) Save() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	data, err := json.MarshalIndent(m.cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.cachePath, data, 0644)
}

// Get 获取缓存条目
func (m *Manager) Get(key string) (Entry, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	entry, found := m.cache[key]
	return entry, found
}

// Set 设置缓存条目
func (m *Manager) Set(key string, entry Entry) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.cache[key] = entry
}

// Clear 清除缓存
func (m *Manager) Clear() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.cache = make(map[string]Entry)
	return os.Remove(m.cachePath)
}

// IsEmpty 检查缓存是否为空
func (m *Manager) IsEmpty() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.cache) == 0
}

// GetAll 获取所有缓存条目（用于调试）
func (m *Manager) GetAll() map[string]Entry {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	result := make(map[string]Entry)
	for k, v := range m.cache {
		result[k] = v
	}
	return result
}