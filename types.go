package main

// CardVersion 代表一个卡片的特定版本
type CardVersion struct {
	Path         string `json:"path"`
	FileName     string `json:"fileName"`
	Mtime        string `json:"mtime"`
	InternalName string `json:"internalName"`
}

// Character 代表一个角色
type Character struct {
	Name               string        `json:"name"`
	InternalName       string        `json:"internalName"`
	FolderPath         string        `json:"folderPath"`
	LatestVersionPath  string        `json:"latestVersionPath"`
	VersionCount       int           `json:"versionCount"`
	Versions           []CardVersion `json:"versions"`
	ImportInfo         ImportInfo    `json:"importInfo"`
	HasNote            bool          `json:"hasNote"`
	HasFaceFolder      bool          `json:"hasFaceFolder"`
	LocalizationNeeded *bool         `json:"localizationNeeded,omitempty"`
	IsLocalized        bool          `json:"isLocalized"`
}

// ImportInfo 包含卡片的导入状态
type ImportInfo struct {
	IsImported          bool   `json:"isImported"`
	ImportedVersionPath string `json:"importedVersionPath"`
	IsLatestImported    bool   `json:"isLatestImported"`
}

// StrayCard 代表一张待整理的卡片
type StrayCard struct {
	FileName string `json:"fileName"`
	Path     string `json:"path"`
}

// CardsResponse 是 /api/cards 端点的响应结构
type CardsResponse struct {
	Categories map[string][]Character `json:"categories"`
	StrayCards []StrayCard            `json:"strayCards"`
}

// StatsResponse 是 /api/stats 端点的响应结构
type StatsResponse struct {
	TotalCharacters   int `json:"totalCharacters"`
	NeedsLocalization int `json:"needsLocalization"`
	NotLocalized      int `json:"notLocalized"`
	NotImported       int `json:"notImported"`
	NotLatestImported int `json:"notLatestImported"`
}
