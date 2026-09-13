package plugins

// NovelMerger 纯函数层单测：中文数字、章节号识别、排序、正文拼装
// （与 contenttype.SplitNovelChapters 往返一致）、slug 生成。
// 只依赖纯包，不触发 config/service 初始化。

import (
	"regexp"
	"testing"

	"moss/application/service/contenttype"
	"moss/domain/core/entity"
)

func TestChineseNumeralToInt(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"7", 7, true},
		{"12345", 12345, true},
		{"七", 7, true},
		{"十", 10, true},
		{"二十三", 23, true},
		{"一百零五", 105, true},
		{"一千零一", 1001, true},
		{"三千五百", 3500, true},
		{"一万二千", 12000, true},
		{"两百", 200, true},
		{"〇", 0, false}, // 结果为 0 视为非法
		{"", 0, false},
		{"abc", 0, false},
		{"一x三", 0, false},
		{"0", 0, false}, // 0 视为非法（章节号从 1 起）
	}
	for _, c := range cases {
		got, ok := chineseNumeralToInt(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("chineseNumeralToInt(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestExtractChapterNo(t *testing.T) {
	cases := []struct {
		title  string
		pattern string
		wantNo int
		wantOK bool
	}{
		{"斗破苍穹 第1章 起点", "", 1, true},
		{"斗破苍穹 第 12 章 异火", "", 12, true},
		{"斗破苍穹 第二章 退婚", "", 2, true},
		{"斗破苍穹 第 两千零一 章 大结局", "", 2001, true},
		{"凡人修仙传 第3节 灵根", "", 3, true},
		{"凡人修仙传 第十回 拜师", "", 10, true},
		{"My Novel Chapter 12 Trial", "", 12, true},
		{"my novel chapter.3", "", 3, true},
		{"斗破苍穹 1024", "", 1024, true},
		{"斗破苍穹 1024（大结局）", "", 0, false}, // 尾部非纯数字
		{"斗破苍穹 第3卷", "", 3, true},         // 卷兜底
		{"随便一篇文章", "", 0, false},
		{"斗破苍穹 EP05 试炼", "", 0, false},
		// 自定义正则优先
		{"斗破苍穹 EP05 试炼", `EP\s*(\d+)`, 5, true},
		// 自定义不命中 → 回退内置
		{"斗破苍穹 第9章 天焚炼气塔", `EP\s*(\d+)`, 9, true},
		// 第N章 优先于尾数字
		{"斗破苍穹 第3章 之 100 篇外传", "", 3, true},
	}
	for _, c := range cases {
		no, _, ok := extractChapterNo(c.title, c.pattern)
		if ok != c.wantOK || (ok && no != c.wantNo) {
			t.Errorf("extractChapterNo(%q, %q) = (%d, %v), want (%d, %v)",
				c.title, c.pattern, no, ok, c.wantNo, c.wantOK)
		}
	}
}

func TestCompileChapterPattern(t *testing.T) {
	if re, err := compileChapterPattern(""); re != nil || err != nil {
		t.Errorf("empty pattern should return (nil, nil)")
	}
	if _, err := compileChapterPattern(`\d+`); err == nil {
		t.Errorf("pattern without capture group should error")
	}
	if _, err := compileChapterPattern(`(`); err == nil {
		t.Errorf("invalid pattern should error")
	}
	if _, err := compileChapterPattern(`第(\d+)章`); err != nil {
		t.Errorf("valid pattern should not error: %v", err)
	}
}

func TestSortMergeChapters(t *testing.T) {
	items := []NovelMergeChapterItem{
		{ID: 1, Title: "第3章", ChapterNo: 3, Recognized: true, CreateTime: 300},
		{ID: 2, Title: "第1章", ChapterNo: 1, Recognized: true, CreateTime: 200},
		{ID: 3, Title: "未知章节B", ChapterNo: 0, Recognized: false, CreateTime: 500},
		{ID: 4, Title: "第1章重复", ChapterNo: 1, Recognized: true, CreateTime: 100}, // 同号取时间早
		{ID: 5, Title: "第2章", ChapterNo: 2, Recognized: true, CreateTime: 400},
		{ID: 6, Title: "未知章节A", ChapterNo: 0, Recognized: false, CreateTime: 450}, // 未识别按时间排尾
	}
	sortMergeChapters(items)
	wantIDs := []int{4, 2, 5, 1, 6, 3}
	for i, want := range wantIDs {
		if items[i].ID != want {
			t.Fatalf("sortMergeChapters[%d].ID = %d, want %d (full: %v)", i, items[i].ID, want, items)
		}
		if items[i].SortIndex != i+1 {
			t.Errorf("SortIndex = %d, want %d", items[i].SortIndex, i+1)
		}
	}

	dups := duplicateChapterNos(items)
	if len(dups) != 1 || dups[0] != 1 {
		t.Errorf("duplicateChapterNos = %v, want [1]", dups)
	}
}

func TestMajorityCategoryID(t *testing.T) {
	if got := majorityCategoryID(nil); got != 0 {
		t.Errorf("empty input should return 0")
	}
	items := []NovelMergeChapterItem{
		{ID: 1, CategoryID: 5, CreateTime: 300},
		{ID: 2, CategoryID: 5, CreateTime: 200},
		{ID: 3, CategoryID: 3, CreateTime: 100},
	}
	if got := majorityCategoryID(items); got != 5 {
		t.Errorf("majority = %d, want 5", got)
	}
	// 平票取 CreateTime 最早
	items = []NovelMergeChapterItem{
		{ID: 1, CategoryID: 7, CreateTime: 300},
		{ID: 2, CategoryID: 9, CreateTime: 100},
	}
	if got := majorityCategoryID(items); got != 9 {
		t.Errorf("tie-break = %d, want 9", got)
	}
}

func TestBuildNovelContentRoundTrip(t *testing.T) {
	articles := []entity.Article{
		{ // 正文已有标题标签 → 不重复包
			ArticleBase:  entity.ArticleBase{ID: 1, Title: "斗破苍穹 第1章 起点"},
			ArticleDetail: entity.ArticleDetail{Content: "<h2>第1章 起点</h2><p>甲</p>"},
		},
		{ // 正文无标题 → 包 h2，标题取文章 Title
			ArticleBase:  entity.ArticleBase{ID: 2, Title: "斗破苍穹 第2章 转折"},
			ArticleDetail: entity.ArticleDetail{Content: "<p>乙</p>"},
		},
		{ // 正文标题与文章标题不同 → 保留正文原标题
			ArticleBase:  entity.ArticleBase{ID: 3, Title: "斗破苍穹 第3章"},
			ArticleDetail: entity.ArticleDetail{Content: "<h3>风起云涌</h3><p>丙</p>"},
		},
	}
	content := buildNovelContent(articles)
	if !regexp.MustCompile(regexp.QuoteMeta(contenttype.ChapterSeparator)).MatchString(content) {
		t.Fatalf("content missing chapter separator: %s", content)
	}

	chapters := contenttype.SplitNovelChapters(content)
	if len(chapters) != 3 {
		t.Fatalf("SplitNovelChapters count = %d, want 3 (content: %s)", len(chapters), content)
	}
	wantTitles := []string{"第1章 起点", "斗破苍穹 第2章 转折", "风起云涌"}
	wantHTML := []string{"<h2>第1章 起点</h2><p>甲</p>", "<h2>斗破苍穹 第2章 转折</h2>\n<p>乙</p>", "<h3>风起云涌</h3><p>丙</p>"}
	for i, ch := range chapters {
		if ch.Index != i+1 {
			t.Errorf("chapter[%d].Index = %d", i, ch.Index)
		}
		if ch.Title != wantTitles[i] {
			t.Errorf("chapter[%d].Title = %q, want %q", i, ch.Title, wantTitles[i])
		}
		if ch.HTML != wantHTML[i] {
			t.Errorf("chapter[%d].HTML = %q, want %q", i, ch.HTML, wantHTML[i])
		}
	}
}

func TestBuildNovelSlug(t *testing.T) {
	if got := buildNovelSlug("斗破苍穹"); len(got) != 12 {
		t.Errorf("slug length = %d, want 12", len(got))
	}
	if buildNovelSlug("斗破苍穹") != buildNovelSlug(" 斗破苍穹 ") {
		t.Errorf("slug should be stable after trim")
	}
	if buildNovelSlug("斗破苍穹") == buildNovelSlug("武动乾坤") {
		t.Errorf("different titles should produce different slugs")
	}
	if buildNovelSlug("") != "" {
		t.Errorf("empty title should produce empty slug")
	}
}

func TestParseNovelMergeRequest(t *testing.T) {
	if _, err := parseNovelMergeRequest(""); err == nil {
		t.Errorf("empty body should error")
	}
	if _, err := parseNovelMergeRequest(`{"keyword":""}`); err == nil {
		t.Errorf("empty keyword should error")
	}
	if _, err := parseNovelMergeRequest(`{"keyword":"测试","pattern":"("}`); err == nil {
		t.Errorf("invalid pattern should error")
	}
	if _, err := parseNovelMergeRequest(`{"keyword":"测试","pattern":"\\d+"}`); err == nil {
		t.Errorf("pattern without capture group should error")
	}
	req, err := parseNovelMergeRequest(`{"keyword":" 测试 ","pattern":" 第(\\d+)章 ","ids":[3,1,2]}`)
	if err != nil {
		t.Fatalf("valid request should parse: %v", err)
	}
	if req.Keyword != "测试" || req.Pattern != "第(\\d+)章" || len(req.IDs) != 3 {
		t.Errorf("unexpected parsed request: %+v", req)
	}
}
