package plugins

import (
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// httpDoc 用 HTML 片段构造 goquery 文档（解析层测试不碰网络）
func httpDoc(html string) *goquery.Document {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		panic(err)
	}
	return doc
}

// httpMustURL 解析测试用基准 URL，失败直接 Fatal（测试前置条件）
func httpMustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func TestHttpParseTasks(t *testing.T) {
	// 合法配置：默认值与 TrimSpace（后台手填易带空格）
	h := &HttpSpider{Tasks: `[{"name":" 站点A ","enable":true,"source_url":" https://a.tv/list ",` +
		`"list_selector":" .item ","encoding":" gbk "}]`}
	tasks, err := h.parseTasks()
	if err != nil {
		t.Fatalf("valid config should parse: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(tasks))
	}
	tk := tasks[0]
	if tk.Name != "站点A" || tk.SourceURL != "https://a.tv/list" || tk.ListSelector != ".item" || tk.Encoding != "gbk" {
		t.Fatalf("trim failed: %+v", tk)
	}
	if tk.Mode != "detail" {
		t.Errorf("default mode = %q, want detail", tk.Mode)
	}
	if tk.MaxPages != 1 {
		t.Errorf("default max_pages = %d, want 1", tk.MaxPages)
	}

	// next_page_sel 模式不填 max_pages 给安全上限 50
	h.Tasks = `[{"name":"x","enable":true,"source_url":"https://a.tv/","list_selector":".item","next_page_sel":".next"}]`
	if tasks, err = h.parseTasks(); err != nil {
		t.Fatalf("next_page task should parse: %v", err)
	}
	if tasks[0].MaxPages != 50 {
		t.Errorf("next_page_sel default max_pages = %d, want 50", tasks[0].MaxPages)
	}

	// 必填校验
	for _, c := range []struct{ name, task string }{
		{"缺 source_url", `{"name":"x","enable":true,"list_selector":".item"}`},
		{"缺 list_selector", `{"name":"x","enable":true,"source_url":"https://a.tv/"}`},
	} {
		h.Tasks = "[" + c.task + "]"
		if _, err := h.parseTasks(); err == nil {
			t.Errorf("%s should fail parseTasks", c.name)
		}
	}

	// mode / dedup_by 非法值
	h.Tasks = `[{"name":"x","enable":true,"source_url":"https://a.tv/","list_selector":".i","mode":"js"}]`
	if _, err := h.parseTasks(); err == nil {
		t.Error("invalid mode should fail")
	}
	h.Tasks = `[{"name":"x","enable":true,"source_url":"https://a.tv/","list_selector":".i","dedup_by":"title"}]`
	if _, err := h.parseTasks(); err == nil {
		t.Error("invalid dedup_by should fail")
	}

	// 正则非法在解析期报错
	h.Tasks = `[{"name":"x","enable":true,"source_url":"https://a.tv/","list_selector":".i","title_extract_regex":"("}]`
	if _, err := h.parseTasks(); err == nil {
		t.Error("invalid title_extract_regex should fail")
	}

	// link_include/link_exclude 的 /re/ 包裹正则在解析期校验；非包裹写法按子串匹配不校验
	h.Tasks = `[{"name":"x","enable":true,"source_url":"https://a.tv/","list_selector":".i","link_include":"/detail-(\d+/"}]`
	if _, err := h.parseTasks(); err == nil {
		t.Error("invalid /re/ link_include should fail")
	}
	h.Tasks = `[{"name":"x","enable":true,"source_url":"https://a.tv/","list_selector":".i","link_exclude":"/[/"}]`
	if _, err := h.parseTasks(); err == nil {
		t.Error("invalid /re/ link_exclude should fail")
	}
	h.Tasks = `[{"name":"x","enable":true,"source_url":"https://a.tv/","list_selector":".i","link_include":"detail(子串"}]`
	if _, err := h.parseTasks(); err != nil {
		t.Errorf("substring-style link_include should not be regex-validated: %v", err)
	}
	h.Tasks = `[{"name":"x","enable":true,"source_url":"https://a.tv/","list_selector":".i","link_include":"/detail-\\d+/","link_exclude":"/\\?from=/"}]`
	if _, err := h.parseTasks(); err != nil {
		t.Errorf("valid /re/ filters should parse: %v", err)
	}

	// headers 非法 JSON 报错
	h.Tasks = `[{"name":"x","enable":true,"source_url":"https://a.tv/","list_selector":".i","headers":"{bad"}]`
	if _, err := h.parseTasks(); err == nil {
		t.Error("invalid headers json should fail")
	}

	// 空 tasks / 非法 JSON
	if _, err := (&HttpSpider{Tasks: ""}).parseTasks(); err == nil {
		t.Error("empty tasks should fail")
	}
	if _, err := (&HttpSpider{Tasks: `[bad`}).parseTasks(); err == nil {
		t.Error("invalid json should fail")
	}
}

func TestHttpPageURL(t *testing.T) {
	// 第 1 页始终返回起始页
	tk := &httpTask{SourceURL: "https://a.tv/list", PageURLPattern: "https://a.tv/list_{page}.html"}
	if got := tk.pageURL(1); got != "https://a.tv/list" {
		t.Errorf("pageURL(1) = %q, want source_url", got)
	}
	if got := tk.pageURL(3); got != "https://a.tv/list_3.html" {
		t.Errorf("pageURL(3) = %q, want template substitution", got)
	}

	// 无模板：任何页码都返回起始页
	tk2 := &httpTask{SourceURL: "https://a.tv/list"}
	if got := tk2.pageURL(5); got != "https://a.tv/list" {
		t.Errorf("no pattern pageURL(5) = %q, want source_url", got)
	}
}

func TestHttpParseHeaders(t *testing.T) {
	// 空/空白配置：nil 无错误
	if hd, err := parseHeaders(""); hd != nil || err != nil {
		t.Fatalf("empty config: hd=%v err=%v", hd, err)
	}
	if hd, err := parseHeaders("   "); hd != nil || err != nil {
		t.Fatalf("blank config: hd=%v err=%v", hd, err)
	}

	hd, err := parseHeaders(`{"Referer":"https://a.tv/","X-Token":"abc"}`)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(hd) != 2 || hd["Referer"] != "https://a.tv/" || hd["X-Token"] != "abc" {
		t.Fatalf("headers wrong: %v", hd)
	}

	// 非法 JSON / 非对象结构
	if _, err := parseHeaders(`{bad`); err == nil {
		t.Error("invalid json should error")
	}
	if _, err := parseHeaders(`["a","b"]`); err == nil {
		t.Error("non-object json should error")
	}
}

func TestHttpDecodeBody(t *testing.T) {
	text := "中文标题 Hello"
	utf8Bytes := []byte(text)
	gbkBytes, err := simplifiedchinese.GBK.NewEncoder().Bytes(utf8Bytes)
	if err != nil {
		t.Fatalf("encode gbk: %v", err)
	}

	// UTF-8 原样（header 声明）
	if got, err := decodeHTMLBody(utf8Bytes, "text/html; charset=utf-8", ""); err != nil || got != text {
		t.Errorf("utf8 via header: got %q err %v", got, err)
	}

	// header 声明 GBK
	if got, err := decodeHTMLBody(gbkBytes, "text/html; charset=gbk", ""); err != nil || got != text {
		t.Errorf("gbk via header: got %q err %v", got, err)
	}

	// meta 声明 GBK（header 无 charset）：整个文档应按 GBK 解码出正文
	metaHTML := append([]byte(`<html><head><meta http-equiv="Content-Type" content="text/html; charset=gbk"></head><body>`), gbkBytes...)
	if got, err := decodeHTMLBody(metaHTML, "text/html", ""); err != nil || !strings.Contains(got, text) {
		t.Errorf("gbk via meta: got %q err %v", got, err)
	}

	// 无任何声明：非 UTF-8 字节嗅探回退 GBK
	if got, err := decodeHTMLBody(gbkBytes, "text/html", ""); err != nil || got != text {
		t.Errorf("gbk sniffed: got %q err %v", got, err)
	}

	// 强制指定编码：优先于 header 声明
	if got, err := decodeHTMLBody(gbkBytes, "text/html; charset=utf-8", "gbk"); err != nil || got != text {
		t.Errorf("forced gbk: got %q err %v", got, err)
	}

	// force 非法编码名报错
	if _, err := decodeHTMLBody(utf8Bytes, "text/html", "not-an-encoding"); err == nil {
		t.Error("invalid force encoding should error")
	}
}

func TestHttpExtractLinks(t *testing.T) {
	html := `<html><body>
		<div class="item"><a href="/video/1"><h2>标题一</h2></a></div>
		<div class="item"><a href="video/2"><h3>标题二</h3></a></div>
		<div class="item"><a href="https://b.tv/other"><h2>外链条目</h2></a></div>
		<div class="item"><a href="/video/1">重复链接</a></div>
		<div class="item"><a href="javascript:void(0)"><h2>JS链接</h2></a></div>
		<div class="item"><a href="/page/2"><h2>分页链接</h2></a></div>
		<div class="item">纯文本无链接</div>
	</body></html>`
	doc := httpDoc(html)
	base := httpMustURL(t, "https://a.tv/list")

	tk := &httpTask{SourceURL: "https://a.tv/list", ListSelector: ".item", LinkExclude: "/page/"}
	links := tk.extractLinks(doc, base)

	// 期望：/video/1、video/2 绝对化、外链保留；重复去重、javascript 跳过、/page/ 被排除、无链接跳过
	want := []struct{ url, title string }{
		{"https://a.tv/video/1", "标题一"},
		{"https://a.tv/video/2", "标题二"},
		{"https://b.tv/other", "外链条目"},
	}
	if len(links) != len(want) {
		t.Fatalf("links = %d, want %d: %+v", len(links), len(want), links)
	}
	for i, w := range want {
		if links[i].URL != w.url || links[i].Title != w.title {
			t.Errorf("links[%d] = {%q %q}, want {%q %q}", i, links[i].URL, links[i].Title, w.url, w.title)
		}
	}

	// include 过滤
	tk.LinkInclude = "/video/"
	tk.LinkExclude = ""
	links = tk.extractLinks(doc, base)
	if len(links) != 2 {
		t.Fatalf("include filter: links = %d, want 2", len(links))
	}
}

func TestHttpExtractLinkSelectors(t *testing.T) {
	// link_selector 定位条目内链接元素 + link_attr 读属性 + template 拼接（对齐 HeadlessSpider 通道）
	html := `<html><body>
		<div class="item" data-id="101"><h2>条目一</h2></div>
		<div class="item" data-id="102"><h2>条目二</h2></div>
	</body></html>`
	doc := httpDoc(html)
	base := httpMustURL(t, "https://a.tv/list")

	tk := &httpTask{
		SourceURL:       "https://a.tv/list",
		ListSelector:    ".item",
		LinkAttr:        "data-id",
		LinkURLTemplate: "https://a.tv/video/{value}.html",
	}
	links := tk.extractLinks(doc, base)
	if len(links) != 2 {
		t.Fatalf("links = %d, want 2: %+v", len(links), links)
	}
	if links[0].URL != "https://a.tv/video/101.html" || links[0].Title != "条目一" {
		t.Fatalf("links[0] = %+v", links[0])
	}
	if links[1].URL != "https://a.tv/video/102.html" {
		t.Fatalf("links[1] = %+v", links[1])
	}
}

func TestHttpExtractListArticles(t *testing.T) {
	html := `<html><body>
		<div class="item">
			<a href="/video/1"><h2>第一个视频</h2></a>
			<img class="thumb" src="/img/1.jpg">
			<p class="desc">第一个视频的描述</p>
		</div>
		<div class="item">
			<a href="/video/2"><h2>第二个视频</h2></a>
			<img class="thumb" data-src="/img/2.jpg">
		</div>
		<div class="item"><a href="/video/1">重复标题条目</a></div>
	</body></html>`
	doc := httpDoc(html)
	base := httpMustURL(t, "https://a.tv/list/1.html")

	tk := &httpTask{
		SourceURL:     "https://a.tv/list/1.html",
		ListSelector:  ".item",
		ListCoverSel:  ".thumb",
		ListCoverAttr: "data-src", // 懒加载：条目 2 走 data-src；条目 1 无 data-src 回退 src
		ListDescSel:   ".desc",
		CategoryID:    7,
		ContentType:   "video",
	}
	articles := tk.extractListArticles(doc, base)
	if len(articles) != 2 {
		t.Fatalf("articles = %d, want 2: %+v", len(articles), articles)
	}
	a := articles[0]
	if a.Title != "第一个视频" || a.CategoryID != 7 || a.ContentType != "video" || !a.Status {
		t.Fatalf("article0 base fields wrong: %+v", a.ArticleBase)
	}
	if a.Slug != hashSlug("https://a.tv/video/1", "第一个视频") {
		t.Fatalf("slug wrong: %q", a.Slug)
	}
	if a.Thumbnail != "https://a.tv/img/1.jpg" {
		t.Fatalf("cover absolute: %q", a.Thumbnail)
	}
	if a.Description != "第一个视频的描述" {
		t.Fatalf("description = %q", a.Description)
	}
	// 条目 2：data-src 命中；无 desc 回退条目文本首行
	b := articles[1]
	if b.Thumbnail != "https://a.tv/img/2.jpg" {
		t.Fatalf("lazy cover: %q", b.Thumbnail)
	}
	if b.Description == "" {
		t.Fatal("fallback description should not be empty")
	}
	if !strings.Contains(b.Description, "第二个视频") {
		t.Fatalf("fallback description should contain item text: %q", b.Description)
	}
}

func TestHttpElementValue(t *testing.T) {
	// attr 空：content 属性优先（meta 场景），回退文本
	meta := httpDoc(`<html><body><meta name="k" content="meta值"></body></html>`).Find("meta")
	if got := httpElementValue(meta, ""); got != "meta值" {
		t.Errorf("meta content first: got %q", got)
	}
	plain := httpDoc(`<html><body><div>文本值</div></body></html>`).Find("div")
	if got := httpElementValue(plain, ""); got != "文本值" {
		t.Errorf("fallback text: got %q", got)
	}

	// attr=html：innerHTML
	rich := httpDoc(`<html><body><div><p>段落</p><b>加粗</b></div></body></html>`).Find("div")
	if got := httpElementValue(rich, "html"); got != "<p>段落</p><b>加粗</b>" {
		t.Errorf("html attr: got %q", got)
	}

	// 指定属性
	lazy := httpDoc(`<html><body><img data-src="/a.jpg"></body></html>`).Find("img")
	if got := httpElementValue(lazy, "data-src"); got != "/a.jpg" {
		t.Errorf("custom attr: got %q", got)
	}
	if got := httpElementValue(lazy, "missing"); got != "" {
		t.Errorf("missing attr: got %q", got)
	}
}

func TestHttpExtractArticle(t *testing.T) {
	link := "https://a.tv/detail/123"
	html := `<html><head>
		<title>页面标题 - 站名</title>
		<meta name="keywords" content="kw1,kw2">
		<meta property="og:image" content="https://cdn.a.tv/og.jpg">
		<meta property="article:published_time" content="2024-05-01 12:30:45">
		</head><body>
		<h1 class="title">文章标题</h1>
		<div class="poster"><img src="/img/cover.jpg"></div>
		<article class="content"><p>正文段落</p></article>
		<span class="vid" data-id="12345"></span>
		</body></html>`
	doc := httpDoc(html)

	tk := &httpTask{
		SourceURL:    "https://a.tv/list",
		ListSelector: ".item",
		TitleSel:     "h1.title",
		CoverSel:     ".poster img",
		ContentSel:   "article.content",
		CategoryID:   3,
		ContentType:  "novel",
		Extra: []spiderExtra{
			{Key: "vid", Selector: ".vid", Attr: "data-id"},
			{Key: "page_num", Selector: "@url", Regex: `/detail/(\d+)`},
		},
	}
	article, err := tk.extractArticle(doc, link, nil)
	if err != nil {
		t.Fatalf("extractArticle: %v", err)
	}

	if article.Title != "文章标题" {
		t.Errorf("title = %q", article.Title)
	}
	if article.Slug != hashSlug(link, "文章标题") {
		t.Errorf("slug = %q", article.Slug)
	}
	if article.CategoryID != 3 || article.ContentType != "novel" || !article.Status {
		t.Errorf("base fields wrong: %+v", article.ArticleBase)
	}
	if article.CreateTime != ts(2024, 5, 1, 12, 30, 45) {
		t.Errorf("create_time = %d, want published_time meta", article.CreateTime)
	}
	if article.Thumbnail != "https://a.tv/img/cover.jpg" {
		t.Errorf("cover = %q, want absolutized cover_sel", article.Thumbnail)
	}
	if article.Keywords != "kw1,kw2" {
		t.Errorf("keywords = %q", article.Keywords)
	}
	if !strings.Contains(article.Content, "正文段落") {
		t.Errorf("content missing: %q", article.Content)
	}
	if article.Description == "" {
		t.Error("description should be derived from content")
	}

	// extends：extra 单值 + @url 提取
	var vid, pageNum string
	for _, it := range article.Extends {
		switch it.Key {
		case "vid":
			vid, _ = it.Value.(string)
		case "page_num":
			pageNum, _ = it.Value.(string)
		}
	}
	if vid != "12345" {
		t.Errorf("extra vid = %v", vid)
	}
	if pageNum != "123" {
		t.Errorf("extra page_num = %v, want from @url", pageNum)
	}
}

func TestHttpExtractArticleFallbacks(t *testing.T) {
	// title_sel/cover_sel/keywords_sel/publish_time_sel 未配置时回退 <title>/og:image/meta keywords/published_time
	link := "https://a.tv/detail/9"
	html := `<html><head>
		<title>回退标题</title>
		<meta name="keywords" content="回退词">
		<meta property="og:image" content="/og/fallback.jpg">
		<meta property="article:published_time" content="2024年6月2日 08:05">
		</head><body>
		<article class="content"><p>回退正文</p></article>
		</body></html>`
	doc := httpDoc(html)

	tk := &httpTask{SourceURL: "https://a.tv/list", ListSelector: ".item", ContentSel: "article.content"}
	article, err := tk.extractArticle(doc, link, nil)
	if err != nil {
		t.Fatalf("extractArticle: %v", err)
	}
	if article.Title != "回退标题" {
		t.Errorf("title fallback = %q", article.Title)
	}
	if article.Thumbnail != "https://a.tv/og/fallback.jpg" {
		t.Errorf("og:image fallback should be absolutized: %q", article.Thumbnail)
	}
	if article.Keywords != "回退词" {
		t.Errorf("keywords fallback = %q", article.Keywords)
	}
	if article.CreateTime != ts(2024, 6, 2, 8, 5, 0) {
		t.Errorf("publish_time fallback = %d", article.CreateTime)
	}

	// 完全无标题 → 报错
	doc2 := httpDoc(`<html><head></head><body><p>无标题页面</p></body></html>`)
	if _, err := tk.extractArticle(doc2, link, nil); err == nil {
		t.Fatal("missing title should error")
	}
}

func TestHttpExtractArticleMedia(t *testing.T) {
	link := "https://a.tv/play/5"
	html := `<html><head><title>剧集页</title></head><body>
		<h1>某剧集</h1>
		<div class="labels"><span>正片</span><span>花絮</span></div>
		<video src="https://cdn.a.tv/v1.mp4"></video>
		<video data-src="https://cdn.a.tv/v2.mp4"></video>
		<iframe src="https://player.tv/e/abc"></iframe>
		<div class="gallery"><img src="/g/1.jpg"><img data-src="/g/2.jpg"></div>
		</body></html>`
	doc := httpDoc(html)

	tk := &httpTask{
		SourceURL:      "https://a.tv/list",
		ListSelector:   ".item",
		TitleSel:       "h1",
		VideoSrcSel:    "video",
		VideoAttr:      "data-src", // 懒加载：video2 走 data-src；video1 无 data-src 回退 src
		VideoIframeSel: "iframe",
		VideoLabelSel:  ".labels span",
		GallerySel:     ".gallery img",
	}
	article, err := tk.extractArticle(doc, link, nil)
	if err != nil {
		t.Fatalf("extractArticle: %v", err)
	}

	var sources []map[string]any
	var gallery []string
	for _, it := range article.Extends {
		switch it.Key {
		case "video_sources":
			sources, _ = it.Value.([]map[string]any)
		case "gallery_images":
			gallery, _ = it.Value.([]string)
		}
	}
	// 2 直链（embed=false）+ 1 iframe（embed=true）；集名按直链在前、iframe 在后全序对应
	if len(sources) != 3 {
		t.Fatalf("video_sources = %d, want 3: %v", len(sources), sources)
	}
	if sources[0]["url"] != "https://cdn.a.tv/v1.mp4" || sources[0]["embed"] != false || sources[0]["label"] != "正片" {
		t.Errorf("source0 wrong: %v", sources[0])
	}
	if sources[1]["url"] != "https://cdn.a.tv/v2.mp4" || sources[1]["label"] != "花絮" {
		t.Errorf("source1 wrong: %v", sources[1])
	}
	if sources[2]["url"] != "https://player.tv/e/abc" || sources[2]["embed"] != true || sources[2]["label"] != "第03集" {
		t.Errorf("source2 wrong: %v", sources[2])
	}
	if len(gallery) != 2 || gallery[0] != "https://a.tv/g/1.jpg" || gallery[1] != "https://a.tv/g/2.jpg" {
		t.Fatalf("gallery wrong: %v", gallery)
	}
}

func TestHttpExtractArticleContentPaging(t *testing.T) {
	// 正文提取自当前文档；翻页由调用方逐页 fetch 后拼接（解析层只验证单页提取）
	html := `<html><head><title>标题</title></head><body><div class="content">第一页内容</div></body></html>`
	doc := httpDoc(html)
	tk := &httpTask{SourceURL: "https://a.tv/list", ListSelector: ".item", ContentSel: ".content"}
	article, err := tk.extractArticle(doc, "https://a.tv/detail/1", nil)
	if err != nil {
		t.Fatalf("extractArticle: %v", err)
	}
	if !strings.Contains(article.Content, "第一页内容") {
		t.Errorf("content = %q", article.Content)
	}
}

func TestHttpDetectBlockSigns(t *testing.T) {
	// 正常页面返回空串
	if got := detectBlockSigns("<html><body><h1>正常列表</h1><a href='/1'>x</a></body></html>"); got != "" {
		t.Errorf("normal page should not be flagged: %q", got)
	}
	// 常见反爬特征
	for _, body := range []string{
		"请完成验证码后继续访问",
		"Just a moment...",
		"Enable JavaScript and cookies to continue",
		"访问过于频繁，请稍后再试",
		"Checking your browser before accessing",
	} {
		if got := detectBlockSigns(body); got == "" {
			t.Errorf("block sign not detected: %q", body)
		}
	}
}

func TestGroupHTTPTasksByDomain(t *testing.T) {
	enabled := []*httpTask{
		{Name: "a1", SourceURL: "https://a.tv/list"},
		{Name: "b1", SourceURL: "https://www.b.tv/list"},
		{Name: "a2", SourceURL: "http://www.a.tv/list2"}, // www/http 与裸域 https 同域名归并
		{Name: "c1", SourceURL: "http://127.0.0.1:9000/list"},
	}
	groups := groupHTTPTasksByDomain(enabled)

	want := []struct {
		domain  string
		names   []string
		indexes []int
	}{
		{domain: "a.tv", names: []string{"a1", "a2"}, indexes: []int{0, 2}},
		{domain: "b.tv", names: []string{"b1"}, indexes: []int{1}},
		{domain: "127.0.0.1:9000", names: []string{"c1"}, indexes: []int{3}},
	}
	if len(groups) != len(want) {
		t.Fatalf("group count = %d, want %d: %+v", len(groups), len(want), groups)
	}
	for gi, g := range groups {
		if g.Domain != want[gi].domain {
			t.Errorf("group[%d] domain = %q, want %q", gi, g.Domain, want[gi].domain)
		}
		if len(g.Tasks) != len(want[gi].names) {
			t.Fatalf("group[%d] %s task count = %d, want %d", gi, g.Domain, len(g.Tasks), len(want[gi].names))
		}
		for ti, s := range g.Tasks {
			if s.task.Name != want[gi].names[ti] {
				t.Errorf("group[%d] task[%d] name = %q, want %q", gi, ti, s.task.Name, want[gi].names[ti])
			}
			if s.index != want[gi].indexes[ti] {
				t.Errorf("group[%d] task[%d] %s index = %d, want %d", gi, ti, s.task.Name, s.index, want[gi].indexes[ti])
			}
		}
	}
}
