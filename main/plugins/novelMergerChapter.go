package plugins

// NovelMerger 的纯函数层：章节号识别（含中文数字）、排序、正文拼装、slug 生成。
// 不依赖 config/service/DB，保证可单测；数据访问全部在 NovelMerger.go 中。

import (
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"moss/application/service/contenttype"
	"moss/domain/core/entity"
	"github.com/duke-git/lancet/v2/cryptor"
)

// 中文数字字符集（内置章节号正则的数字部分）
const chineseNumeralClass = `[零〇一二三四五六七八九十百千两]`

// builtinChapterPatterns 内置章节号识别模式，按序尝试，先命中先用。
// 卷/集/部放最后：卷号通常每卷重新计数，直接用于全局排序会误导顺序
var builtinChapterPatterns = []struct {
	Name string
	Re   *regexp.Regexp
}{
	{"builtin:第N章", regexp.MustCompile(`第\s*([0-9]+|` + chineseNumeralClass + `+)\s*章`)},
	{"builtin:第N节/回/话", regexp.MustCompile(`第\s*([0-9]+|` + chineseNumeralClass + `+)\s*[节回话]`)},
	{"builtin:chapter", regexp.MustCompile(`(?i)chapter\s*\.?\s*([0-9]{1,5})`)},
	{"builtin:尾数字", regexp.MustCompile(`(?:^|[\s\-_.·])([0-9]{1,5})$`)},
	{"builtin:第N卷/集/部", regexp.MustCompile(`第\s*([0-9]+|` + chineseNumeralClass + `+)\s*[卷集部]`)},
}

var chineseDigits = map[rune]int{
	'零': 0, '〇': 0,
	'一': 1, '二': 2, '两': 2, '三': 3, '四': 4,
	'五': 5, '六': 6, '七': 7, '八': 8, '九': 9,
}

var chineseUnits = map[rune]int{'十': 10, '百': 100, '千': 1000, '万': 10000}

// chineseNumeralToInt 中文数字转整数（支持到万级，如 一万二千=12000、两百=200、
// 一百零五=105）；纯 ASCII 数字串直接转换。非法或空串返回 false。
func chineseNumeralToInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(s); err == nil {
		if n <= 0 {
			return 0, false // 章节号从 1 起，非正数视为非法
		}
		return n, true
	}
	var total, section, number int
	for _, c := range s {
		if d, ok := chineseDigits[c]; ok {
			number = d
			continue
		}
		unit, ok := chineseUnits[c]
		if !ok {
			return 0, false
		}
		if unit == 10000 {
			// 万：当前段（含未乘单位的个位数）结算进万级
			total = (total + section + number) * unit
			section, number = 0, 0
			continue
		}
		if number == 0 {
			number = 1 // 裸单位开头：十X / 十
		}
		section += number * unit
		number = 0
	}
	result := total + section + number
	if result <= 0 {
		return 0, false
	}
	return result, true
}

// compileChapterPattern 编译自定义章节号正则，要求至少一个捕获组（第一组为章节号）
func compileChapterPattern(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	if re.NumSubexp() < 1 {
		return nil, errNoCaptureGroup
	}
	return re, nil
}

// extractChapterNo 从标题提取章节号：自定义正则优先（第一捕获组），
// 不命中或未提供时回退内置模式表。返回 0/false 表示未识别。
func extractChapterNo(title, customPattern string) (no int, matchedBy string, ok bool) {
	if customPattern != "" {
		if re, err := compileChapterPattern(customPattern); err == nil {
			if m := re.FindStringSubmatch(title); len(m) >= 2 {
				if n, parsed := parseChapterNumber(m[1]); parsed {
					return n, "custom", true
				}
			}
		}
	}
	for _, p := range builtinChapterPatterns {
		m := p.Re.FindStringSubmatch(title)
		if len(m) < 2 {
			continue
		}
		if n, parsed := parseChapterNumber(m[1]); parsed {
			return n, p.Name, true
		}
	}
	return 0, "", false
}

// parseChapterNumber 捕获组内容 → 章节号：先按 ASCII 数字再按中文数字，
// 结果必须为正数（0 视为未识别）
func parseChapterNumber(s string) (int, bool) {
	n, ok := chineseNumeralToInt(s)
	if !ok || n <= 0 {
		return 0, false
	}
	return n, true
}

// sortMergeChapters 就地稳定排序：已识别在前按（章节号, 时间, ID）升序，
// 未识别排尾按（时间, ID）升序；回填 SortIndex（从 1 起）。
func sortMergeChapters(items []NovelMergeChapterItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Recognized != b.Recognized {
			return a.Recognized // 已识别在前
		}
		if a.Recognized && b.Recognized && a.ChapterNo != b.ChapterNo {
			return a.ChapterNo < b.ChapterNo
		}
		if a.CreateTime != b.CreateTime {
			return a.CreateTime < b.CreateTime
		}
		return a.ID < b.ID
	})
	for i := range items {
		items[i].SortIndex = i + 1
	}
}

// duplicateChapterNos 返回出现多于一次的章节号（升序去重），空输入返回空切片
func duplicateChapterNos(items []NovelMergeChapterItem) []int {
	counts := make(map[int]int)
	for _, it := range items {
		if it.Recognized {
			counts[it.ChapterNo]++
		}
	}
	res := make([]int, 0)
	for no, count := range counts {
		if count > 1 {
			res = append(res, no)
		}
	}
	sort.Ints(res)
	return res
}

// majorityCategoryID 多数决取分类：章节数最多的分类；平票取 CreateTime 最早
// （再平取 ID 小）的分类。空输入返回 0。
func majorityCategoryID(items []NovelMergeChapterItem) int {
	type stat struct {
		count          int
		minCreateTime  int64
		minID          int
	}
	stats := make(map[int]*stat)
	for _, it := range items {
		s, exists := stats[it.CategoryID]
		if !exists {
			s = &stat{minCreateTime: it.CreateTime, minID: it.ID}
			stats[it.CategoryID] = s
		}
		s.count++
		if it.CreateTime < s.minCreateTime || (it.CreateTime == s.minCreateTime && it.ID < s.minID) {
			s.minCreateTime = it.CreateTime
			s.minID = it.ID
		}
	}
	winner, best := 0, (*stat)(nil)
	for id, s := range stats {
		if best == nil || s.count > best.count ||
			(s.count == best.count && (s.minCreateTime < best.minCreateTime ||
				(s.minCreateTime == best.minCreateTime && s.minID < best.minID))) {
			winner, best = id, s
		}
	}
	return winner
}

// buildNovelContent 按 articles 顺序拼装多章正文：每段 TrimSpace，
// 段内无 h1-h6 标题时包 <h2>文章标题</h2>（否则前台目录会全部回退为"第N章"），
// 段间以 ===chapter=== 分隔。与 contenttype.SplitNovelChapters 往返一致。
func buildNovelContent(articles []entity.Article) string {
	segs := make([]string, 0, len(articles))
	for _, a := range articles {
		seg := strings.TrimSpace(a.Content)
		if seg == "" {
			continue // 执行层预检已拒绝空正文，此处兜底跳过
		}
		if contenttype.ExtractHeading(seg) == "" {
			seg = "<h2>" + html.EscapeString(a.Title) + "</h2>\n" + seg
		}
		segs = append(segs, seg)
	}
	return strings.Join(segs, "\n"+contenttype.ChapterSeparator+"\n")
}

// buildNovelSlug 由小说名生成稳定 slug（标题 hash 前 12 位，GnDownSpider 同款先例）
func buildNovelSlug(title string) string {
	title = strings.TrimSpace(strings.ToLower(title))
	if title == "" {
		return ""
	}
	return cryptor.Md5String(title)[:12]
}
