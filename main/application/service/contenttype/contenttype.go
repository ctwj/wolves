// Package contenttype 提供多媒体内容类型（普通/小说/图片/视频）的
// 纯函数处理：类型规范化、小说章节切分、视频选集与图集解析。
// 独立成包以避免对 config/DB 的传递依赖，保证可单测。
package contenttype

import (
	"regexp"
	"strings"

	"moss/domain/core/vo"
)

// 内容类型取值（空与 standard 等价，表示普通文章）
const (
	TypeStandard = "standard"
	TypeNovel    = "novel"
	TypeImage    = "image"
	TypeVideo    = "video"
)

// chapterSeparator 小说章节分隔符（纯文本，TinyMCE 可视化模式可直接输入，
// 且经 bluemonday UGCPolicy 清洗后存活——HTML 注释会被剥离，不可用）
const chapterSeparator = "===chapter==="

// VideoItem 视频选集：Embed=false 为直链 mp4（<video> 播放），
// true 为第三方播放页（<iframe> 嵌入）
type VideoItem struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Embed bool   `json:"embed"`
}

// ImageItem 图集图片
type ImageItem struct {
	URL string `json:"url"`
}

// NovelChapter 小说章节：Index 从 1 起，Title 取段内首个标题文本（可为空）
type NovelChapter struct {
	Index int    `json:"index"`
	Title string `json:"title"`
	HTML  string `json:"html"`
}

var headingRegexp = regexp.MustCompile(`(?is)<h[1-6][^>]*>(.*?)</h[1-6]>`)
var tagRegexp = regexp.MustCompile(`(?s)<[^>]*>`)

// NormalizeContentType 规范化内容类型：仅接受已知枚举值，
// 非法/未设置值返回空串（普通），与显式 "standard" 等价处理。
func NormalizeContentType(t string) string {
	switch t {
	case TypeStandard, TypeNovel, TypeImage, TypeVideo:
		return t
	default:
		return ""
	}
}

// SplitNovelChapters 按分隔符切分小说正文；空正文返回空切片。
// 连续分隔符（空段）自动跳过，分隔符本身不出现在输出中。
func SplitNovelChapters(content string) []NovelChapter {
	chapters := make([]NovelChapter, 0)
	if strings.TrimSpace(content) == "" {
		return chapters
	}
	for _, seg := range strings.Split(content, chapterSeparator) {
		html := strings.TrimSpace(seg)
		if html == "" {
			continue
		}
		chapters = append(chapters, NovelChapter{
			Index: len(chapters) + 1,
			Title: extractHeading(html),
			HTML:  html,
		})
	}
	return chapters
}

// extractHeading 提取段内首个 h1-h6 的文本作为章节标题，无标题返回空串
func extractHeading(html string) string {
	m := headingRegexp.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	title := strings.TrimSpace(tagRegexp.ReplaceAllString(m[1], ""))
	if title == "" {
		return ""
	}
	return title
}

// ClampChapter 将章节号钳制到 [1, total]，total 异常（<1）按单章处理
func ClampChapter(chapter, total int) int {
	if total < 1 {
		total = 1
	}
	if chapter < 1 {
		return 1
	}
	if chapter > total {
		return total
	}
	return chapter
}

// ParseVideoSources 从文章 Extends 的 "video_sources" 键解析选集列表，
// 跳过空 url/缺 url/非对象项；embed 缺省为 false。恒返回非 nil 切片。
func ParseVideoSources(ext vo.Extends) []VideoItem {
	list := make([]VideoItem, 0)
	raw := ext.Get("video_sources")
	if raw == nil {
		return list
	}
	arr, ok := raw.([]any)
	if !ok {
		return list
	}
	for _, v := range arr {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		url, _ := m["url"].(string)
		if strings.TrimSpace(url) == "" {
			continue
		}
		item := VideoItem{URL: url}
		if label, ok := m["label"].(string); ok {
			item.Label = label
		}
		if embed, ok := m["embed"].(bool); ok {
			item.Embed = embed
		}
		list = append(list, item)
	}
	return list
}

// ParseImageList 从文章 Extends 的 "gallery_images" 键解析有序图片列表，
// 过滤空串与非字符串项。恒返回非 nil 切片。
func ParseImageList(ext vo.Extends) []ImageItem {
	list := make([]ImageItem, 0)
	raw := ext.Get("gallery_images")
	if raw == nil {
		return list
	}
	arr, ok := raw.([]any)
	if !ok {
		return list
	}
	for _, v := range arr {
		url, ok := v.(string)
		if !ok || strings.TrimSpace(url) == "" {
			continue
		}
		list = append(list, ImageItem{URL: url})
	}
	return list
}
