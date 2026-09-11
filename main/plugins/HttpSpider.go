package plugins

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"

	"moss/domain/core/entity"
	"moss/domain/core/service"
	"moss/domain/core/vo"
	pluginEntity "moss/domain/support/entity"
	"moss/infrastructure/utils/request"
)

// HttpSpider 通用 HTTP 采集插件（无浏览器）
// 面向无需 JS 渲染即可拿到完整 HTML 的站点（服务端渲染/静态站）：
// 纯 HTTP 请求 + goquery 解析，任务配置模型与 HeadlessSpider 对齐（任务 JSON 可平移），
// 靠任务内详情页并发抓取提速。浏览器专属能力（渲染等待/模拟点击/滚动加载）不适用本插件，
// 对应场景请改用 HeadlessSpider。
type HttpSpider struct {
	Proxy       string `json:"proxy"`       // HTTP 代理（如 http://127.0.0.1:7890），留空不用
	Timeout     int    `json:"timeout"`     // 单请求超时秒数（默认 30）
	Concurrency int    `json:"concurrency"` // 任务内详情页并发抓取数（默认 4，提速核心；目标站有限频时调小）
	Interval    int    `json:"interval"`    // worker 每篇入库后间隔秒数（默认 0；HTTP 场景无需浏览器那么保守）
	Tasks       string `json:"tasks"`       // 任务数组 JSON（见 About），多站多分类在此配置

	Retry    int    `json:"retry"`     // 详情页采集失败重试次数（默认 0，间隔逐次递增 1s/2s/...）
	Limit    int    `json:"limit"`     // 每任务最多入库篇数（0=不限；配合 dry_run 做小样调试）
	DryRun   bool   `json:"dry_run"`   // 试运行：只解析与打日志，不入库（调试选择器用）
	DebugDir string `json:"debug_dir"` // 调试目录：列表页/详情页失败时保存响应 HTML（留空关闭）

	ctx *pluginEntity.Plugin
}

// httpTask 单个采集任务（字段语义与 spiderTask 对齐，便于配置平移）
type httpTask struct {
	Name           string `json:"name"`             // 任务名（日志区分）
	Enable         bool   `json:"enable"`           // 是否启用
	SourceURL      string `json:"source_url"`       // 起始页 URL
	PageURLPattern string `json:"page_url_pattern"` // 翻页模板，含 {page}；留空只采起始页
	MaxPages       int    `json:"max_pages"`        // 最多翻页数（默认 1；next_page_sel 模式默认 50）

	Mode         string `json:"mode"`          // detail=进详情页提取（默认）；list=直接从列表条目出文章（配 list_cover_sel/list_desc_sel）
	ListSelector string `json:"list_selector"` // 列表页条目选择器
	LinkInclude  string `json:"link_include"`  // 链接过滤：子串或 /正则/；留空不过滤
	LinkExclude  string `json:"link_exclude"`  // 链接排除：子串或 /正则/
	// 「下一页」链接选择器：静态页翻页方式（page_url_pattern 之外的第二种）。
	// 仅支持 a[href] —— 找到元素后读 href GET 翻页；JS 按钮翻页（无 href）请用 HeadlessSpider
	NextPageSel string `json:"next_page_sel"`

	ListTitleSel          string `json:"list_title_sel"`           // 列表条目标题选择器（预查重/列表出文用；回退 h1~h6/首行文本）
	ListCoverSel          string `json:"list_cover_sel"`           // mode=list 条目封面选择器
	ListCoverAttr         string `json:"list_cover_attr"`          // mode=list 封面读哪个属性（默认 src；懒加载可填 data-src，取不到自动回退 src）
	ListCoverExtractRegex string `json:"list_cover_extract_regex"` // mode=list 封面正则抽取
	ListDescSel           string `json:"list_desc_sel"`            // mode=list 条目摘要选择器（回退条目文本）

	// 属性提取通道：link_selector 定位条目内链接元素（空=条目自身）；link_attr 读哪个属性
	// （默认 href，可 data-url/data-id 等）；link_extract_regex 从属性值抽取 URL（可 /re/ 包裹）；
	// link_url_template 拼接最终 URL（含 {value} 则替换，否则视为前缀拼接），适配 data-id 拼详情地址
	LinkSelector     string `json:"link_selector"`
	LinkAttr         string `json:"link_attr"`
	LinkExtractRegex string `json:"link_extract_regex"`
	LinkURLTemplate  string `json:"link_url_template"`

	StopWhenExists int `json:"stop_when_exists"` // 连续 N 篇已存在则提前结束任务（0=不启用）；定时增量采集推荐 3~5

	// 字段统一提取模型：*_sel 定位元素 → *_attr 读属性（空=自动：meta 的 content 优先、回退文本；
	// 封面是 img 的 src 优先）→ *_extract_regex 从取到的值里正则抽取（可 /re/ 包裹，取第一捕获组）。
	// 语义与 HeadlessSpider 一致
	TitleSel                string `json:"title_sel"`                  // 详情页标题选择器（回退 <title>）
	TitleAttr               string `json:"title_attr"`                 // 标题读哪个属性（空=自动）
	TitleExtractRegex       string `json:"title_extract_regex"`        // 标题正则抽取（去“ - 站名”后缀等）
	CoverSel                string `json:"cover_sel"`                  // 封面选择器（未取到回退 og:image meta）
	CoverAttr               string `json:"cover_attr"`                 // 封面读哪个属性（空=自动 src→content）
	CoverExtractRegex       string `json:"cover_extract_regex"`        // 封面正则抽取
	ContentSel              string `json:"content_sel"`                // 正文容器选择器（取 innerHTML）
	ContentExtractRegex     string `json:"content_extract_regex"`      // 正文容器 HTML 的正则抽取
	KeywordsSel             string `json:"keywords_sel"`               // 关键词选择器（回退 meta[name=keywords]）
	KeywordsAttr            string `json:"keywords_attr"`              // 关键词读哪个属性（空=自动）
	KeywordsExtractRegex    string `json:"keywords_extract_regex"`     // 关键词正则抽取
	PublishTimeSel          string `json:"publish_time_sel"`           // 发布时间选择器（回退 meta[property=article:published_time]）
	PublishTimeAttr         string `json:"publish_time_attr"`          // 发布时间读哪个属性（空=自动）
	PublishTimeExtractRegex string `json:"publish_time_extract_regex"` // 发布时间正则抽取
	// 正文翻页：详情页正文分页时，content_next_sel 指向「下一页」链接（仅支持 a[href]），
	// 逐页 GET 拼接正文（内容无新增即停；content_max_pages 安全上限，默认 20）
	ContentNextSel  string `json:"content_next_sel"`
	ContentMaxPages int    `json:"content_max_pages"`

	// 播放源：video_src_sel=直链（video 标签，embed=false）；video_iframe_sel=第三方播放页（iframe，embed=true）。
	// *_attr 支持懒加载 data-src（取不到自动回退 src）；*_extract_regex 从属性值正则抽地址；
	// video_label_sel 给出与播放源同数量、同顺序（直链在前、iframe 在后）的集名元素（缺省回退“第NN集”）
	VideoSrcSel             string `json:"video_src_sel"`
	VideoAttr               string `json:"video_attr"`
	VideoExtractRegex       string `json:"video_extract_regex"`
	VideoIframeSel          string `json:"video_iframe_sel"`
	VideoIframeAttr         string `json:"video_iframe_attr"`
	VideoIframeExtractRegex string `json:"video_iframe_extract_regex"`
	VideoLabelSel           string `json:"video_label_sel"`
	VideoLabelAttr          string `json:"video_label_attr"`

	GallerySel          string `json:"gallery_sel"`           // 图集图片选择器 → extends[gallery_images]
	GalleryAttr         string `json:"gallery_attr"`          // 图集读哪个属性（默认 src；懒加载可填 data-src，取不到自动回退 src）
	GalleryExtractRegex string `json:"gallery_extract_regex"` // 从属性值正则抽图址

	Extra []spiderExtra `json:"extra"` // 通用 extends 键值提取（任意 key：作者/版本/自定义...）

	ContentType string `json:"content_type"` // 入库类型：standard/novel/image/video
	CategoryID  int    `json:"category_id"`  // 入库分类 ID

	// 登录态/指纹：每任务请求头。cookies 为原始 Cookie 头字符串（k=v; k2=v2），比浏览器插件的
	// JSON 数组更贴合 HTTP 场景；headers 为 JSON 对象（如 {"Referer":"https://a.tv/"}）
	UserAgent string `json:"user_agent"` // 请求 UA（留空用内置默认 Chrome UA）
	Headers   string `json:"headers"`    // 自定义请求头 JSON 对象
	Cookies   string `json:"cookies"`    // 原始 Cookie 头字符串
	Encoding  string `json:"encoding"`   // 强制响应解码编码（如 gbk/gb18030；留空自动检测：header 声明 → meta 声明 → 非 UTF-8 字节回退 GBK）

	// 去重键：默认/ url_title = 源URL+标题 哈希；"url" = 仅按源 URL（站点标题微调不会重复入库；
	// 注意切换 dedup_by 后既有文章会按新键被重新采集）
	DedupBy string `json:"dedup_by"`
}

func NewHttpSpider() *HttpSpider {
	return &HttpSpider{
		Timeout:     30,
		Concurrency: 4,
		Tasks:       "[]",
	}
}

func (h *HttpSpider) Info() *pluginEntity.PluginInfo {
	return &pluginEntity.PluginInfo{
		ID: "HttpSpider",
		About: "通用 HTTP 采集插件（无浏览器，多任务）：面向无需 JS 渲染即可拿到完整 HTML 的站点，" +
			"纯 HTTP 请求 + goquery 解析，任务内详情页并发抓取（concurrency 默认 4）远快于无头浏览器。" +
			"tasks 配置 JSON 任务数组，配置模型与 HeadlessSpider 对齐（任务 JSON 可平移）：mode=detail/list、" +
			"翻页 page_url_pattern（URL 模板）/ next_page_sel（下一页 a[href]）；取链属性通道 link_selector+link_attr+" +
			"link_extract_regex+link_url_template（适配 data-id 拼详情链接）；字段统一提取模型 title/cover/content/" +
			"keywords/publish_time/video/video_iframe/gallery/extra 均支持 *_attr 指定属性（空=自动：meta 的 content 优先回退文本）" +
			"与 *_extract_regex 正则抽取；extra 的 selector=@url 时从详情页 URL 提取；" +
			"正文翻页 content_next_sel+content_max_pages；视频源/图集写入 extends 对接 wolves 消费页。" +
			"每任务独立 user_agent/headers（JSON 对象）/cookies（原始 Cookie 头字符串）/encoding" +
			"（强制解码如 gbk，留空自动检测 header/meta 声明，无声明且非 UTF-8 时回退 GBK）。" +
			"全局 proxy/timeout/retry/limit/dry_run/debug_dir（失败存响应 HTML）/dedup_by，" +
			"stop_when_exists 连续已存在早停（触发即终止整个任务翻页，不再请求后续列表页）；" +
			"interval 为单 worker 每篇入库后的等待秒数，并发下不构成全局节流，严格限频请调低 concurrency。" +
			"与 HeadlessSpider 的分工：目标站查看源代码（Ctrl+U）能看到完整内容的用本插件；" +
			"源代码里没有、需浏览器渲染/模拟点击/滚动加载的用 HeadlessSpider",
		RunEnable:  true,
		CronEnable: true,
		PluginInfoPersistent: pluginEntity.PluginInfoPersistent{
			CronStart: false,
			CronExp:   "@every 1h",
		},
	}
}

func (h *HttpSpider) Load(ctx *pluginEntity.Plugin) error {
	h.ctx = ctx
	// 数据库旧记录的 cron_exp 为空时补默认值，避免装载失败
	if ctx.Info.CronEnable && ctx.Info.CronExp == "" {
		ctx.Info.CronExp = "@every 1h"
	}
	return nil
}

func (h *HttpSpider) Run(ctx *pluginEntity.Plugin) error {
	h.ctx = ctx
	if h.Timeout <= 0 {
		h.Timeout = 30
	}
	if h.Concurrency <= 0 {
		h.Concurrency = 4
	}
	if h.Retry < 0 {
		h.Retry = 0
	}
	tasks, err := h.parseTasks()
	if err != nil {
		return err
	}
	var enabled []*httpTask
	for i := range tasks {
		if tasks[i].Enable {
			enabled = append(enabled, &tasks[i])
		}
	}
	if len(enabled) == 0 {
		return fmt.Errorf("tasks 中没有启用（enable=true）的任务")
	}

	h.ctx.Log.Info("开始 HTTP 采集", zap.Int("tasks", len(enabled)),
		zap.Int("concurrency", h.Concurrency), zap.Bool("dry_run", h.DryRun), zap.Int("limit", h.Limit))
	startTime := time.Now()

	// 任务间并行（HTTP 无浏览器实例上限）；每任务返回独立汇总
	results := make([]taskResult, len(enabled))
	var wg sync.WaitGroup
	for i, t := range enabled {
		wg.Add(1)
		go func(i int, t *httpTask) {
			defer wg.Done()
			results[i] = h.runTask(t)
		}(i, t)
	}
	wg.Wait()

	var collected, skipped, failed int
	for _, r := range results {
		collected += r.Collected
		skipped += r.Skipped
		failed += r.Failed
	}
	h.ctx.Log.Info("全部任务结束", zap.Duration("cost", time.Since(startTime)),
		zap.Int("collected", collected), zap.Int("skipped", skipped), zap.Int("failed", failed))
	return nil
}

// taskResult 单任务汇总
type taskResult struct {
	Collected, Skipped, Failed int
}

// httpFetcher 任务级 HTTP 抓取器：共享连接池（Transport 复用），任务级请求头
type httpFetcher struct {
	client  *http.Client
	headers map[string]string
	cookies string
}

// defaultUserAgent 内置默认 Chrome UA（取 utils/request 默认值缓存于包级变量，避免每次建 fetcher 重复分配）
var defaultUserAgent = request.New().Header["User-Agent"]

// newHTTPFetcher 按插件全局 + 任务配置构建抓取器。
// 自建 http.Client 而不复用 utils/request：并发 worker 共用 request.Request 会竞争其 retryCount，
// 且共享的 Transport 连接池对高频抓取至关重要
func newHTTPFetcher(h *HttpSpider, t *httpTask) *httpFetcher {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 采集站证书常不规范，与 utils/request 行为一致
		Proxy:           http.ProxyFromEnvironment,
	}
	if h.Proxy != "" {
		if u, err := url.Parse(h.Proxy); err == nil {
			transport.Proxy = http.ProxyURL(u)
		} else {
			h.ctx.Log.Warn("代理地址无效，忽略", zap.String("proxy", h.Proxy), zap.Error(err))
		}
	}
	timeout := time.Duration(h.Timeout) * time.Second
	f := &httpFetcher{
		client:  &http.Client{Transport: transport, Timeout: timeout},
		headers: map[string]string{},
		cookies: t.Cookies,
	}
	ua := t.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	f.headers["User-Agent"] = ua
	f.headers["Accept"] = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
	f.headers["Accept-Language"] = "zh-CN,zh;q=0.9,en;q=0.8"
	if t.Referer() != "" {
		f.headers["Referer"] = t.Referer()
	}
	if custom, err := parseHeaders(t.Headers); err != nil {
		h.ctx.Log.Warn("headers 配置无效（parseTasks 已校验，不应出现）", zap.String("task", t.Name), zap.Error(err))
	} else {
		for k, v := range custom {
			f.headers[k] = v
		}
	}
	return f
}

// Referer 任务默认 Referer：起始页（列表页与详情页请求共用，最常见的防盗链判据）
func (t *httpTask) Referer() string { return t.SourceURL }

// maxBodySize 单响应体积上限（10MB）：防异常站点返回超大响应拖垮内存
const maxBodySize = 10 << 20

// get GET 页面返回原始字节、Content-Type 与最终 URL（重定向后）。非 2xx/3xx 视为失败（错误含状态码）
func (f *httpFetcher) get(pageURL string) (body []byte, contentType string, finalURL *url.URL, err error) {
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, "", nil, fmt.Errorf("构造请求失败: %w", err)
	}
	for k, v := range f.headers {
		req.Header.Set(k, v)
	}
	if f.cookies != "" {
		req.Header.Set("Cookie", f.cookies)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, "", nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	contentType = resp.Header.Get("Content-Type")
	body, err = io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	return body, contentType, resp.Request.URL, err
}

// fetchDoc GET 并解码为 goquery 文档。返回原始字节供失败时存调试现场；
// doc.Url 记录重定向后的最终地址（goquery 从字节构造时该字段为空，这里补上供相对地址解析）
func (h *HttpSpider) fetchDoc(f *httpFetcher, t *httpTask, pageURL string) (doc *goquery.Document, body []byte, err error) {
	body, ctype, finalURL, err := f.get(pageURL)
	if err != nil {
		return nil, body, err
	}
	if len(body) >= maxBodySize {
		h.ctx.Log.Warn("响应体达到 10MB 上限可能被截断，解析结果或残缺",
			zap.String("task", t.Name), zap.String("url", pageURL))
	}
	html, err := decodeHTMLBody(body, ctype, t.Encoding)
	if err != nil {
		return nil, body, fmt.Errorf("响应解码失败: %w", err)
	}
	doc, perr := goquery.NewDocumentFromReader(strings.NewReader(html))
	if perr != nil {
		return nil, body, fmt.Errorf("HTML 解析失败: %w", perr)
	}
	doc.Url = finalURL
	return doc, body, nil
}

// parseTasks 解析并校验任务数组：TrimSpace（后台手填易带空格）、默认值、必填与格式校验。
// 校验失败的配置在解析期报错，而不是采集期静默取不到值
func (h *HttpSpider) parseTasks() ([]httpTask, error) {
	raw := strings.TrimSpace(h.Tasks)
	if raw == "" {
		return nil, fmt.Errorf("tasks 未配置")
	}
	var tasks []httpTask
	if err := json.Unmarshal([]byte(raw), &tasks); err != nil {
		return nil, fmt.Errorf("tasks JSON 解析失败: %w", err)
	}
	for i := range tasks {
		if err := normalizeHTTPTask(&tasks[i], fmt.Sprintf("任务[%d]", i+1)); err != nil {
			return nil, err
		}
	}
	return tasks, nil
}

// normalizeHTTPTask 单任务清洗与校验（parseTasks 与 ValidateTask 共用）
func normalizeHTTPTask(t *httpTask, label string) error {
	for _, p := range []*string{
		&t.Name, &t.SourceURL, &t.PageURLPattern, &t.ListSelector, &t.LinkInclude, &t.LinkExclude,
		&t.NextPageSel, &t.ListTitleSel, &t.ListCoverSel, &t.ListCoverAttr, &t.ListCoverExtractRegex, &t.ListDescSel,
		&t.LinkSelector, &t.LinkAttr, &t.LinkExtractRegex, &t.LinkURLTemplate,
		&t.TitleSel, &t.TitleAttr, &t.TitleExtractRegex,
		&t.CoverSel, &t.CoverAttr, &t.CoverExtractRegex,
		&t.ContentSel, &t.ContentExtractRegex, &t.ContentNextSel,
		&t.KeywordsSel, &t.KeywordsAttr, &t.KeywordsExtractRegex,
		&t.PublishTimeSel, &t.PublishTimeAttr, &t.PublishTimeExtractRegex,
		&t.VideoSrcSel, &t.VideoAttr, &t.VideoExtractRegex,
		&t.VideoIframeSel, &t.VideoIframeAttr, &t.VideoIframeExtractRegex,
		&t.VideoLabelSel, &t.VideoLabelAttr, &t.GallerySel, &t.GalleryAttr, &t.GalleryExtractRegex,
		&t.ContentType, &t.UserAgent, &t.Headers, &t.Cookies, &t.Encoding, &t.DedupBy,
	} {
		*p = strings.TrimSpace(*p)
	}
	for j := range t.Extra {
		t.Extra[j].Key = strings.TrimSpace(t.Extra[j].Key)
		t.Extra[j].Selector = strings.TrimSpace(t.Extra[j].Selector)
		t.Extra[j].Regex = strings.TrimSpace(t.Extra[j].Regex)
	}
	if t.MaxPages <= 0 {
		if t.NextPageSel != "" {
			t.MaxPages = 50 // 按钮翻页模式不填则给安全上限
		} else {
			t.MaxPages = 1
		}
	}
	if t.Mode == "" {
		t.Mode = "detail"
	}
	if t.Mode != "detail" && t.Mode != "list" {
		return fmt.Errorf("%s%s mode 无效（仅支持 detail/list）: %q", label, t.Name, t.Mode)
	}
	if t.SourceURL == "" || t.ListSelector == "" {
		return fmt.Errorf("%s%s 缺少 source_url 或 list_selector", label, t.Name)
	}
	if t.DedupBy != "" && t.DedupBy != "url" && t.DedupBy != "url_title" {
		return fmt.Errorf("%s%s dedup_by 无效（仅支持 url/url_title）: %q", label, t.Name, t.DedupBy)
	}
	for field, pattern := range map[string]string{
		"link_extract_regex":         t.LinkExtractRegex,
		"title_extract_regex":        t.TitleExtractRegex,
		"cover_extract_regex":        t.CoverExtractRegex,
		"list_cover_extract_regex":   t.ListCoverExtractRegex,
		"content_extract_regex":      t.ContentExtractRegex,
		"keywords_extract_regex":     t.KeywordsExtractRegex,
		"publish_time_extract_regex": t.PublishTimeExtractRegex,
		"video_extract_regex":        t.VideoExtractRegex,
		"video_iframe_extract_regex": t.VideoIframeExtractRegex,
		"gallery_extract_regex":      t.GalleryExtractRegex,
	} {
		if _, err := compileExtractRegex(pattern); err != nil {
			return fmt.Errorf("%s%s %s 无效: %w", label, t.Name, field, err)
		}
	}
	for j := range t.Extra {
		if _, err := compileExtractRegex(t.Extra[j].Regex); err != nil {
			return fmt.Errorf("%s%s extra[%d]%s regex 无效: %w", label, t.Name, j, t.Extra[j].Key, err)
		}
	}
	// link_include/link_exclude 按 matchOne 约定：/re/ 包裹为正则（解析期校验语法），其余按子串匹配不校验
	for field, pattern := range map[string]string{
		"link_include": t.LinkInclude,
		"link_exclude": t.LinkExclude,
	} {
		if len(pattern) > 2 && strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
			if _, err := regexp.Compile(pattern[1 : len(pattern)-1]); err != nil {
				return fmt.Errorf("%s%s %s 正则无效: %w", label, t.Name, field, err)
			}
		}
	}
	if _, err := parseHeaders(t.Headers); err != nil {
		return fmt.Errorf("%s%s %w", label, t.Name, err)
	}
	if t.Encoding != "" {
		if _, name := charset.Lookup(t.Encoding); name == "" {
			return fmt.Errorf("%s%s encoding 无效: %q", label, t.Name, t.Encoding)
		}
	}
	return nil
}

// pageURL 翻页地址：第 1 页/无模板返回起始页，否则按模板替换 {page}
func (t *httpTask) pageURL(page int) string {
	if page <= 1 || t.PageURLPattern == "" {
		return t.SourceURL
	}
	return strings.ReplaceAll(t.PageURLPattern, "{page}", strconv.Itoa(page))
}

// parseHeaders 解析自定义请求头配置（JSON 对象）。空配置返回 nil
func parseHeaders(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("headers JSON 解析失败（应为对象）: %w", err)
	}
	return m, nil
}

// decodeHTMLBody 响应字节 → UTF-8 文本。优先级：force 强制指定 > Content-Type header 声明 >
// HTML meta 声明（DetermineEncoding 内置 prescan）> 无声明且非合法 UTF-8 时回退 GBK（中文站最常见）
func decodeHTMLBody(body []byte, contentType, force string) (string, error) {
	if force = strings.TrimSpace(force); force != "" {
		enc, name := charset.Lookup(force)
		if name == "" {
			return "", fmt.Errorf("未知编码 %q", force)
		}
		return decodeWith(body, enc)
	}
	enc, _, certain := charset.DetermineEncoding(body, contentType)
	if !certain && !utf8.Valid(body) {
		// 无有效声明且不是合法 UTF-8：老站常见无声明 GBK；按 GBK 解（GBK 双字节覆盖面之外的
		// 生僻字场景可显式配 encoding=gb18030）
		enc, _ = charset.Lookup("gbk")
	}
	return decodeWith(body, enc)
}

// decodeWith 按指定编码解码为 UTF-8
func decodeWith(body []byte, enc encoding.Encoding) (string, error) {
	if enc == nil || enc == encoding.Nop {
		return string(body), nil
	}
	reader := transform.NewReader(bytes.NewReader(body), enc.NewDecoder())
	out, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// blockSigns 反爬拦截特征（参考 tgTool block_detector 的简化版）：命中即打告警日志。
// 只提示不中断——特征词出现在正常页面的可能存在，由解析结果自然兜底
var blockSigns = []string{
	"验证码", "Just a moment", "Enable JavaScript and cookies", "访问过于频繁",
	"Checking your browser", "cf-browser-verification", "请开启 JavaScript", "拒绝访问",
}

// detectBlockSigns 返回响应中命中的第一个反爬特征词，无命中返回空串
func detectBlockSigns(body string) string {
	for _, sign := range blockSigns {
		if strings.Contains(body, sign) {
			return sign
		}
	}
	return ""
}

// ---- 解析层（goquery，可测）----

// httpLink 列表页提取的详情链接及其标题（标题用于预查重，已存在则无需请求详情页）
type httpLink struct {
	URL   string
	Title string
}

// extractLinks 遍历 list_selector 条目，经属性通道（link_selector/link_attr/regex/template）
// 解析详情 URL：相对路径绝对化、include/exclude 过滤、去重；标题取条目（回退 h1~h6/首行文本）
func (t *httpTask) extractLinks(doc *goquery.Document, base *url.URL) []httpLink {
	var links []httpLink
	seen := map[string]struct{}{}
	doc.Find(t.ListSelector).Each(func(_ int, item *goquery.Selection) {
		raw := t.linkValueFromItem(item)
		if raw == "" || strings.HasPrefix(raw, "javascript:") {
			return
		}
		ref, err := url.Parse(raw)
		if err != nil {
			return
		}
		abs := raw
		if base != nil {
			abs = base.ResolveReference(ref).String()
		}
		abs = strings.SplitN(abs, "#", 2)[0]
		if abs == "" || (base != nil && abs == base.String()) {
			return
		}
		if !matchFilter(abs, t.LinkInclude, t.LinkExclude) {
			return
		}
		if _, dup := seen[abs]; dup {
			return
		}
		seen[abs] = struct{}{}
		links = append(links, httpLink{URL: abs, Title: httpItemTitle(item, t.ListTitleSel)})
	})
	return links
}

// linkValueFromItem 条目取链：link_selector 显式定位，否则条目自身优先，
// 自身取不到值回退条目内第一个 a[href]/[data-url]（容器型条目站点占多数）
func (t *httpTask) linkValueFromItem(item *goquery.Selection) string {
	if t.LinkSelector != "" {
		if sub := item.Find(t.LinkSelector).First(); sub.Length() > 0 {
			return t.linkValue(sub)
		}
		return ""
	}
	if v := t.linkValue(item); v != "" {
		return v
	}
	if inner := item.Find("a[href], [data-url]").First(); inner.Length() > 0 {
		return t.linkValue(inner)
	}
	return ""
}

// linkValue 从链接元素解析详情 URL：读 link_attr（默认 href）属性，可选 regex 抽取、模板拼接
func (t *httpTask) linkValue(s *goquery.Selection) string {
	attr := t.LinkAttr
	if attr == "" {
		attr = "href"
	}
	v := strings.TrimSpace(s.AttrOr(attr, ""))
	if v == "" {
		return ""
	}
	return buildLinkURL(t.LinkURLTemplate, applyExtractRegex(t.LinkExtractRegex, v))
}

// httpItemTitle 条目标题：list_title_sel 优先，回退 h1~h6，再回退条目文本首行
func httpItemTitle(item *goquery.Selection, titleSel string) string {
	if titleSel != "" {
		if s := strings.TrimSpace(item.Find(titleSel).First().Text()); s != "" {
			return s
		}
	}
	if s := strings.TrimSpace(item.Find("h1,h2,h3,h4,h5,h6").First().Text()); s != "" {
		return s
	}
	txt := strings.TrimSpace(item.Text())
	if i := strings.IndexAny(txt, "\n\r"); i > 0 {
		txt = txt[:i]
	}
	return strings.TrimSpace(txt)
}

// httpElementValue 按规则取元素值：attr 空=自动（meta 类元素的 content 属性优先，回退文本），
// "html"=innerHTML，否则取指定属性（取不到自动回退 src↔data-src 互备，适配混合懒加载站）
func httpElementValue(s *goquery.Selection, attr string) string {
	switch {
	case attr == "":
		if v := strings.TrimSpace(s.AttrOr("content", "")); v != "" {
			return v
		}
		return strings.TrimSpace(s.Text())
	case attr == "html":
		html, _ := s.Html()
		return html
	default:
		return httpAttrWithFallback(s, attr)
	}
}

// httpAttrWithFallback 读属性值；为空时在 src/data-src 间自动互补
// （站点懒加载改造常一半 img 有 data-src 一半没有，显式配 data-src 时另一半不丢）
func httpAttrWithFallback(s *goquery.Selection, attr string) string {
	if v := strings.TrimSpace(s.AttrOr(attr, "")); v != "" {
		return v
	}
	switch attr {
	case "src":
		return strings.TrimSpace(s.AttrOr("data-src", ""))
	case "data-src":
		return strings.TrimSpace(s.AttrOr("src", ""))
	}
	return ""
}

// listCoverAttr 封面读哪个属性（默认 src）
func (t *httpTask) listCoverAttr() string {
	if t.ListCoverAttr != "" {
		return t.ListCoverAttr
	}
	return "src"
}

// extractListArticles mode=list：遍历列表条目直接构造文章（不进详情页）。
// 链接走属性通道（与 extractLinks 同规则）；标题 list_title_sel，封面 list_cover_sel，摘要 list_desc_sel
func (t *httpTask) extractListArticles(doc *goquery.Document, base *url.URL) []*entity.Article {
	var articles []*entity.Article
	seen := map[string]struct{}{}
	now := time.Now().Unix()
	doc.Find(t.ListSelector).Each(func(_ int, item *goquery.Selection) {
		raw := t.linkValueFromItem(item)
		if raw == "" || strings.HasPrefix(raw, "javascript:") {
			return
		}
		ref, err := url.Parse(raw)
		if err != nil {
			return
		}
		abs := raw
		if base != nil {
			abs = base.ResolveReference(ref).String()
		}
		abs = strings.SplitN(abs, "#", 2)[0]
		if abs == "" || (base != nil && abs == base.String()) {
			return
		}
		if !matchFilter(abs, t.LinkInclude, t.LinkExclude) {
			return
		}
		if _, dup := seen[abs]; dup {
			return
		}
		seen[abs] = struct{}{}
		title := httpItemTitle(item, t.ListTitleSel)
		if title == "" {
			return // 无标题无法成文
		}
		art := &entity.Article{
			ArticleBase: entity.ArticleBase{
				Slug:        t.dedupSlug(abs, title),
				Title:       title,
				CategoryID:  t.CategoryID,
				Status:      true,
				CreateTime:  now,
				ContentType: t.ContentType,
			},
		}
		if t.ListCoverSel != "" {
			if n := item.Find(t.ListCoverSel).First(); n.Length() > 0 {
				v := applyExtractRegex(t.ListCoverExtractRegex, httpElementValue(n, t.listCoverAttr()))
				if v != "" && base != nil {
					if ref2, perr := url.Parse(v); perr == nil {
						art.Thumbnail = base.ResolveReference(ref2).String()
					}
				}
			}
		}
		desc := ""
		if t.ListDescSel != "" {
			desc = strings.TrimSpace(item.Find(t.ListDescSel).First().Text())
		}
		if desc == "" {
			desc = item.Text()
		}
		art.Description = plainSummary(desc, 120)
		articles = append(articles, art)
	})
	return articles
}

// dedupSlug 去重键：默认（含 url_title）= 源URL+标题 哈希；"url" = 仅源 URL 哈希。
// 与 spiderTask.dedupSlug 逻辑一致（复用 hashSlug，保持跨插件去重键兼容）
func (t *httpTask) dedupSlug(link, title string) string {
	if t.DedupBy == "url" {
		return hashSlug(link, "")
	}
	return hashSlug(link, title)
}

// attrList 提取一组元素的指定值（多值聚合）；regex 非空时对每个值做正则抽取
func (t *httpTask) attrList(doc *goquery.Document, selector, attr, regex string) []string {
	var vals []string
	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		if v := applyExtractRegex(regex, httpElementValue(s, attr)); v != "" {
			vals = append(vals, v)
		}
	})
	return vals
}

// videoAttr 直链播放源读哪个属性（默认 src）
func (t *httpTask) videoAttr() string {
	if t.VideoAttr != "" {
		return t.VideoAttr
	}
	return "src"
}

// videoIframeAttr iframe 播放源读哪个属性（默认 src）
func (t *httpTask) videoIframeAttr() string {
	if t.VideoIframeAttr != "" {
		return t.VideoIframeAttr
	}
	return "src"
}

// galleryAttr 图集读哪个属性（默认 src）
func (t *httpTask) galleryAttr() string {
	if t.GalleryAttr != "" {
		return t.GalleryAttr
	}
	return "src"
}

// contentOf 当前文档提取正文（content_sel + content_extract_regex）
func (t *httpTask) contentOf(doc *goquery.Document) string {
	if t.ContentSel == "" {
		return ""
	}
	s := doc.Find(t.ContentSel).First()
	if s.Length() == 0 {
		return ""
	}
	html, _ := s.Html()
	return applyExtractRegex(t.ContentExtractRegex, html)
}

// findNextURL 从当前文档解析「下一页」链接地址（next_page_sel/content_next_sel 共用）：
// 读元素 href 并以 base 绝对化；无元素/无有效 href（JS 按钮翻页）返回空串。
// 纯 HTTP 无法执行 JS 翻页，此时应停止翻页（提示改用 HeadlessSpider）
func findNextURL(doc *goquery.Document, sel string, base *url.URL) string {
	if sel == "" || doc == nil {
		return ""
	}
	s := doc.Find(sel).First()
	if s.Length() == 0 {
		return ""
	}
	href := strings.TrimSpace(s.AttrOr("href", ""))
	if href == "" || strings.HasPrefix(href, "javascript") {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if base != nil {
		return base.ResolveReference(ref).String()
	}
	return ref.String()
}

// extractArticle 从已解析的详情文档提取字段构造文章（不查库、不发请求——查重与翻页由调用方负责）。
// missed 非 nil 时收集未命中的字段选择器名，随“详情页提取完成”日志汇总输出，便于核对配置
func (t *httpTask) extractArticle(doc *goquery.Document, link string, missed *[]string) (*entity.Article, error) {
	noteMiss := func(name string) {
		if missed != nil {
			*missed = append(*missed, name)
		}
	}
	field := func(name, sel string) *goquery.Selection {
		s := doc.Find(sel).First()
		if s.Length() == 0 {
			noteMiss(name)
			return nil
		}
		return s
	}

	// 标题：title_sel + title_attr/title_extract_regex（统一提取模型），回退 <title>
	title := ""
	if t.TitleSel != "" {
		if s := field("title_sel", t.TitleSel); s != nil {
			title = applyExtractRegex(t.TitleExtractRegex, httpElementValue(s, t.TitleAttr))
		}
	}
	if title == "" {
		title = strings.TrimSpace(doc.Find("title").First().Text())
	}
	if title == "" {
		return nil, fmt.Errorf("标题提取失败（核对 title_sel，或页面无 <title>）")
	}

	// 发布时间：publish_time_sel 优先，回退 meta[property=article:published_time]；解析失败用采集时刻
	createAt := time.Now().Unix()
	publishRaw := ""
	if t.PublishTimeSel != "" {
		if s := field("publish_time_sel", t.PublishTimeSel); s != nil {
			publishRaw = applyExtractRegex(t.PublishTimeExtractRegex, httpElementValue(s, t.PublishTimeAttr))
		}
	}
	if publishRaw == "" {
		if s := field("published_time_meta", `meta[property="article:published_time"]`); s != nil {
			publishRaw = strings.TrimSpace(s.AttrOr("content", ""))
		}
	}
	if v := parsePublishTime(publishRaw); v > 0 {
		createAt = v
	}

	item := &entity.Article{
		ArticleBase: entity.ArticleBase{
			Slug:        t.dedupSlug(link, title),
			Title:       title,
			CategoryID:  t.CategoryID,
			Status:      true,
			CreateTime:  createAt,
			ContentType: t.ContentType,
		},
	}

	// 关键词：keywords_sel 优先，回退 meta[name=keywords]
	keywords := ""
	if t.KeywordsSel != "" {
		if s := field("keywords_sel", t.KeywordsSel); s != nil {
			keywords = applyExtractRegex(t.KeywordsExtractRegex, httpElementValue(s, t.KeywordsAttr))
		}
	}
	if keywords == "" {
		if s := field("keywords_meta", `meta[name="keywords"]`); s != nil {
			keywords = strings.TrimSpace(s.AttrOr("content", ""))
		}
	}
	item.Keywords = truncateRunes(keywords, 250)

	// 封面：cover_sel 按 cover_attr（空=自动 src→content）取值，回退 og:image meta；
	// 取到后按详情页 URL 绝对化（属性/正则抽出常是相对路径）
	if t.CoverSel != "" {
		if s := field("cover_sel", t.CoverSel); s != nil {
			item.Thumbnail = t.coverValue(s)
		}
	}
	if item.Thumbnail == "" {
		if s := field("og_image_meta", `meta[property="og:image"]`); s != nil {
			item.Thumbnail = strings.TrimSpace(s.AttrOr("content", ""))
		}
	}
	if item.Thumbnail != "" {
		if base, berr := url.Parse(link); berr == nil {
			if ref, perr := url.Parse(item.Thumbnail); perr == nil {
				item.Thumbnail = base.ResolveReference(ref).String()
			}
		}
	}

	// 正文（翻页拼接由调用方逐页 fetch 后调用 contentOf 追加）
	item.Content = t.contentOf(doc)
	item.Description = plainSummary(item.Content, 120)

	// 类型专属数据：视频源（直链 embed=false + iframe 嵌入 embed=true）/ 图集
	// （写入 extends 对接 wolves 消费页；*_attr 支持懒加载 data-src）
	var extends vo.Extends
	if t.VideoSrcSel != "" || t.VideoIframeSel != "" {
		var labels []string
		if t.VideoLabelSel != "" {
			labels = t.attrList(doc, t.VideoLabelSel, t.VideoLabelAttr, "")
		}
		direct := t.attrList(doc, t.VideoSrcSel, t.videoAttr(), t.VideoExtractRegex)
		embeds := t.attrList(doc, t.VideoIframeSel, t.videoIframeAttr(), t.VideoIframeExtractRegex)
		sources := buildVideoSources(direct, labels, false, 0)
		embedLabels := labels
		if len(embedLabels) > len(direct) {
			embedLabels = embedLabels[len(direct):] // 集名按“直链在前、iframe 在后”的全序对应
		} else {
			embedLabels = nil
		}
		sources = append(sources, buildVideoSources(embeds, embedLabels, true, len(direct))...)
		if len(sources) > 0 {
			extends = append(extends, vo.ExtendsItem{Key: "video_sources", Value: sources})
		}
	}
	if t.GallerySel != "" {
		if srcs := t.attrList(doc, t.GallerySel, t.galleryAttr(), t.GalleryExtractRegex); len(srcs) > 0 {
			extends = append(extends, vo.ExtendsItem{Key: "gallery_images", Value: srcs})
		}
	}

	// 通用 extends 键值提取；selector="@url" 时跳过 DOM，从详情页 URL 本身提取
	for _, ex := range t.Extra {
		if ex.Key == "" || ex.Selector == "" {
			continue
		}
		if ex.Selector == "@url" {
			if v := applyExtractRegex(ex.Regex, link); v != "" {
				extends = append(extends, vo.ExtendsItem{Key: ex.Key, Value: v})
			}
			continue
		}
		if ex.Multiple {
			if vals := t.attrList(doc, ex.Selector, ex.Attr, ex.Regex); len(vals) > 0 {
				extends = append(extends, vo.ExtendsItem{Key: ex.Key, Value: vals})
			}
		} else if s := doc.Find(ex.Selector).First(); s.Length() > 0 {
			if v := applyExtractRegex(ex.Regex, httpElementValue(s, ex.Attr)); v != "" {
				extends = append(extends, vo.ExtendsItem{Key: ex.Key, Value: v})
			} else {
				noteMiss("extra:" + ex.Key)
			}
		} else {
			noteMiss("extra:" + ex.Key)
		}
	}
	item.Extends = extends

	// 图集地址按详情页 URL 绝对化（与封面同理）
	if base, berr := url.Parse(link); berr == nil {
		for i := range extends {
			if extends[i].Key != "gallery_images" {
				continue
			}
			if srcs, ok := extends[i].Value.([]string); ok {
				for j, src := range srcs {
					if ref, perr := url.Parse(src); perr == nil {
						srcs[j] = base.ResolveReference(ref).String()
					}
				}
			}
		}
	}
	return item, nil
}

// coverValue 封面取值：cover_attr 留空=自动（img 的 src 优先，回退 meta 的 content），
// 再经 cover_extract_regex 抽取（如 style 的 background-image: url(...)）
func (t *httpTask) coverValue(s *goquery.Selection) string {
	v := ""
	if t.CoverAttr == "" {
		v = httpAttrWithFallback(s, "src")
		if v == "" {
			v = strings.TrimSpace(s.AttrOr("content", ""))
		}
	} else {
		v = httpAttrWithFallback(s, t.CoverAttr)
	}
	return applyExtractRegex(t.CoverExtractRegex, v)
}

// ---- 采集编排（网络层）----

// runTask 单任务采集：逐列表页取链/出文 → 详情页并发提取入库。返回本任务汇总
func (h *HttpSpider) runTask(t *httpTask) taskResult {
	fetcher := newHTTPFetcher(h, t)
	st := &taskStats{}
	var mu sync.Mutex // 并发 worker 更新 st 与提前结束判定加锁

	h.ctx.Log.Info("执行任务", zap.String("name", t.Name), zap.String("source", t.SourceURL),
		zap.String("mode", t.Mode), zap.Int("max_pages", t.MaxPages))

	// 处理单页文档：detail 取链并发处理 / list 直接出文。
	// body 为该页原始响应（反爬特征检测用）。返回 false 表示应提前结束翻页
	visitPage := func(doc *goquery.Document, base *url.URL, pageURL string, body []byte) bool {
		if t.Mode == "list" {
			articles := t.extractListArticles(doc, base)
			h.ctx.Log.Info("列表页解析完成", zap.String("task", t.Name),
				zap.String("url", pageURL), zap.Int("articles", len(articles)))
			if len(articles) == 0 {
				if sign := detectBlockSigns(string(body)); sign != "" {
					h.ctx.Log.Warn("疑似反爬拦截页", zap.String("sign", sign),
						zap.String("hint", "可配 user_agent/headers/cookies 或降低并发后重试"))
				} else {
					h.ctx.Log.Warn("未提取到列表文章——请核对 list_selector/list_title_sel/link_include 配置")
				}
				return false
			}
			for _, article := range articles {
				if h.processArticle(t, article, st, &mu) {
					return false
				}
			}
			return true
		}

		links := t.extractLinks(doc, base)
		h.ctx.Log.Info("列表页解析完成", zap.String("task", t.Name),
			zap.String("url", pageURL), zap.Int("links", len(links)))
		if len(links) == 0 {
			if sign := detectBlockSigns(string(body)); sign != "" {
				h.ctx.Log.Warn("疑似反爬拦截页", zap.String("sign", sign),
					zap.String("hint", "可配 user_agent/headers/cookies 或降低并发后重试"))
			} else {
				h.ctx.Log.Warn("未提取到链接——请核对 list_selector/link_include 配置（或已采完）")
			}
			return false
		}
		h.processLinks(t, fetcher, links, st, &mu)
		mu.Lock()
		stop := st.stopTask // stop_when_exists/limit 已触发则终止翻页，不再请求后续列表页
		mu.Unlock()
		return !stop
	}

	if t.NextPageSel != "" {
		// next_page_sel 模式：从起始页开始，逐页从 DOM 解析「下一页」链接 GET 翻页
		pageURL := t.pageURL(1)
		for i := 1; i <= t.MaxPages; i++ {
			doc, body, err := h.fetchDoc(fetcher, t, pageURL)
			if err != nil {
				mu.Lock()
				st.failed++
				mu.Unlock()
				h.saveDebugHTML(t, body, "list_error")
				h.ctx.Log.Error("列表页采集失败", zap.String("task", t.Name),
					zap.String("url", pageURL), zap.Error(err))
				break
			}
			base := resolveBase(pageURL, doc)
			h.ctx.Log.Info("采集列表页", zap.String("task", t.Name), zap.Int("page", i), zap.String("url", pageURL))
			if !visitPage(doc, base, pageURL, body) {
				break
			}
			next := findNextURL(doc, t.NextPageSel, base)
			if next == "" {
				if i == 1 {
					h.ctx.Log.Warn("next_page_sel 未找到下一页链接（纯 HTTP 不支持 JS 按钮翻页，此类站点请用 HeadlessSpider）",
						zap.String("selector", t.NextPageSel))
				}
				break
			}
			if next == pageURL {
				break // 翻页地址未变化（如「下一页」指向当前页），防死循环
			}
			pageURL = next
		}
	} else {
		// page_url_pattern 模式（默认）：按模板生成每页地址，单页失败继续下一页
		for i := 1; i <= t.MaxPages; i++ {
			pageURL := t.pageURL(i)
			doc, body, err := h.fetchDoc(fetcher, t, pageURL)
			if err != nil {
				mu.Lock()
				st.failed++
				mu.Unlock()
				h.saveDebugHTML(t, body, "list_error")
				h.ctx.Log.Error("列表页采集失败", zap.String("task", t.Name),
					zap.Int("page", i), zap.String("url", pageURL), zap.Error(err))
				continue
			}
			base := resolveBase(pageURL, doc)
			h.ctx.Log.Info("采集列表页", zap.String("task", t.Name), zap.Int("page", i), zap.String("url", pageURL))
			if !visitPage(doc, base, pageURL, body) {
				break
			}
		}
	}

	h.ctx.Log.Info("任务完成", zap.String("name", t.Name),
		zap.Int("collected", st.collected), zap.Int("skipped", st.skipped), zap.Int("failed", st.failed))
	return taskResult{Collected: st.collected, Skipped: st.skipped, Failed: st.failed}
}

// resolveBase 文档基准 URL：优先重定向后的最终地址（doc.Url），否则用请求地址
func resolveBase(pageURL string, doc *goquery.Document) *url.URL {
	if doc != nil && doc.Url != nil && doc.Url.String() != "" {
		return doc.Url
	}
	if u, err := url.Parse(pageURL); err == nil {
		return u
	}
	return &url.URL{}
}

// processLinks 详情链接并发处理：worker 池上限 h.Concurrency，共享 st 加锁。
// 早停/limit 触发时置位 st.stopTask（跨列表页生效，visitPage 依据它终止翻页）并停止投递，等待在途 worker 完成
func (h *HttpSpider) processLinks(t *httpTask, f *httpFetcher, links []httpLink, st *taskStats, mu *sync.Mutex) {
	sem := make(chan struct{}, h.Concurrency)
	var wg sync.WaitGroup
	for _, link := range links {
		mu.Lock()
		if st.stopTask || (h.Limit > 0 && st.collected >= h.Limit) {
			mu.Unlock()
			break
		}
		mu.Unlock()

		// 列表标题预查重：已存在则不请求详情页
		if link.Title != "" {
			slug := t.dedupSlug(link.URL, link.Title)
			if exists, err := service.Article.ExistsSlug(slug); err == nil && exists {
				mu.Lock()
				st.skipped++
				st.consecExists++
				if h.hitStop(t, st) {
					st.stopTask = true
					mu.Unlock()
					break
				}
				mu.Unlock()
				continue
			}
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(link httpLink) {
			defer wg.Done()
			defer func() { <-sem }()
			article, err := h.fetchArticleRetry(f, t, link.URL)
			if err != nil {
				mu.Lock()
				st.failed++
				mu.Unlock()
				h.ctx.Log.Error("采集失败", zap.String("task", t.Name), zap.String("url", link.URL), zap.Error(err))
				return
			}
			if article == nil { // 详情页判重已存在
				mu.Lock()
				st.skipped++
				st.consecExists++
				if h.hitStop(t, st) {
					st.stopTask = true
				}
				mu.Unlock()
				return
			}
			if h.processArticle(t, article, st, mu) {
				mu.Lock()
				st.stopTask = true
				mu.Unlock()
			}
		}(link)
	}
	wg.Wait()
}

// processArticle 单篇入库（试运行只记录不入库；limit 达标或连续已存在早停返回 true，调用方置位 st.stopTask 结束任务）。
// 统计与入库判定在锁内串行（DB 查重+写入毫秒级，并发 worker 串行化换取防重复入库的正确性）；
// interval 睡眠在锁外，不阻塞其他 worker
func (h *HttpSpider) processArticle(t *httpTask, article *entity.Article, st *taskStats, mu *sync.Mutex) bool {
	mu.Lock()
	defer mu.Unlock()
	exists, err := service.Article.ExistsSlug(article.Slug)
	if err != nil {
		st.failed++
		h.ctx.Log.Error("查重失败", zap.String("slug", article.Slug), zap.Error(err))
		return false
	}
	if exists {
		st.skipped++
		st.consecExists++
		h.ctx.Log.Debug("文章已存在，跳过", zap.String("slug", article.Slug))
		return h.hitStop(t, st)
	}
	st.consecExists = 0

	if h.Limit > 0 && st.collected >= h.Limit {
		return true
	}
	if h.DryRun {
		st.collected++
		h.ctx.Log.Info("[试运行] 命中文章（未入库）", zap.String("task", t.Name),
			zap.String("title", article.Title), zap.String("slug", article.Slug),
			zap.Int("content_len", len(article.Content)))
		return h.Limit > 0 && st.collected >= h.Limit
	}
	if err := service.Article.Create(article); err != nil {
		st.failed++
		h.ctx.Log.Error("创建文章失败", zap.String("title", article.Title), zap.Error(err))
		return false
	}
	st.collected++
	h.ctx.Log.Info("成功采集", zap.String("task", t.Name), zap.String("title", article.Title), zap.String("slug", article.Slug))
	if h.Interval > 0 {
		mu.Unlock()
		time.Sleep(time.Duration(h.Interval) * time.Second)
		mu.Lock()
	}
	return h.Limit > 0 && st.collected >= h.Limit
}

// hitStop 连续已存在计数是否触发提前结束（调用方须持有 mu；命中后由调用方置位 st.stopTask 终止整个任务）
func (h *HttpSpider) hitStop(t *httpTask, st *taskStats) bool {
	if t.StopWhenExists > 0 && st.consecExists >= t.StopWhenExists {
		h.ctx.Log.Info("连续多篇已存在，提前结束任务（终止翻页）",
			zap.String("task", t.Name), zap.Int("consecutive", st.consecExists))
		return true
	}
	return false
}

// fetchArticleRetry 详情页抓取带重试（间隔逐次递增 1s/2s/...）。
// 文章已存在返回 (nil, nil)
func (h *HttpSpider) fetchArticleRetry(f *httpFetcher, t *httpTask, link string) (*entity.Article, error) {
	var lastErr error
	for attempt := 0; attempt <= h.Retry; attempt++ {
		if attempt > 0 {
			h.ctx.Log.Warn("详情页采集失败，重试",
				zap.String("url", link), zap.Int("attempt", attempt), zap.Error(lastErr))
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		article, err := h.fetchArticleDetail(f, t, link)
		if err == nil {
			return article, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// fetchArticleDetail 抓取详情页并提取字段；配置了正文翻页时逐页拼接。
// 已存在（预查重未拦下、标题变化的更新场景）返回 (nil, nil)
func (h *HttpSpider) fetchArticleDetail(f *httpFetcher, t *httpTask, link string) (*entity.Article, error) {
	doc, body, err := h.fetchDoc(f, t, link)
	if err != nil {
		h.saveDebugHTML(t, body, "detail_error")
		return nil, err
	}

	// URL 键预查重（dedup_by=url 时列表标题预查的键与详情页不一致，这里统一拦一次，
	// 避免已采集文章再走正文翻页的无谓请求）
	if t.DedupBy == "url" {
		if exists, err := service.Article.ExistsSlug(hashSlug(link, "")); err == nil && exists {
			h.ctx.Log.Debug("文章已存在（URL 键预查），跳过", zap.String("url", link))
			return nil, nil
		}
	}

	var missed []string
	article, err := t.extractArticle(doc, link, &missed)
	if err != nil {
		h.saveDebugHTML(t, body, "detail_error")
		return nil, err
	}
	if len(missed) > 0 {
		h.ctx.Log.Debug("部分字段选择器未命中", zap.String("url", link), zap.Strings("missed", missed))
	}

	// 详情页判重
	exists, err := service.Article.ExistsSlug(article.Slug)
	if err != nil {
		return nil, err
	}
	if exists {
		h.ctx.Log.Debug("文章已存在，跳过", zap.String("slug", article.Slug))
		return nil, nil
	}

	h.collectContentPages(f, t, link, doc, article)

	h.ctx.Log.Info("详情页提取完成", zap.String("url", link),
		zap.String("title", truncateRunes(article.Title, 40)),
		zap.Int("content_len", len(article.Content)),
		zap.Bool("cover", article.Thumbnail != ""),
		zap.Int("extends", len(article.Extends)))
	return article, nil
}

// collectContentPages 正文翻页：逐页 GET「下一页」并追加正文
// （内容无新增即停，防「下一页」永在的站）；fetchArticleDetail 与 ValidateTask 共用
func (h *HttpSpider) collectContentPages(f *httpFetcher, t *httpTask, link string, doc *goquery.Document, article *entity.Article) {
	if t.ContentSel == "" || t.ContentNextSel == "" {
		return
	}
	maxPages := t.ContentMaxPages
	if maxPages <= 0 {
		maxPages = 20
	}
	contentHTML := article.Content
	curURL, curDoc := link, doc
	for i := 1; i < maxPages; i++ {
		next := findNextURL(curDoc, t.ContentNextSel, resolveBase(curURL, curDoc))
		if next == "" || next == curURL {
			break
		}
		pageDoc, _, perr := h.fetchDoc(f, t, next)
		if perr != nil {
			h.ctx.Log.Warn("正文翻页失败", zap.String("next", next), zap.Error(perr))
			break
		}
		chunk := t.contentOf(pageDoc)
		if chunk == "" || strings.HasSuffix(contentHTML, chunk) {
			break
		}
		contentHTML += "\n" + chunk
		curURL, curDoc = next, pageDoc
	}
	article.Content = contentHTML
	article.Description = plainSummary(contentHTML, 120)
}

// saveDebugHTML 调试目录非空时保存失败现场响应 HTML（排查 selector/编码问题）
func (h *HttpSpider) saveDebugHTML(t *httpTask, body []byte, tag string) {
	if h.DebugDir == "" || len(body) == 0 {
		return
	}
	if err := os.MkdirAll(h.DebugDir, 0o755); err != nil {
		return
	}
	name := fmt.Sprintf("%s_%s_%d.html", time.Now().Format("20060102_150405"), tag, time.Now().UnixNano()%1000000)
	if t != nil && t.Name != "" {
		name = fmt.Sprintf("%s_%s_%s_%d.html", time.Now().Format("20060102_150405"),
			sanitizeFilename(t.Name), tag, time.Now().UnixNano()%1000000)
	}
	path := filepath.Join(h.DebugDir, name)
	if err := os.WriteFile(path, body, 0o644); err == nil {
		h.ctx.Log.Warn("已保存失败现场响应", zap.String("path", path))
	}
}

// ValidateTask 任务校验（不入库）：抓列表页第一页 → 取第一条链接 → 提取文章字段并返回结构化结果。
// 供 PluginService.SpiderValidate 接口调用；跳过全部判重短路，已入库文章也照样提取
func (h *HttpSpider) ValidateTask(raw string) (interface{}, error) {
	var t httpTask
	if err := validateUnmarshalTask(raw, &t); err != nil {
		return nil, err
	}
	if err := normalizeHTTPTask(&t, "任务"); err != nil {
		return nil, err
	}
	start := time.Now()
	res := newValidateResult("HttpSpider", t.Name, t.SourceURL)
	defer func() { res.CostMS = time.Since(start).Milliseconds() }()

	if t.Mode == "list" {
		res.Error = "list 模式（列表页直接入库）无详情页字段可校验，请切换为 detail 模式验证"
		return res, nil
	}

	f := newHTTPFetcher(h, &t)
	doc, body, err := h.fetchDoc(f, &t, t.SourceURL)
	if err != nil {
		h.saveDebugHTML(&t, body, "validate_list_error")
		res.Error = fmt.Sprintf("列表页抓取失败: %v", err)
		return res, nil
	}
	links := t.extractLinks(doc, resolveBase(t.SourceURL, doc))
	res.LinksFound = len(links)
	if len(links) == 0 {
		res.Error = "列表页未提取到链接：核对「详情链接选择器」、链接过滤（只保留/排除）与取链属性通道"
		return res, nil
	}
	res.SampleLinks = validateSampleLinks(5, func(i int) ValidateLink {
		return ValidateLink{URL: links[i].URL, Title: links[i].Title}
	})
	res.FirstLink = res.SampleLinks[0]

	detailDoc, dbody, err := h.fetchDoc(f, &t, links[0].URL)
	if err != nil {
		h.saveDebugHTML(&t, dbody, "validate_detail_error")
		res.Error = fmt.Sprintf("详情页抓取失败: %v", err)
		return res, nil
	}
	var missed []string
	article, err := t.extractArticle(detailDoc, links[0].URL, &missed)
	if err != nil {
		h.saveDebugHTML(&t, dbody, "validate_detail_error")
		res.Error = fmt.Sprintf("详情页提取失败: %v", err)
		return res, nil
	}
	h.collectContentPages(f, &t, links[0].URL, detailDoc, article)
	res.Article = articleToValidate(article, missed)
	return res, nil
}
