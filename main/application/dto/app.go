package dto

type AppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type DiskInfo struct {
	Name        string  `json:"name"`        // 挂载点名称（如 /、C:、/home）
	UsedPercent float64 `json:"usedPercent"` // 使用率（0-100）
}
