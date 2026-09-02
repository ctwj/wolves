package plugins

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"go.uber.org/zap"

	"moss/domain/core/entity"
	"moss/domain/core/service"
	"moss/domain/core/vo"
	pluginEntity "moss/domain/support/entity"
)

// HeadlessSpider 通用无头浏览器采集插件
// 面向 JS 动态渲染 + 反调试站点的采集器：浏览器全局配置 + JSON 任务数组（多站/多分类），
// 每个任务独立配置选择器/翻页/过滤/入库类型；支持任意 extends 键值提取，
// 视频源/图集写入 extends 后直接对接 wolves 多媒体消费页。
type HeadlessSpider struct {
	BrowserPath string `json:"browser_path"` // 浏览器路径（Chrome/Edge；留空自动查找，查找失败时必填）
	Proxy       string `json:"proxy"`        // 浏览器代理（如 http://127.0.0.1:7890），留空不用
	Headless    bool   `json:"headless"`     // 无头模式（调试时可关）
	Timeout     int    `json:"timeout"`      // 单页加载超时秒数（默认 30）
	Interval    int    `json:"interval"`     // 页面间隔秒数（默认 3，全局）
	Tasks       string `json:"tasks"`        // 任务数组 JSON（见 About），多站多分类在此配置

	Retry    int    `json:"retry"`     // 详情页采集失败重试次数（默认 0）
	Limit    int    `json:"limit"`     // 每任务最多入库篇数（0=不限；配合 dry_run 做小样调试）
	DryRun   bool   `json:"dry_run"`   // 试运行：只解析与打日志，不入库（调试选择器用）
	DebugDir string `json:"debug_dir"` // 调试目录：列表页/详情页失败时保存页面截图（留空关闭）

	ctx *pluginEntity.Plugin
}

// spiderTask 单个采集任务
type spiderTask struct {
	Name           string `json:"name"`             // 任务名（日志区分）
	Enable         bool   `json:"enable"`           // 是否启用
	SourceURL      string `json:"source_url"`       // 起始页 URL
	PageURLPattern string `json:"page_url_pattern"` // 翻页模板，含 {page}；留空只采起始页
	MaxPages       int    `json:"max_pages"`        // 最多翻页数（默认 1；next_page_sel 模式默认 50）

	Mode         string `json:"mode"`          // detail=进详情页提取（默认）；list=直接从列表条目出文章（不进详情页，配 list_cover_sel/list_desc_sel）
	WaitSelector string `json:"wait_selector"` // 渲染完成等待元素；留空等 DOM 稳定
	ListSelector string `json:"list_selector"` // 列表页详情链接选择器
	LinkInclude  string `json:"link_include"`  // 链接过滤：子串或 /正则/；留空不过滤
	LinkExclude  string `json:"link_exclude"`  // 链接排除：子串或 /正则/

	// 翻页方式：page_url_pattern（URL 模板，默认）之外还有两种——
	// next_page_sel「下一页」按钮/链接选择器（SPA 常见；a[href] 直接导航，无 href 则点击，MaxPages 封顶）；
	// scroll_times 列表页加载后先滚动到底 N 次（无限滚动/“加载更多”/图片懒加载列表）
	NextPageSel string `json:"next_page_sel"`
	ScrollTimes int    `json:"scroll_times"`

	ListTitleSel  string `json:"list_title_sel"`  // 列表条目标题选择器（预查重/列表出文用；回退 h1~h6/首行文本）
	ListCoverSel  string `json:"list_cover_sel"`  // mode=list 条目封面选择器
	ListCoverAttr string `json:"list_cover_attr"` // mode=list 封面读哪个属性（默认 src；懒加载可填 data-src）
	ListDescSel   string `json:"list_desc_sel"`   // mode=list 条目摘要选择器（回退条目文本）

	LinkMode string `json:"link_mode"` // 链接获取：空/href=属性提取（默认）| click=模拟点击劫持 window.open（自动叠加属性提取，适配 javascript:void(0) 点击跳转站）
	// 属性提取通道的组合配置（href 与 click 模式均生效）：
	// link_selector 定位条目内的链接元素（空=条目自身），适配容器型条目；
	// link_attr 读哪个属性（默认 href，可为 data-url/data-src 等）；
	// link_extract_regex 从属性值抽取 URL 的正则（可 /re/ 包裹，取第一个捕获组，无捕获组取整段匹配），
	//   适配 onclick="location.href='...'" 这类内嵌地址；
	// link_url_template 拼接最终 URL（含 {value} 则以属性值替换，否则视为前缀拼接），
	//   适配 data-id 拼详情地址的站点
	LinkSelector     string `json:"link_selector"`
	LinkAttr         string `json:"link_attr"`
	LinkExtractRegex string `json:"link_extract_regex"`
	LinkURLTemplate  string `json:"link_url_template"`
	// click 模式下默认只收与列表页同源的跳转（广告弹窗几乎都是跨域，自动丢弃）；
	// 详情确在另一域名时才置 true
	LinkCrossOrigin bool `json:"link_cross_origin"`
	// click 模式逐条目点击后等待跳转的毫秒数（默认 250）
	ClickWaitMs int `json:"click_wait_ms"`

	StopWhenExists int `json:"stop_when_exists"` // 连续 N 篇已存在则提前结束任务（0=不启用）；定时增量采集推荐 3~5

	TitleSel    string `json:"title_sel"`    // 详情页标题选择器（回退 <title>）
	CoverSel    string `json:"cover_sel"`    // 封面选择器（取 src；回退 og:image）
	ContentSel  string `json:"content_sel"`  // 正文容器选择器（取 innerHTML）
	KeywordsSel string `json:"keywords_sel"` // 关键词选择器（优先读 content 属性，回退文本；缺省回退 meta[name=keywords]）
	// 正文翻页：详情页正文分页时，content_next_sel 指向“下一页”按钮/链接，逐页拼接正文
	// （a[href] 直接导航，无 href 则点击；content_max_pages 安全上限，默认 20）
	ContentNextSel  string `json:"content_next_sel"`
	ContentMaxPages int    `json:"content_max_pages"`
	// 发布时间选择器（兼容 content 属性与文本；缺省回退 meta[property=article:published_time]；
	// 解析失败用采集时刻）
	PublishTimeSel string `json:"publish_time_sel"`

	// 播放源：video_src_sel=直链（video 标签，embed=false）；video_iframe_sel=第三方播放页（iframe，embed=true）。
	// 懒加载站 *_attr 可填 data-src；video_label_sel 给出与播放源同数量、同顺序（直链在前、iframe 在后）的
	// 集名元素（缺省回退“第NN集”），video_label_attr 非空时读集名元素属性而非文本
	VideoSrcSel     string `json:"video_src_sel"`     // 直链播放源选择器（video src）→ extends[video_sources]
	VideoAttr       string `json:"video_attr"`        // 直链读哪个属性（默认 src）
	VideoIframeSel  string `json:"video_iframe_sel"`  // iframe 播放源选择器（embed=true）→ extends[video_sources]
	VideoIframeAttr string `json:"video_iframe_attr"` // iframe 读哪个属性（默认 src）
	VideoLabelSel   string `json:"video_label_sel"`   // 集名元素选择器（与播放源一一对应）
	VideoLabelAttr  string `json:"video_label_attr"`  // 集名读哪个属性（空=取文本）

	GallerySel  string `json:"gallery_sel"`  // 图集图片选择器 → extends[gallery_images]
	GalleryAttr string `json:"gallery_attr"` // 图集读哪个属性（默认 src；懒加载可填 data-src）

	Extra []spiderExtra `json:"extra"` // 通用 extends 键值提取（任意 key）

	ContentType string `json:"content_type"` // 入库类型：standard/novel/image/video
	CategoryID  int    `json:"category_id"`  // 入库分类 ID

	// 登录态/指纹：每任务浏览器 UA 与 cookies
	// （cookies 为 JSON 数组 [{"name":"..","value":"..","domain":"..","path":".."}]，domain 缺省绑定 source_url 站点）
	UserAgent string `json:"user_agent"`
	Cookies   string `json:"cookies"`

	// 去重键：默认/ url_title = 源URL+标题 哈希；"url" = 仅按源 URL（站点标题微调不会重复入库；
	// 注意切换 dedup_by 后既有文章会按新键被重新采集）
	DedupBy string `json:"dedup_by"`
}

// spiderExtra 通用 extends 字段提取规则
type spiderExtra struct {
	Key      string `json:"key"`      // extends 键名
	Selector string `json:"selector"` // CSS 选择器
	Attr     string `json:"attr"`     // 属性名（src/href/...）；空=取文本；"html"=innerHTML
	Multiple bool   `json:"multiple"` // 多值聚合为数组
}

func NewHeadlessSpider() *HeadlessSpider {
	return &HeadlessSpider{
		Headless: true,
		Timeout:  30,
		Interval: 3,
		Tasks:    "[]",
	}
}

func (h *HeadlessSpider) Info() *pluginEntity.PluginInfo {
	return &pluginEntity.PluginInfo{
		ID: "HeadlessSpider",
		About: "通用无头采集插件（多任务）：tasks 配置 JSON 任务数组，每任务独立选择器/翻页/过滤/入库类型。" +
			"取链三通道：① href 属性（link_selector+link_attr）；② 属性值正则抽取+模板拼URL（link_extract_regex+link_url_template，适配 data-id 拼详情链接）；" +
			"③ link_mode=click 模拟点击劫持 window.open（逐条目精确配对标题，默认仅同源防广告）。" +
			"翻页：page_url_pattern（URL 模板）/ next_page_sel（下一页按钮）/ scroll_times（滚动加载）。" +
			"mode=list 直接从列表出文章（list_cover_sel/list_desc_sel）；正文翻页 content_next_sel+content_max_pages；" +
			"keywords_sel/publish_time_sel 提取关键词与发布时间（回退 meta）；" +
			"视频源 video_src_sel（直链 embed=false）+ video_iframe_sel（iframe 嵌入 embed=true）+ 懒加载 video_attr/video_iframe_attr/gallery_attr + 集名 video_label_sel；" +
			"登录态 user_agent/cookies；全局 retry 重试 / limit 限量 / dry_run 试运行 / debug_dir 失败截图 / dedup_by=url 按 URL 去重",
		RunEnable:  true,
		CronEnable: true,
		PluginInfoPersistent: pluginEntity.PluginInfoPersistent{
			CronStart: false,
			CronExp:   "@every 1h",
		},
	}
}

func (h *HeadlessSpider) Load(ctx *pluginEntity.Plugin) error {
	h.ctx = ctx
	// 数据库旧记录的 cron_exp 为空时补默认值，避免装载失败
	if ctx.Info.CronEnable && ctx.Info.CronExp == "" {
		ctx.Info.CronExp = "@every 1h"
	}
	return nil
}

func (h *HeadlessSpider) Run(ctx *pluginEntity.Plugin) error {
	h.ctx = ctx
	if h.Timeout <= 0 {
		h.Timeout = 30
	}
	if h.Interval <= 0 {
		h.Interval = 3
	}
	if h.Retry < 0 {
		h.Retry = 0
	}

	tasks, err := h.parseTasks()
	if err != nil {
		return err
	}
	enabled := 0
	for _, t := range tasks {
		if t.Enable {
			enabled++
		}
	}
	if enabled == 0 {
		return fmt.Errorf("tasks 中没有启用（enable=true）的任务")
	}

	browser, err := h.launchBrowser()
	if err != nil {
		return fmt.Errorf("启动浏览器失败（本机需有 Chrome/Edge，或配置 browser_path）: %w", err)
	}
	defer browser.MustClose()

	h.ctx.Log.Info("开始无头采集", zap.Int("tasks", enabled),
		zap.Bool("dry_run", h.DryRun), zap.Int("limit", h.Limit))
	startTime := time.Now()
	for i, task := range tasks {
		if !task.Enable {
			continue
		}
		h.ctx.Log.Info("执行任务", zap.Int("idx", i+1), zap.String("name", task.Name), zap.String("source", task.SourceURL))
		c, s, f := h.runTask(browser, &task)
		h.ctx.Log.Info("任务完成", zap.String("name", task.Name),
			zap.Int("collected", c), zap.Int("skipped", s), zap.Int("failed", f))
	}
	h.ctx.Log.Info("全部任务结束", zap.Duration("cost", time.Since(startTime)))
	return nil
}

func (h *HeadlessSpider) parseTasks() ([]spiderTask, error) {
	raw := strings.TrimSpace(h.Tasks)
	if raw == "" {
		return nil, fmt.Errorf("tasks 未配置")
	}
	var tasks []spiderTask
	if err := json.Unmarshal([]byte(raw), &tasks); err != nil {
		return nil, fmt.Errorf("tasks JSON 解析失败: %w", err)
	}
	for i := range tasks {
		t := &tasks[i]
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
			return nil, fmt.Errorf("任务[%d]%s mode 无效（仅支持 detail/list）: %q", i+1, t.Name, t.Mode)
		}
		if t.SourceURL == "" || t.ListSelector == "" {
			return nil, fmt.Errorf("任务[%d]%s 缺少 source_url 或 list_selector", i+1, t.Name)
		}
		if t.LinkMode != "" && t.LinkMode != "href" && t.LinkMode != "click" {
			return nil, fmt.Errorf("任务[%d]%s link_mode 无效（仅支持 href/click）: %q", i+1, t.Name, t.LinkMode)
		}
		if t.DedupBy != "" && t.DedupBy != "url" && t.DedupBy != "url_title" {
			return nil, fmt.Errorf("任务[%d]%s dedup_by 无效（仅支持 url/url_title）: %q", i+1, t.Name, t.DedupBy)
		}
		if t.ClickWaitMs <= 0 {
			t.ClickWaitMs = 250
		}
		if t.LinkExtractRegex != "" {
			if _, err := compileLinkRegex(t.LinkExtractRegex); err != nil {
				return nil, fmt.Errorf("任务[%d]%s link_extract_regex 无效: %w", i+1, t.Name, err)
			}
		}
		if _, err := parseCookies(t.Cookies, ""); err != nil {
			return nil, fmt.Errorf("任务[%d]%s %w", i+1, t.Name, err)
		}
	}
	return tasks, nil
}

// taskStats 任务内累计计数（processLink/processArticle 更新）
type taskStats struct {
	collected, skipped, failed, consecExists int
}

func (h *HeadlessSpider) runTask(browser *rod.Browser, t *spiderTask) (collected, skipped, failed int) {
	st := &taskStats{}
	if t.Mode == "list" {
		// list 模式：直接从列表条目出文章，不进详情页（取链走 href 属性通道）
		h.eachListPage(browser, t, st, func(page *rod.Page, base *url.URL, title string) bool {
			articles := h.extractListArticles(page, t, base)
			h.ctx.Log.Info("列表页渲染完成", zap.String("task", t.Name),
				zap.String("page_title", title), zap.Int("articles", len(articles)))
			if len(articles) == 0 {
				h.ctx.Log.Warn("未提取到列表文章——请核对 list_selector/list_title_sel/link_include 配置")
				return false
			}
			for _, article := range articles {
				if h.processArticle(t, article, st) {
					return false
				}
			}
			return true
		})
		return st.collected, st.skipped, st.failed
	}

	// detail 模式：列表取链 → 逐链接进详情页提取
	h.eachListPage(browser, t, st, func(page *rod.Page, base *url.URL, title string) bool {
		links, lerr := h.linksOnPage(page, t, base)
		if lerr != nil {
			st.failed++
			h.ctx.Log.Error("列表页取链失败", zap.String("task", t.Name), zap.Error(lerr))
			return true // 页级失败，继续尝试下一页
		}
		h.ctx.Log.Info("列表页渲染完成", zap.String("task", t.Name),
			zap.String("page_title", title), zap.Int("links", len(links)))
		if len(links) == 0 {
			h.ctx.Log.Warn("未提取到链接——请核对 list_selector/wait_selector/link_include 配置")
			return false
		}
		for _, link := range links {
			if h.processLink(browser, t, link, st) {
				return false
			}
		}
		return true
	})
	return st.collected, st.skipped, st.failed
}

// processLink 处理单个详情链接：列表标题预查重 →（重试）抓取详情 → 入库。
// 返回 true 表示应提前结束任务
func (h *HeadlessSpider) processLink(browser *rod.Browser, t *spiderTask, link spiderLink, st *taskStats) bool {
	// 预查重：列表页标题可用时直接比对 slug，已存在则不打开详情页
	if link.Title != "" {
		slug := t.dedupSlug(link.URL, link.Title)
		if exists, err := service.Article.ExistsSlug(slug); err == nil && exists {
			st.skipped++
			st.consecExists++
			h.ctx.Log.Debug("文章已存在（预查），跳过", zap.String("url", link.URL))
			return h.hitStopWhenExists(t, st)
		}
	}
	article, err := h.fetchArticleRetry(browser, t, link.URL)
	if err != nil {
		st.failed++
		h.ctx.Log.Error("采集失败", zap.String("url", link.URL), zap.Error(err))
		return false
	}
	if article == nil { // 详情页判重已存在
		st.skipped++
		st.consecExists++
		return h.hitStopWhenExists(t, st)
	}
	return h.processArticle(t, article, st)
}

// processArticle 单篇入库（试运行只记录不入库；limit 达标返回 true 应结束任务）
func (h *HeadlessSpider) processArticle(t *spiderTask, article *entity.Article, st *taskStats) bool {
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
		return h.hitStopWhenExists(t, st)
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
	time.Sleep(time.Duration(h.Interval) * time.Second)
	return h.Limit > 0 && st.collected >= h.Limit
}

// hitStopWhenExists 连续已存在计数是否触发提前结束
func (h *HeadlessSpider) hitStopWhenExists(t *spiderTask, st *taskStats) bool {
	if t.StopWhenExists > 0 && st.consecExists >= t.StopWhenExists {
		h.ctx.Log.Info("连续多篇已存在，提前结束任务",
			zap.String("task", t.Name), zap.Int("consecutive", st.consecExists))
		return true
	}
	return false
}

// fetchArticleRetry 详情页抓取带重试（间隔逐次递增 1s/2s/...）
func (h *HeadlessSpider) fetchArticleRetry(browser *rod.Browser, t *spiderTask, link string) (*entity.Article, error) {
	var lastErr error
	for attempt := 0; attempt <= h.Retry; attempt++ {
		if attempt > 0 {
			h.ctx.Log.Warn("详情页采集失败，重试",
				zap.String("url", link), zap.Int("attempt", attempt), zap.Error(lastErr))
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		article, err := h.fetchArticle(browser, t, link)
		if err == nil {
			return article, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// eachListPage 逐列表页执行 visit 回调：
//   - page_url_pattern 模式（默认）：每页新开 page，打开失败记 failed 后继续下一页
//   - next_page_sel 模式：单 page 连续翻页（“下一页”缺失或翻页失败即结束）
//
// visit 返回 false 提前结束
func (h *HeadlessSpider) eachListPage(browser *rod.Browser, t *spiderTask, st *taskStats, visit func(page *rod.Page, base *url.URL, title string) bool) {
	if t.NextPageSel != "" {
		page, err := h.newPage(browser, t)
		if err != nil {
			st.failed++
			h.ctx.Log.Error("创建页面失败", zap.String("task", t.Name), zap.Error(err))
			return
		}
		defer page.MustClose()
		if err := h.openPage(page, t, h.pageURL(t, 1)); err != nil {
			st.failed++
			h.saveDebugShot(page, t, "list_error")
			h.ctx.Log.Error("列表页打开失败", zap.String("task", t.Name), zap.Error(err))
			return
		}
		for i := 1; i <= t.MaxPages; i++ {
			h.ctx.Log.Info("采集列表页", zap.String("task", t.Name), zap.Int("page", i))
			h.scrollPage(page, t.ScrollTimes)
			base, title := pageInfo(page, "")
			if !visit(page, base, title) {
				return
			}
			if !h.gotoListNext(page, t, base) {
				return
			}
		}
		return
	}
	for i := 1; i <= t.MaxPages; i++ {
		pageURL := h.pageURL(t, i)
		h.ctx.Log.Info("采集列表页", zap.String("task", t.Name), zap.Int("page", i), zap.String("url", pageURL))
		page, err := h.newPage(browser, t)
		if err != nil {
			st.failed++
			h.ctx.Log.Error("创建页面失败", zap.String("url", pageURL), zap.Error(err))
			continue
		}
		if err := h.openPage(page, t, pageURL); err != nil {
			st.failed++
			h.saveDebugShot(page, t, "list_error")
			h.ctx.Log.Error("列表页采集失败", zap.String("url", pageURL), zap.Error(err))
			page.MustClose()
			continue
		}
		h.scrollPage(page, t.ScrollTimes)
		base, title := pageInfo(page, pageURL)
		visit(page, base, title)
		page.MustClose()
	}
}

// openPage 导航并等待渲染就绪（wait_selector 或 DOM 稳定）
func (h *HeadlessSpider) openPage(page *rod.Page, t *spiderTask, pageURL string) error {
	if err := page.Navigate(pageURL); err != nil {
		return err
	}
	if err := page.WaitLoad(); err != nil {
		return err
	}
	if t.WaitSelector != "" {
		if _, err := page.Element(t.WaitSelector); err != nil {
			return fmt.Errorf("等待元素 %s 超时（渲染判据未出现）: %w", t.WaitSelector, err)
		}
	} else if err := page.WaitStable(time.Second * 2); err != nil {
		return err
	}
	return nil
}

// pageInfo 当前页信息：标题与用于相对地址解析的 baseURL
func pageInfo(page *rod.Page, fallbackURL string) (base *url.URL, title string) {
	baseURL := fallbackURL
	if info, err := page.Info(); err == nil && info != nil {
		baseURL = info.URL
		title = info.Title
	}
	base, perr := url.Parse(baseURL)
	if perr != nil || base == nil {
		base, _ = url.Parse(fallbackURL)
	}
	if base == nil {
		base = &url.URL{}
	}
	return base, title
}

// scrollPage 滚动到页底触发懒加载/“加载更多”（每次间隔约 1s）
func (h *HeadlessSpider) scrollPage(page *rod.Page, times int) {
	for i := 0; i < times; i++ {
		if _, err := page.Eval(`window.scrollTo(0, document.body.scrollHeight)`); err != nil {
			return
		}
		time.Sleep(time.Second)
	}
}

// gotoListNext 列表页翻到下一页（next_page_sel）：a[href] 直接导航，无 href 则点击按钮。
// 返回是否成功翻页（找不到“下一页”元素或翻页失败返回 false）
func (h *HeadlessSpider) gotoListNext(page *rod.Page, t *spiderTask, base *url.URL) bool {
	el, err := page.Element(t.NextPageSel)
	if err != nil {
		return false // 没有下一页了
	}
	hrefAttr := ""
	if a, e := el.Attribute("href"); e == nil && a != nil {
		hrefAttr = strings.TrimSpace(*a)
	}
	if hrefAttr != "" && !strings.HasPrefix(hrefAttr, "javascript") {
		if ref, perr := url.Parse(hrefAttr); perr == nil {
			next := hrefAttr
			if base != nil {
				next = base.ResolveReference(ref).String()
			}
			if nerr := h.openPage(page, t, next); nerr != nil {
				h.ctx.Log.Warn("列表翻页导航失败", zap.String("task", t.Name), zap.String("next", next), zap.Error(nerr))
				return false
			}
			return true
		}
	}
	// 无有效 href：点击按钮（JS 翻页）
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		h.ctx.Log.Warn("列表翻页点击失败", zap.String("task", t.Name), zap.Error(err))
		return false
	}
	_ = page.WaitStable(time.Second * 2)
	if t.WaitSelector != "" {
		_, _ = page.Element(t.WaitSelector)
	}
	return true
}

// spiderLink 列表页提取的详情链接及其标题（用于预查重，已存在则无需打开详情页）
type spiderLink struct {
	URL   string
	Title string
}

// linksOnPage 当前列表页提取详情链接：
// href 模式走属性提取通道（extractLinksByHref）；click 模式双通道（属性提取+点击拦截）合并去重
func (h *HeadlessSpider) linksOnPage(page *rod.Page, t *spiderTask, base *url.URL) ([]spiderLink, error) {
	if t.LinkMode != "click" {
		return h.extractLinksByHref(page, t, base)
	}
	// click 模式 = 属性提取 + 点击拦截 双通道合并去重（部分条目有真实链接、部分靠 JS 跳转的混合站也能全覆盖）
	hrefLinks, herr := h.extractLinksByHref(page, t, base)
	clickLinks, cerr := h.extractLinksByClick(page, t, base)
	if cerr != nil && len(hrefLinks) == 0 {
		return nil, cerr
	}
	if cerr != nil {
		h.ctx.Log.Warn("click 拦截失败，仅使用属性提取结果", zap.Error(cerr))
	}
	if herr != nil {
		h.ctx.Log.Debug("属性提取通道失败", zap.Error(herr))
	}
	seenURL := make(map[string]struct{}, len(hrefLinks))
	links := make([]spiderLink, 0, len(hrefLinks)+len(clickLinks))
	for _, l := range hrefLinks {
		seenURL[l.URL] = struct{}{}
		links = append(links, l)
	}
	for _, l := range clickLinks {
		if _, dup := seenURL[l.URL]; !dup {
			links = append(links, l)
		}
	}
	return links, nil
}

// stealthJS 最小隐身注入：遮 navigator.webdriver、伪造 plugins/languages
const stealthJS = `
	Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
	Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3, 4, 5] });
	Object.defineProperty(navigator, 'languages', { get: () => ['zh-CN', 'zh', 'en'] });
	window.chrome = window.chrome || { runtime: {} };
`

// launchBrowser 启动浏览器：优先本机 Chrome/Edge，可显式指定路径/代理。
// 目标站的 debugger 断点陷阱依赖 DevTools 附加，无头采集不附加 Debugger 域即天然免疫。
func (h *HeadlessSpider) launchBrowser() (*rod.Browser, error) {
	l := launcher.New().
		Headless(h.Headless).
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-infobars")
	if h.BrowserPath != "" {
		l = l.Bin(h.BrowserPath)
	}
	if h.Proxy != "" {
		l = l.Proxy(h.Proxy)
	}
	controlURL, err := l.Launch()
	if err != nil {
		return nil, err
	}
	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return nil, err
	}
	return browser, nil
}

// newPage 创建未导航页面，注入隐身脚本（须在 Navigate 之前注册）、按任务应用 UA/cookies 并挂超时
func (h *HeadlessSpider) newPage(browser *rod.Browser, t *spiderTask) (*rod.Page, error) {
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, err
	}
	_, _ = page.EvalOnNewDocument(stealthJS)
	if t != nil {
		if t.UserAgent != "" {
			_ = page.SetUserAgent(&proto.NetworkSetUserAgentOverride{UserAgent: t.UserAgent})
		}
		if cookies, cerr := parseCookies(t.Cookies, t.SourceURL); cerr != nil {
			h.ctx.Log.Warn("cookies 配置无效", zap.String("task", t.Name), zap.Error(cerr))
		} else if len(cookies) > 0 {
			if serr := page.SetCookies(cookies); serr != nil {
				h.ctx.Log.Warn("cookies 注入失败", zap.String("task", t.Name), zap.Error(serr))
			}
		}
	}
	return page.Timeout(time.Duration(h.Timeout) * time.Second), nil
}

func (h *HeadlessSpider) pageURL(t *spiderTask, page int) string {
	if page <= 1 || t.PageURLPattern == "" {
		return t.SourceURL
	}
	return strings.ReplaceAll(t.PageURLPattern, "{page}", fmt.Sprintf("%d", page))
}

// extractLinksByHref 属性取链通道：遍历 list_selector 匹配的条目元素，
// 按 link_selector（空=条目自身）定位链接元素，读 link_attr（默认 href）属性值，
// 经 link_extract_regex 抽取 / link_url_template 拼接得到详情 URL；
// 相对路径绝对化、include/exclude 过滤、去重；标题取条目文本（仅预查重用）
func (h *HeadlessSpider) extractLinksByHref(page *rod.Page, t *spiderTask, base *url.URL) ([]spiderLink, error) {
	els, err := page.Elements(t.ListSelector)
	if err != nil {
		return nil, fmt.Errorf("list_selector 无匹配: %w", err)
	}
	var links []spiderLink
	seen := map[string]struct{}{}
	for _, el := range els {
		_, raw := t.resolveLinkValue(el)
		if raw == "" || strings.HasPrefix(raw, "javascript:") {
			continue
		}
		ref, parseErr := url.Parse(raw)
		if parseErr != nil {
			continue
		}
		abs := base.ResolveReference(ref).String()
		abs = strings.SplitN(abs, "#", 2)[0]
		if abs == "" || abs == base.String() {
			continue
		}
		if !matchFilter(abs, t.LinkInclude, t.LinkExclude) {
			continue
		}
		if _, dup := seen[abs]; dup {
			continue
		}
		seen[abs] = struct{}{}
		links = append(links, spiderLink{URL: abs, Title: itemTitle(el, t.ListTitleSel)})
	}
	return links, nil
}

// resolveLinkValue 条目内定位链接元素并解析属性值：
// link_selector（空=条目自身）定位，link_attr（默认 href）取值，regex 抽取/模板拼接。
// 条目内无链接元素时返回 nil
func (t *spiderTask) resolveLinkValue(el *rod.Element) (*rod.Element, string) {
	linkEl := el
	if t.LinkSelector != "" {
		sub, subErr := el.Element(t.LinkSelector)
		if subErr != nil {
			return nil, "" // 条目内无链接元素
		}
		linkEl = sub
	}
	return linkEl, t.linkValue(linkEl)
}

// itemTitle 条目标题：list_title_sel 优先，回退 h1~h6，再回退条目文本首行
func itemTitle(el *rod.Element, titleSel string) string {
	if titleSel != "" {
		if n, err := el.Element(titleSel); err == nil {
			if txt, err := n.Text(); err == nil {
				if s := strings.TrimSpace(txt); s != "" {
					return s
				}
			}
		}
	}
	if h, err := el.Element("h1,h2,h3,h4,h5,h6"); err == nil {
		if txt, err := h.Text(); err == nil {
			if s := strings.TrimSpace(txt); s != "" {
				return s
			}
		}
	}
	if txt, err := el.Text(); err == nil {
		txt = strings.TrimSpace(txt)
		if i := strings.IndexAny(txt, "\n\r"); i > 0 {
			txt = txt[:i]
		}
		return strings.TrimSpace(txt)
	}
	return ""
}

// linkValue 从链接元素解析详情 URL：读 link_attr（默认 href）属性，
// 可选 link_extract_regex 抽取、link_url_template 拼接
func (t *spiderTask) linkValue(el *rod.Element) string {
	attr := t.LinkAttr
	if attr == "" {
		attr = "href"
	}
	v := ""
	if a, err := el.Attribute(attr); err == nil && a != nil {
		v = strings.TrimSpace(*a)
	}
	if v == "" {
		return ""
	}
	if t.LinkExtractRegex != "" {
		re, err := compileLinkRegex(t.LinkExtractRegex)
		if err != nil {
			return ""
		}
		m := re.FindStringSubmatch(v)
		if m == nil {
			return ""
		}
		if len(m) > 1 {
			v = m[1]
		} else {
			v = m[0]
		}
	}
	return buildLinkURL(t.LinkURLTemplate, v)
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

// compileLinkRegex 编译 link_extract_regex；兼容 /re/ 包裹写法（与 link_include 过滤约定一致）
func compileLinkRegex(pattern string) (*regexp.Regexp, error) {
	if len(pattern) > 2 && strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
		pattern = pattern[1 : len(pattern)-1]
	}
	return regexp.Compile(pattern)
}

// extractLinksByClick 点击拦截模式：劫持 window.open（只记录不真开新窗），对 list_selector
// 匹配的元素逐个派发 click（含元素内的 a/button，选择器填条目容器或可点元素均可），
// 按条目顺序捕获 Vue @click 等跳转处理器生成的详情 URL——适用于 href="javascript:void(0)" 的站点。
// 每个条目点击后等待 click_wait_ms（默认 250ms）再点下一个，捕获 URL 与条目按下标精确配对；
// 标题仅用于预查重优化，取不到时由详情页真实标题兜底。
// 注意：只可靠捕获 window.open 型跳转；处理器用 location.href 当前页跳转时页面会真导航、
// 本通道报错（有 href 链接时自动降级用属性提取结果）。
func (h *HeadlessSpider) extractLinksByClick(page *rod.Page, t *spiderTask, base *url.URL) ([]spiderLink, error) {
	js := `async (listSel, titleSel, perWaitMs) => {
		const sleep = ms => new Promise(r => setTimeout(r, ms));
		const hits = [];
		const origOpen = window.open;
		let cur = -1;
		window.open = function (u) { if (u) hits.push({ idx: cur, url: String(u) }); return null; };
		// 捕获阶段取消默认行为：防止条目内的真实 <a href> 被 el.click() 触发页面导航，
		// 导致后续点击与等待中断（Vue 的 @click 监听不受 preventDefault 影响）
		const stopNav = e => e.preventDefault();
		document.addEventListener('click', stopNav, true);
		const els = Array.from(document.querySelectorAll(listSel));
		const titles = els.map(el => {
			let n = titleSel ? el.querySelector(titleSel) : null;
			if (!n) n = el.querySelector('h1,h2,h3,h4,h5,h6');
			let text = n ? n.textContent : el.textContent;
			text = (text || '').trim();
			const i = text.search(/\n|\r/);
			return i > 0 ? text.slice(0, i).trim() : text;
		});
		for (let i = 0; i < els.length; i++) {
			cur = i;
			try { els[i].click(); } catch (e) {}
			els[i].querySelectorAll('a, button, [role="button"]').forEach(c => { try { c.click(); } catch (e) {} });
			await sleep(perWaitMs);
		}
		cur = -1;
		await sleep(400); // 收尾：等待最后一批异步跳转
		window.open = origOpen;
		document.removeEventListener('click', stopNav, true);
		return JSON.stringify({ hits, titles });
	}`
	res, err := page.Eval(js, t.ListSelector, t.ListTitleSel, t.ClickWaitMs)
	if err != nil {
		return nil, fmt.Errorf("click 模式执行失败: %w", err)
	}
	var payload struct {
		Hits []struct {
			Idx int    `json:"idx"`
			URL string `json:"url"`
		} `json:"hits"`
		Titles []string `json:"titles"`
	}
	if err := json.Unmarshal([]byte(res.Value.Str()), &payload); err != nil {
		return nil, fmt.Errorf("click 模式返回解析失败: %w", err)
	}

	var links []spiderLink
	crossDropped := 0
	seen := map[string]struct{}{}
	for _, hit := range payload.Hits {
		u := strings.TrimSpace(hit.URL)
		if u == "" {
			continue
		}
		ref, parseErr := url.Parse(u)
		if parseErr != nil {
			continue
		}
		// 同源校验：click 劫持捕获的跨域 URL 视为广告/统计弹窗丢弃（相对路径天然同源）
		if !t.LinkCrossOrigin && ref.Host != "" && !sameSite(ref.Host, base.Host) {
			crossDropped++
			h.ctx.Log.Debug("click 捕获跨域跳转，按广告丢弃", zap.String("url", u),
				zap.String("host", ref.Host), zap.String("site", base.Host))
			continue
		}
		abs := base.ResolveReference(ref).String()
		abs = strings.SplitN(abs, "#", 2)[0]
		if abs == "" || abs == base.String() {
			continue
		}
		if !matchFilter(abs, t.LinkInclude, t.LinkExclude) {
			continue
		}
		if _, dup := seen[abs]; dup {
			continue
		}
		seen[abs] = struct{}{}
		title := ""
		if hit.Idx >= 0 && hit.Idx < len(payload.Titles) {
			title = payload.Titles[hit.Idx]
		}
		links = append(links, spiderLink{URL: abs, Title: title})
	}
	if crossDropped > 0 {
		h.ctx.Log.Info("click 捕获跳转过滤完成", zap.String("task", t.Name),
			zap.Int("captured", len(payload.Hits)), zap.Int("cross_origin_dropped", crossDropped),
			zap.Int("kept", len(links)))
	}
	return links, nil
}

// extractListArticles mode=list：遍历列表条目直接构造文章（不进详情页）。
// 链接走 href 属性通道（link_selector/link_attr/regex/template 组合同样生效）；
// 标题 list_title_sel（回退 h1~h6/首行），封面 list_cover_sel，摘要 list_desc_sel（回退条目文本）
func (h *HeadlessSpider) extractListArticles(page *rod.Page, t *spiderTask, base *url.URL) []*entity.Article {
	els, err := page.Elements(t.ListSelector)
	if err != nil {
		h.ctx.Log.Warn("list_selector 无匹配", zap.String("task", t.Name), zap.Error(err))
		return nil
	}
	var articles []*entity.Article
	seen := map[string]struct{}{}
	now := time.Now().Unix()
	for _, el := range els {
		_, raw := t.resolveLinkValue(el)
		if raw == "" || strings.HasPrefix(raw, "javascript:") {
			continue
		}
		ref, parseErr := url.Parse(raw)
		if parseErr != nil {
			continue
		}
		abs := base.ResolveReference(ref).String()
		abs = strings.SplitN(abs, "#", 2)[0]
		if abs == "" || abs == base.String() {
			continue
		}
		if !matchFilter(abs, t.LinkInclude, t.LinkExclude) {
			continue
		}
		if _, dup := seen[abs]; dup {
			continue
		}
		seen[abs] = struct{}{}
		title := itemTitle(el, t.ListTitleSel)
		if title == "" {
			continue // 无标题无法成文
		}
		item := &entity.Article{
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
			if n, e := el.Element(t.ListCoverSel); e == nil {
				if v, e2 := n.Attribute(t.listCoverAttr()); e2 == nil && v != nil && strings.TrimSpace(*v) != "" {
					if ref2, perr2 := url.Parse(strings.TrimSpace(*v)); perr2 == nil {
						item.Thumbnail = base.ResolveReference(ref2).String()
					}
				}
			}
		}
		desc := ""
		if t.ListDescSel != "" {
			if n, e := el.Element(t.ListDescSel); e == nil {
				if txt, e2 := n.Text(); e2 == nil {
					desc = txt
				}
			}
		}
		if desc == "" {
			if txt, e := el.Text(); e == nil {
				desc = txt
			}
		}
		item.Description = plainSummary(desc, 120)
		articles = append(articles, item)
	}
	return articles
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

// sameSite 判断两个 host 是否同一站点：忽略 www. 前缀，允许子域互认
// （example.tv 与 www.example.tv / m.example.tv 视为同源）
func sameSite(a, b string) bool {
	a, b = strings.ToLower(strings.TrimPrefix(a, "www.")), strings.ToLower(strings.TrimPrefix(b, "www."))
	return a == b || strings.HasSuffix(a, "."+b) || strings.HasSuffix(b, "."+a)
}

// fetchArticle 打开详情页提取字段并构造文章（已存在则返回 nil）
func (h *HeadlessSpider) fetchArticle(browser *rod.Browser, t *spiderTask, link string) (article *entity.Article, err error) {
	page, err := h.newPage(browser, t)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			h.saveDebugShot(page, t, "detail_error")
		}
		page.MustClose()
	}()
	if err := page.Navigate(link); err != nil {
		return nil, err
	}
	if err := page.WaitLoad(); err != nil {
		return nil, err
	}
	if t.WaitSelector != "" {
		_, _ = page.Element(t.WaitSelector) // 详情页尽力等待，超时不致命
	} else {
		_ = page.WaitStable(time.Second * 2)
	}

	// 标题：选择器或回退 <title>
	title := ""
	if t.TitleSel != "" {
		if el, e := page.Element(t.TitleSel); e == nil {
			title = strings.TrimSpace(el.MustText())
		}
	}
	if title == "" {
		if info, e := page.Info(); e == nil && info.Title != "" {
			title = strings.TrimSpace(info.Title)
		}
	}
	if title == "" {
		err = fmt.Errorf("标题提取失败（核对 title_sel）")
		return nil, err
	}

	slug := t.dedupSlug(link, title)
	exists, err := service.Article.ExistsSlug(slug)
	if err != nil {
		return nil, err
	}
	if exists {
		h.ctx.Log.Debug("文章已存在，跳过", zap.String("slug", slug))
		return nil, nil
	}

	// 发布时间：publish_time_sel 优先（兼容 content 属性与文本），
	// 回退 meta[property=article:published_time]；解析失败用采集时刻
	createAt := time.Now().Unix()
	publishRaw := ""
	if t.PublishTimeSel != "" {
		if el, e := page.Element(t.PublishTimeSel); e == nil {
			publishRaw = elementValueAuto(el)
		}
	}
	if publishRaw == "" {
		if el, e := page.Element(`meta[property="article:published_time"]`); e == nil {
			if c, e2 := el.Attribute("content"); e2 == nil && c != nil {
				publishRaw = strings.TrimSpace(*c)
			}
		}
	}
	if v := parsePublishTime(publishRaw); v > 0 {
		createAt = v
	}

	item := &entity.Article{
		ArticleBase: entity.ArticleBase{
			Slug:        slug,
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
		if el, e := page.Element(t.KeywordsSel); e == nil {
			keywords = elementValueAuto(el)
		}
	}
	if keywords == "" {
		if el, e := page.Element(`meta[name="keywords"]`); e == nil {
			if c, e2 := el.Attribute("content"); e2 == nil && c != nil {
				keywords = strings.TrimSpace(*c)
			}
		}
	}
	item.Keywords = truncateRunes(keywords, 250)

	// 封面：优先选择器元素 src，回退 og:image meta content
	if t.CoverSel != "" {
		if el, e := page.Element(t.CoverSel); e == nil {
			if src, e2 := el.Attribute("src"); e2 == nil && src != nil {
				item.Thumbnail = *src
			}
		}
	}
	if item.Thumbnail == "" {
		if el, e := page.Element(`meta[property="og:image"]`); e == nil {
			if content, e2 := el.Attribute("content"); e2 == nil && content != nil {
				item.Thumbnail = *content
			}
		}
	}

	// 正文
	contentHTML := ""
	if t.ContentSel != "" {
		if el, e := page.Element(t.ContentSel); e == nil {
			contentHTML = el.MustHTML()
		}
	}
	// 正文翻页：存在“下一页”时逐页拼接（内容无新增即停，防“下一页”永在的死循环站）
	if t.ContentSel != "" && t.ContentNextSel != "" {
		maxPages := t.ContentMaxPages
		if maxPages <= 0 {
			maxPages = 20
		}
		for i := 1; i < maxPages; i++ {
			if !h.gotoDetailNext(page, t) {
				break
			}
			chunk := ""
			if el, e := page.Element(t.ContentSel); e == nil {
				chunk = el.MustHTML()
			}
			if chunk == "" || strings.HasSuffix(contentHTML, chunk) {
				break
			}
			contentHTML += "\n" + chunk
		}
	}
	item.Content = contentHTML
	item.Description = plainSummary(contentHTML, 120)

	// 类型专属数据：视频源（直链 embed=false + iframe 嵌入 embed=true）/ 图集
	// （写入 extends，对接 wolves 消费页；*_attr 支持懒加载 data-src）
	var extends vo.Extends
	if t.VideoSrcSel != "" || t.VideoIframeSel != "" {
		var labels []string
		if t.VideoLabelSel != "" {
			if els, e := page.Elements(t.VideoLabelSel); e == nil {
				for _, el := range els {
					labels = append(labels, elementValue(el, t.VideoLabelAttr))
				}
			}
		}
		direct, _ := h.extractAttrList(page, t.VideoSrcSel, t.videoAttr())
		embeds, _ := h.extractAttrList(page, t.VideoIframeSel, t.videoIframeAttr())
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
		if srcs, e := h.extractAttrList(page, t.GallerySel, t.galleryAttr()); e == nil && len(srcs) > 0 {
			extends = append(extends, vo.ExtendsItem{Key: "gallery_images", Value: srcs})
		}
	}

	// 通用 extends 键值提取（任意 key：作者/版本/语言/自定义...）
	for _, ex := range t.Extra {
		if ex.Key == "" || ex.Selector == "" {
			continue
		}
		if ex.Multiple {
			if vals, e := h.extractAttrList(page, ex.Selector, ex.Attr); e == nil && len(vals) > 0 {
				extends = append(extends, vo.ExtendsItem{Key: ex.Key, Value: vals})
			}
		} else if el, e := page.Element(ex.Selector); e == nil {
			if v := elementValue(el, ex.Attr); v != "" {
				extends = append(extends, vo.ExtendsItem{Key: ex.Key, Value: v})
			}
		}
	}
	item.Extends = extends
	return item, nil
}

// gotoDetailNext 详情页正文翻到下一页（content_next_sel）：a[href] 直接导航，无 href 则点击。
// 返回是否已翻到下一页（找不到元素/导航失败返回 false）
func (h *HeadlessSpider) gotoDetailNext(page *rod.Page, t *spiderTask) bool {
	el, err := page.Element(t.ContentNextSel)
	if err != nil {
		return false
	}
	hrefAttr := ""
	if a, e := el.Attribute("href"); e == nil && a != nil {
		hrefAttr = strings.TrimSpace(*a)
	}
	if hrefAttr != "" && !strings.HasPrefix(hrefAttr, "javascript") {
		if ref, perr := url.Parse(hrefAttr); perr == nil {
			next := hrefAttr
			if info, e := page.Info(); e == nil && info != nil {
				if b, berr := url.Parse(info.URL); berr == nil {
					next = b.ResolveReference(ref).String()
				}
			}
			if nerr := h.openPage(page, t, next); nerr != nil {
				h.ctx.Log.Warn("正文翻页导航失败", zap.String("next", next), zap.Error(nerr))
				return false
			}
			return true
		}
	}
	// JS 翻页：点击后尽力等待
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return false
	}
	_ = page.WaitStable(time.Second * 2)
	if t.WaitSelector != "" {
		_, _ = page.Element(t.WaitSelector)
	}
	return true
}

// elementValue 按规则取元素值：attr 为空=文本，"html"=innerHTML，否则取指定属性
func elementValue(el *rod.Element, attr string) string {
	switch {
	case attr == "":
		return strings.TrimSpace(el.MustText())
	case attr == "html":
		return el.MustHTML()
	default:
		if v, err := el.Attribute(attr); err == nil && v != nil {
			return *v
		}
	}
	return ""
}

// elementValueAuto 元素取值：优先 content 属性（meta 类元素），为空回退文本
func elementValueAuto(el *rod.Element) string {
	if v, err := el.Attribute("content"); err == nil && v != nil {
		if s := strings.TrimSpace(*v); s != "" {
			return s
		}
	}
	return strings.TrimSpace(el.MustText())
}

// extractAttrList 提取一组元素的指定值（多值聚合）
func (h *HeadlessSpider) extractAttrList(page *rod.Page, selector, attr string) ([]string, error) {
	els, err := page.Elements(selector)
	if err != nil {
		return nil, err
	}
	var vals []string
	for _, el := range els {
		if v := elementValue(el, attr); v != "" {
			vals = append(vals, v)
		}
	}
	return vals, nil
}

func (t *spiderTask) videoAttr() string {
	if t.VideoAttr != "" {
		return t.VideoAttr
	}
	return "src"
}

func (t *spiderTask) videoIframeAttr() string {
	if t.VideoIframeAttr != "" {
		return t.VideoIframeAttr
	}
	return "src"
}

func (t *spiderTask) galleryAttr() string {
	if t.GalleryAttr != "" {
		return t.GalleryAttr
	}
	return "src"
}

func (t *spiderTask) listCoverAttr() string {
	if t.ListCoverAttr != "" {
		return t.ListCoverAttr
	}
	return "src"
}

// hashSlug 以 源URL+标题 生成稳定去重 slug（多源防冲突）
func hashSlug(link, title string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(link) + "|" + strings.TrimSpace(title)))
	return hex.EncodeToString(sum[:8])
}

// dedupSlug 去重键：默认（含 url_title）= 源URL+标题 哈希；"url" = 仅源 URL 哈希
// （站点标题微调不会重复入库；注意切换 dedup_by 后既有文章会按新键被重新采集）
func (t *spiderTask) dedupSlug(link, title string) string {
	if t.DedupBy == "url" {
		return hashSlug(link, "")
	}
	return hashSlug(link, title)
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

// parseCookies 解析任务 cookies 配置：JSON 数组 [{"name":"..","value":"..","domain":"..","path":".."}]，
// 未指定 domain 时以 pageURL 绑定站点；空配置返回 nil
func parseCookies(raw, pageURL string) ([]*proto.NetworkCookieParam, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var items []struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Domain string `json:"domain"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("cookies JSON 解析失败: %w", err)
	}
	var out []*proto.NetworkCookieParam
	for _, it := range items {
		if it.Name == "" {
			return nil, fmt.Errorf("cookies 存在缺少 name 的条目")
		}
		c := &proto.NetworkCookieParam{Name: it.Name, Value: it.Value, URL: pageURL}
		if it.Domain != "" {
			c.Domain = it.Domain
		}
		if it.Path != "" {
			c.Path = it.Path
		}
		out = append(out, c)
	}
	return out, nil
}

// saveDebugShot 调试目录非空时保存当前页面截图（排查 selector/渲染问题的失败现场）
func (h *HeadlessSpider) saveDebugShot(page *rod.Page, t *spiderTask, tag string) {
	if h.DebugDir == "" || page == nil {
		return
	}
	data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
	if err != nil {
		return
	}
	name := fmt.Sprintf("%s_%s_%d.png", time.Now().Format("20060102_150405"), tag, time.Now().UnixNano()%1000000)
	if t != nil && t.Name != "" {
		name = fmt.Sprintf("%s_%s_%s_%d.png", time.Now().Format("20060102_150405"), sanitizeFilename(t.Name), tag, time.Now().UnixNano()%1000000)
	}
	if err := os.MkdirAll(h.DebugDir, 0o755); err != nil {
		return
	}
	path := filepath.Join(h.DebugDir, name)
	if err := os.WriteFile(path, data, 0o644); err == nil {
		h.ctx.Log.Warn("已保存失败现场截图", zap.String("path", path))
	}
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
