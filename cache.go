package main

import (
	"encoding/json"
	"os"
	"sync"
)

// CacheEntry 存储单个文件的缓存元数据
type CacheEntry struct {
	Hash         string `json:"hash"`
	InternalName string `json:"internalName"`
	Mtime        string `json:"mtime"`
}

var (
	cache      = make(map[string]CacheEntry)
	cacheMutex sync.RWMutex
	cachePath  = "cache.json"
)

// loadCache 从 cache.json 加载缓存
func loadCache() error {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		cache = make(map[string]CacheEntry)
		return nil
	}

	file, err := os.ReadFile(cachePath)
	if err != nil {
		cache = make(map[string]CacheEntry)
		return err
	}

	return json.Unmarshal(file, &cache)
}

// saveCache 将缓存保存到 cache.json
func saveCache() error {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0644)
}

// getCache 获取缓存条目
func getCache(key string) (CacheEntry, bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	entry, found := cache[key]
	return entry, found
}

// setCache 设置缓存条目
func setCache(key string, entry CacheEntry) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	cache[key] = entry
}

// clearCache 清除缓存
func clearCache() error {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	cache = make(map[string]CacheEntry)
	return os.Remove(cachePath)
}
