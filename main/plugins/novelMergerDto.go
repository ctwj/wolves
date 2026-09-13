package plugins

// NovelMerger「小说章节合并」的请求/结果模型（仿 spiderValidate.go 风格）：
// 预览与合并共用一套入参；流程级失败不返回 error 而是填 Error 字段，
// 让前端能展示部分结果。合并的 ids 顺序即预览确认的顺序，所见即所得。

import "time"

// novelMergeRequest 预览/合并共用入参
type novelMergeRequest struct {
	Keyword string `json:"keyword"` // 小说名关键词（匹配标题包含）
	Pattern string `json:"pattern"` // 自定义章节号正则（第一捕获组=章节数字），空=内置识别
	IDs     []int  `json:"ids,omitempty"`
}

// NovelMergeChapterItem 预览中的单篇章节文章
type NovelMergeChapterItem struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	CreateTime int64  `json:"create_time"`
	CategoryID int    `json:"category_id"`
	Status     bool   `json:"status"`
	ChapterNo  int    `json:"chapter_no"` // 0=未识别
	MatchedBy  string `json:"matched_by"` // 命中的模式名（custom / builtin:xxx）
	Recognized bool   `json:"recognized"`
	SortIndex  int    `json:"sort_index"` // 排序后位置，从 1 起
}

// NovelMergeExcludedItem 被排除的文章及原因
type NovelMergeExcludedItem struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

// NovelMergeTarget 合并目标预判信息
type NovelMergeTarget struct {
	Title      string `json:"title"`
	Slug       string `json:"slug"`
	CategoryID int    `json:"category_id"`
}

// NovelMergePreviewResult 预览结果（只读，不入库）
type NovelMergePreviewResult struct {
	Plugin        string                     `json:"plugin"`
	Keyword       string                     `json:"keyword"`
	PatternUsed   string                     `json:"pattern_used"`
	Total         int                        `json:"total"`         // 标题命中总数
	Included      int                        `json:"included"`      // 参与合并的章节数
	Recognized    int                        `json:"recognized"`    // 识别出章节号的数量
	Unrecognized  int                        `json:"unrecognized"`  // 未识别章节号的数量
	Excluded      int                        `json:"excluded"`      // 排除数量
	Truncated     bool                       `json:"truncated"`     // 命中超过扫描上限
	Chapters      []NovelMergeChapterItem    `json:"chapters"`      // 排序后列表（未识别排尾）
	ExcludedList  []NovelMergeExcludedItem   `json:"excluded_list"`
	DuplicateNos  []int                      `json:"duplicate_nos"` // 出现多于一次的章节号
	Target        NovelMergeTarget           `json:"target"`
	SlugTaken     bool                       `json:"slug_taken"`   // 预判 slug 已被占用（执行时自动加后缀）
	TitleExists   bool                       `json:"title_exists"` // 已存在同名文章（执行时拒绝）
	Error         string                     `json:"error,omitempty"`  // 非空=流程失败，禁止确认合并
	Warning       string                     `json:"warning,omitempty"` // 非致命提示（如未识别章节号），不阻止合并
	CostMS        int64                      `json:"cost_ms"`
	GeneratedAt   string                     `json:"generated_at"`
}

// NovelMergeFailedDelete 删除失败的原章节文章
type NovelMergeFailedDelete struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Error string `json:"error"`
}

// NovelMergeResult 合并执行结果
type NovelMergeResult struct {
	NewID         int                     `json:"new_id"`
	Title         string                  `json:"title"`
	Slug          string                  `json:"slug"`
	URL           string                  `json:"url"`
	ChapterCount  int                     `json:"chapter_count"`
	DeletedCount  int                     `json:"deleted_count"`
	FailedDeletes []NovelMergeFailedDelete `json:"failed_deletes"`
	CostMS        int64                   `json:"cost_ms"`
}

func newNovelMergePreviewResult(keyword, pattern string) *NovelMergePreviewResult {
	patternUsed := "builtin"
	if pattern != "" {
		patternUsed = pattern
	}
	return &NovelMergePreviewResult{
		Plugin:       "NovelMerger",
		Keyword:      keyword,
		PatternUsed:  patternUsed,
		Chapters:     make([]NovelMergeChapterItem, 0),
		ExcludedList: make([]NovelMergeExcludedItem, 0),
		GeneratedAt:  time.Now().Format("2006-01-02 15:04:05"),
	}
}
