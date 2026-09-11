package plugins

// HttpSpider / HeadlessSpider 共用的采集逻辑（纯函数，无 DOM/浏览器依赖）：
// 统一提取模型（正则编译/抽取）、链接过滤与 URL 模板、去重 slug、
// 任务统计、文章构建辅助（发布时间解析/摘要/截断/视频源组装）。
// 两个爬虫插件的 *_sel/*_attr/*_extract_regex 字段约定在此对齐，保持跨插件配置可平移。

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// taskStats 任务内累计计数（processLink/processArticle 更新）；
// stopTask 为跨页早停标志（stop_when_exists/limit 触发后终止整个任务翻页，mu 保护）
type taskStats struct {
	collected, skipped, failed, consecExists int
	stopTask                                 bool
}

// compileExtractRegex 编译抽取正则（link/cover/gallery/extra 的 *_extract_regex、regex 共用）；
// 兼容 /re/ 包裹写法（与 link_include 过滤约定一致）
func compileExtractRegex(pattern string) (*regexp.Regexp, error) {
	if len(pattern) > 2 && strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
		pattern = pattern[1 : len(pattern)-1]
	}
	return regexp.Compile(pattern)
}

// applyExtractRegex 从属性/文本值中抽取目标：pattern 空=原样返回（仅去空白）；
// 取第一个捕获组，无捕获组取整段匹配；未命中或正则非法返回空串
func applyExtractRegex(pattern, value string) string {
	if pattern == "" {
		return strings.TrimSpace(value)
	}
	re, err := compileExtractRegex(pattern)
	if err != nil {
		return ""
	}
	m := re.FindStringSubmatch(value)
	if m == nil {
		return ""
	}
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(m[0])
}

// matchFilter 链接过滤：include 为空=放行；include/exclude 支持子串或 /正则/ 形式
func matchFilter(link, include, exclude string) bool {
	if include != "" && !matchOne(link, include) {
		return false
	}
	if exclude != "" && matchOne(link, exclude) {
		return false
	}
	return true
}

func matchOne(s, pattern string) bool {
	if len(pattern) > 2 && strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
		if re, err := regexp.Compile(pattern[1 : len(pattern)-1]); err == nil {
			return re.MatchString(s)
		}
	}
	return strings.Contains(s, pattern)
}

// buildLinkURL 按 link_url_template 生成最终 URL：
// 含 {value} 占位则替换为属性值，否则视为前缀直接拼接；模板为空原样返回
func buildLinkURL(template, value string) string {
	if template == "" {
		return value
	}
	if strings.Contains(template, "{value}") {
		return strings.ReplaceAll(template, "{value}", value)
	}
	return template + value
}

// sameSite 判断两个 host 是否同一站点：忽略 www. 前缀，允许子域互认
// （example.tv 与 www.example.tv / m.example.tv 视为同源）
func sameSite(a, b string) bool {
	a, b = strings.ToLower(strings.TrimPrefix(a, "www.")), strings.ToLower(strings.TrimPrefix(b, "www."))
	return a == b || strings.HasSuffix(a, "."+b) || strings.HasSuffix(b, "."+a)
}

// hashSlug 以 源URL+标题 生成稳定去重 slug（多源防冲突）
func hashSlug(link, title string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(link) + "|" + strings.TrimSpace(title)))
	return hex.EncodeToString(sum[:8])
}

// truncateRunes 按 rune 截断到 n 长度（数据库字段长度保护）
func truncateRunes(s string, n int) string {
	rs := []rune(strings.TrimSpace(s))
	if len(rs) > n {
		return string(rs[:n])
	}
	return string(rs)
}

// sanitizeFilename 文件名替换路径分隔与空白等字符
func sanitizeFilename(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ', '\t':
			return '_'
		}
		return r
	}, s)
}

// buildVideoSources 组装 video_sources：labels 与 srcs 按下标对应（空/缺失回退「第NN集」，
// 编号含 idxOffset 以跨直链/iframe 通道连续）；embed=true 标记第三方 iframe 播放页
func buildVideoSources(srcs, labels []string, embed bool, idxOffset int) []map[string]any {
	sources := make([]map[string]any, 0, len(srcs))
	for i, src := range srcs {
		label := ""
		if i < len(labels) {
			label = strings.TrimSpace(labels[i])
		}
		if label == "" {
			label = fmt.Sprintf("第%02d集", idxOffset+i+1)
		}
		sources = append(sources, map[string]any{"label": label, "url": src, "embed": embed})
	}
	return sources
}

// parsePublishTime 从任意文本抽取日期时间转 unix 秒：支持 2024-05-01 12:30:45 / 2024/5/1 /
// 2024.05.01 / 2024年5月1日(08:05) 等；时区按服务器本地（排序/sitemap 用，不追求绝对时区精确）；
// 解析失败返回 0
var publishDateRegexp = regexp.MustCompile(`(\d{4})\s*[-/.年]\s*(\d{1,2})\s*[-/.月]\s*(\d{1,2})日?(?:[\sTt]+(\d{1,2}):(\d{1,2})(?::(\d{1,2}))?)?`)

func parsePublishTime(s string) int64 {
	m := publishDateRegexp.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	num := func(i int) int {
		v, _ := strconv.Atoi(m[i])
		return v
	}
	y, mo, d, hh, mm, ss := num(1), num(2), num(3), num(4), num(5), num(6)
	if y < 1990 || y > 2100 || mo < 1 || mo > 12 || d < 1 || d > 31 || hh > 23 || mm > 59 || ss > 59 {
		return 0
	}
	t := time.Date(y, time.Month(mo), d, hh, mm, ss, 0, time.Local)
	if t.Year() != y || int(t.Month()) != mo || t.Day() != d {
		return 0 // 归一化失败（如 2 月 30 日）
	}
	return t.Unix()
}

var htmlTagRegexp = regexp.MustCompile(`<[^>]+>`)
var whitespaceRegexp = regexp.MustCompile(`\s+`)

// plainSummary 去标签截取摘要
func plainSummary(html string, n int) string {
	s := htmlTagRegexp.ReplaceAllString(html, "")
	s = whitespaceRegexp.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n])
	}
	return s
}
