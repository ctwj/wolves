package plugins

// HttpSpider / HeadlessSpider 共用的「任务校验」结果模型：
// 抓列表页第一页 → 取第一条链接 → 提取文章字段，全程不入库，仅用于配置正确性验证。
// 前端在任务编辑器提供「验证」按钮，调用 PluginService.SpiderValidate 触发。

import (
	"encoding/json"
	"fmt"
	"moss/domain/core/entity"
	"time"
)

// ValidateLink 校验阶段的链接样本（标题用于人工确认取链正确）
type ValidateLink struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// ValidateArticle 从提取结果映射的展示模型（字段级 OK 标记供前端逐行亮灯）
type ValidateArticle struct {
	Title          string           `json:"title"`
	Slug           string           `json:"slug"`
	Cover          string           `json:"cover"`
	ContentLen     int              `json:"content_len"`
	ContentPreview string           `json:"content_preview"`
	Keywords       string           `json:"keywords"`
	PublishTime    string           `json:"publish_time"`
	VideoSources   []map[string]any `json:"video_sources"`
	GalleryImages  []string         `json:"gallery_images"`
	Extras         map[string]any   `json:"extras"`
	Missed         []string         `json:"missed"`
}

// ValidateResult 校验全流程结果：Error 非空代表流程失败（列表页/详情页阶段），
// Article 非 nil 时字段级问题看 Missed（已配置选择器但未命中）
type ValidateResult struct {
	Plugin      string           `json:"plugin"`
	TaskName    string           `json:"task_name"`
	ListURL     string           `json:"list_url"`
	LinksFound  int              `json:"links_found"`
	SampleLinks []ValidateLink   `json:"sample_links"`
	FirstLink   ValidateLink     `json:"first_link"`
	Article     *ValidateArticle `json:"article"`
	Error       string           `json:"error,omitempty"`
	CostMS      int64            `json:"cost_ms"`
}

func newValidateResult(plugin, taskName, listURL string) *ValidateResult {
	return &ValidateResult{
		Plugin:   plugin,
		TaskName: taskName,
		ListURL:  listURL,
	}
}

// articleToValidate entity.Article → 校验展示模型：
// video_sources/gallery_images 从 extends 拆出单列，其余 extends 归入 Extras；
// 正文只带预览（前 600 字符，去标签），完整内容不入校验载荷
func articleToValidate(a *entity.Article, missed []string) *ValidateArticle {
	va := &ValidateArticle{
		Title:          a.Title,
		Slug:           a.Slug,
		Cover:          a.Thumbnail,
		ContentLen:     len([]rune(a.Content)),
		ContentPreview: plainSummary(a.Content, 600),
		Keywords:       a.Keywords,
		PublishTime:    time.Unix(a.CreateTime, 0).Format("2006-01-02 15:04:05"),
		Missed:         missed,
	}
	for _, item := range a.Extends {
		switch item.Key {
		case "video_sources":
			if raw, ok := item.Value.([]map[string]any); ok {
				va.VideoSources = raw
			} else if b, err := json.Marshal(item.Value); err == nil {
				_ = json.Unmarshal(b, &va.VideoSources) // list 模式等场景 Value 可能是任意 JSON 形态
			}
		case "gallery_images":
			if raw, ok := item.Value.([]string); ok {
				va.GalleryImages = raw
			} else if b, err := json.Marshal(item.Value); err == nil {
				_ = json.Unmarshal(b, &va.GalleryImages)
			}
		default:
			if va.Extras == nil {
				va.Extras = map[string]any{}
			}
			va.Extras[item.Key] = item.Value
		}
	}
	return va
}

// validateSampleLinks 截取前 n 条链接样本
func validateSampleLinks(n int, get func(i int) ValidateLink) []ValidateLink {
	out := make([]ValidateLink, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, get(i))
	}
	return out
}

// validateUnmarshalTask 校验入口共用：任务 JSON → 目标结构
func validateUnmarshalTask(raw string, target any) error {
	if raw == "" {
		return fmt.Errorf("任务数据为空")
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return fmt.Errorf("任务 JSON 解析失败: %w", err)
	}
	return nil
}
