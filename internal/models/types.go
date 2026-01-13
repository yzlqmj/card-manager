package models

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

// APIResponse 统一的API响应格式
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// DownloadCardRequest 下载卡片请求
type DownloadCardRequest struct {
	URL           string `json:"url"`
	Category      string `json:"category"`
	CharacterName string `json:"characterName"`
	FileName      string `json:"fileName"`
	IsFace        bool   `json:"isFace"`
}

// OpenFolderRequest 打开文件夹请求
type OpenFolderRequest struct {
	FolderPath string `json:"folderPath"`
}

// DeleteVersionRequest 删除版本请求
type DeleteVersionRequest struct {
	FilePath string `json:"filePath"`
}

// MoveCharacterRequest 移动角色请求
type MoveCharacterRequest struct {
	OldFolderPath string `json:"oldFolderPath"`
	NewCategory   string `json:"newCategory"`
}

// OrganizeStrayRequest 整理待整理卡片请求
type OrganizeStrayRequest struct {
	StrayPath     string `json:"strayPath"`
	Category      string `json:"category"`
	CharacterName string `json:"characterName"`
}

// SaveNoteRequest 保存备注请求
type SaveNoteRequest struct {
	FolderPath string `json:"folderPath"`
	Content    string `json:"content"`
}

// LocalizeCardRequest 本地化卡片请求
type LocalizeCardRequest struct {
	CardPath string `json:"cardPath"`
}

// SubmitUrlRequest 提交URL请求
type SubmitUrlRequest struct {
	URL string `json:"url"`
}

// MergeJsonToPngRequest 合并JSON到PNG请求
type MergeJsonToPngRequest struct {
	FolderPath   string `json:"folderPath"`
	JsonFileName string `json:"jsonFileName"`
	PngFileName  string `json:"pngFileName"`
}