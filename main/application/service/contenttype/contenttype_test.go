package contenttype

import (
	"strings"
	"testing"

	"moss/domain/core/vo"

	"github.com/microcosm-cc/bluemonday"
)

// ---------- 内容类型规范化 ----------

func TestNormalizeContentType(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},                        // 未设置 → 普通
		{"standard", "standard"},        // 显式普通
		{"novel", "novel"},
		{"image", "image"},
		{"video", "video"},
		{"NOVEL", ""},                   // 大小写敏感，非法值 → 普通
		{"movie", ""},                   // 未知值 → 普通
		{" video ", ""},                 // 带空白 → 非法 → 普通
	}
	for _, c := range cases {
		if got := NormalizeContentType(c.in); got != c.want {
			t.Errorf("NormalizeContentType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---------- 小说章节切分（分隔符 ===chapter===） ----------

func TestSplitNovelChapters_NoSeparator(t *testing.T) {
	chs := SplitNovelChapters("<p>只有一个章节的内容</p>")
	if len(chs) != 1 {
		t.Fatalf("无分隔符应得 1 章，got %d", len(chs))
	}
	if chs[0].Index != 1 || chs[0].Title != "" {
		t.Errorf("章节元数据错误: %+v", chs[0])
	}
	if !strings.Contains(chs[0].HTML, "只有一个章节的内容") {
		t.Errorf("正文内容丢失: %q", chs[0].HTML)
	}
}

func TestSplitNovelChapters_Multiple(t *testing.T) {
	content := "<h2>第一章 起点</h2><p>甲……</p>\n===chapter===\n<h2>第二章 转折</h2><p>乙……</p>\n===chapter===\n<p>尾声，无标题章</p>"
	chs := SplitNovelChapters(content)
	if len(chs) != 3 {
		t.Fatalf("应得 3 章，got %d", len(chs))
	}
	if chs[0].Index != 1 || chs[0].Title != "第一章 起点" {
		t.Errorf("第 1 章元数据错误: %+v", chs[0])
	}
	if chs[1].Index != 2 || chs[1].Title != "第二章 转折" {
		t.Errorf("第 2 章元数据错误: %+v", chs[1])
	}
	if chs[2].Index != 3 || chs[2].Title != "" {
		t.Errorf("第 3 章无标题应得空 Title: %+v", chs[2])
	}
	if !strings.Contains(chs[2].HTML, "尾声") {
		t.Errorf("第 3 章正文丢失: %q", chs[2].HTML)
	}
}

func TestSplitNovelChapters_ConsecutiveSeparators(t *testing.T) {
	chs := SplitNovelChapters("<p>开头</p>\n===chapter===\n\n===chapter===\n<p>结尾</p>")
	if len(chs) != 2 {
		t.Fatalf("连续分隔符应跳过空段得 2 章，got %d", len(chs))
	}
}

func TestSplitNovelChapters_SeparatorStripped(t *testing.T) {
	chs := SplitNovelChapters("<p>前</p>===chapter===<p>后</p>")
	for _, ch := range chs {
		if strings.Contains(ch.HTML, "===chapter===") {
			t.Errorf("分隔符残留在输出章节中: %q", ch.HTML)
		}
	}
}

func TestSplitNovelChapters_EmptyContent(t *testing.T) {
	chs := SplitNovelChapters("")
	if len(chs) != 0 {
		t.Fatalf("空正文应得 0 章（由调用方决定占位），got %d", len(chs))
	}
}

// ---------- 章节号钳制 ----------

func TestClampChapter(t *testing.T) {
	cases := []struct{ chapter, total, want int }{
		{1, 3, 1}, {3, 3, 3}, {2, 3, 2},
		{0, 3, 1},   // 下越界 → 1
		{-5, 3, 1},  // 负数 → 1
		{4, 3, 3},   // 上越界 → 总章数
		{99, 3, 3},  // 远超 → 总章数
		{1, 0, 1},   // total 异常按单章处理
		{5, -1, 1},  // total 异常按单章处理
	}
	for _, c := range cases {
		if got := ClampChapter(c.chapter, c.total); got != c.want {
			t.Errorf("ClampChapter(%d, %d) = %d, want %d", c.chapter, c.total, got, c.want)
		}
	}
}

// ---------- 视频选集解析 ----------

func mkExtends(kv map[string]any) vo.Extends {
	ext := vo.Extends{}
	for k, v := range kv {
		ext = append(ext, vo.ExtendsItem{Key: k, Value: v})
	}
	return ext
}

func TestParseVideoSources(t *testing.T) {
	ext := mkExtends(map[string]any{
		"video_sources": []any{
			map[string]any{"label": "第01集", "url": "https://cdn.example.com/a.mp4"},
			map[string]any{"label": "第02集", "url": "https://play.example.com/2", "embed": true},
			map[string]any{"label": "空地址应跳过", "url": ""},
			map[string]any{"label": "缺 url 应跳过"},
			"not-a-map",
		},
	})
	list := ParseVideoSources(ext)
	if len(list) != 2 {
		t.Fatalf("应解析出 2 个有效选集，got %d: %+v", len(list), list)
	}
	if list[0].Label != "第01集" || list[0].URL != "https://cdn.example.com/a.mp4" || list[0].Embed {
		t.Errorf("第 1 集字段错误: %+v", list[0])
	}
	if !list[1].Embed {
		t.Errorf("第 2 集 embed 应为 true: %+v", list[1])
	}
}

func TestParseVideoSources_EmbedDefaultFalse(t *testing.T) {
	ext := mkExtends(map[string]any{
		"video_sources": []any{map[string]any{"label": "EP1", "url": "https://x/y.mp4", "embed": false}},
	})
	list := ParseVideoSources(ext)
	if len(list) != 1 || list[0].Embed {
		t.Errorf("embed 缺省/显式 false 均应为 false: %+v", list)
	}
}

func TestParseVideoSources_MissingOrMalformed(t *testing.T) {
	if got := ParseVideoSources(vo.Extends{}); got == nil || len(got) != 0 {
		t.Errorf("无键应返回空切片（非 nil）: %v", got)
	}
	if got := ParseVideoSources(mkExtends(map[string]any{"video_sources": "不是数组"})); len(got) != 0 {
		t.Errorf("非数组值应返回空切片: %+v", got)
	}
}

// ---------- 图集解析 ----------

func TestParseImageList(t *testing.T) {
	ext := mkExtends(map[string]any{
		"gallery_images": []any{"https://img/1.jpg", "", "https://img/2.jpg", 123},
	})
	list := ParseImageList(ext)
	if len(list) != 2 {
		t.Fatalf("应过滤空串与非字符串得 2 张，got %d: %+v", len(list), list)
	}
	if list[0].URL != "https://img/1.jpg" || list[1].URL != "https://img/2.jpg" {
		t.Errorf("图集顺序错误: %+v", list)
	}
	if got := ParseImageList(vo.Extends{}); got == nil || len(got) != 0 {
		t.Errorf("无键应返回空切片（非 nil）: %v", got)
	}
}

// ---------- 分隔符经 bluemonday UGCPolicy 清洗的存活验证 ----------
// research.md 风险 1：HTML 注释分隔符会被清洗剥离，因此采用纯文本分隔符。
// 本测试固化该依据：===chapter=== 不会被清洗剥离。

func TestSeparatorSurvivesBluemondayUGCPolicy(t *testing.T) {
	p := bluemonday.UGCPolicy()
	content := "<p>第一章</p>\n===chapter===\n<p>第二章</p>"
	out := p.Sanitize(content)
	if !strings.Contains(out, "===chapter===") {
		t.Fatalf("纯文本分隔符应经清洗存活: %q", out)
	}
	// 对照：HTML 注释分隔符会被剥离（这就是不采用 <!--chapter--> 的原因）
	if strings.Contains(p.Sanitize("<p>a</p><!--chapter--><p>b</p>"), "<!--chapter-->") {
		t.Fatal("HTML 注释应被 UGCPolicy 剥离（若未剥离可考虑注释分隔符）")
	}
}
