package plugins

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
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
	Tasks       string `json:"tasks"`        // 任务数组 JSON（见 README/About），多站多分类在此配置

	ctx *pluginEntity.Plugin
}

// spiderTask 单个采集任务
type spiderTask struct {
	Name           string `json:"name"`             // 任务名（日志区分）
	Enable         bool   `json:"enable"`           // 是否启用
	SourceURL      string `json:"source_url"`       // 起始页 URL
	PageURLPattern string `json:"page_url_pattern"` // 翻页模板，含 {page}；留空只采起始页
	MaxPages       int    `json:"max_pages"`        // 最多翻页数（默认 1）

	Mode         string `json:"mode"`          // detail=进详情页提取（默认）；list 预留未实现，当前行为恒同 detail
	WaitSelector string `json:"wait_selector"` // 渲染完成等待元素；留空等 DOM 稳定
	ListSelector string `json:"list_selector"` // 列表页详情链接选择器
	LinkInclude  string `json:"link_include"`  // 链接过滤：子串或 /正则/；留空不过滤
	LinkExclude  string `json:"link_exclude"`  // 链接排除：子串或 /正则/

	ListTitleSel string `json:"list_title_sel"` // 列表条目标题选择器（预查重用；回退 h1~h6/首行文本）

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

	StopWhenExists int `json:"stop_when_exists"` // 连续 N 篇已存在则提前结束任务（0=不启用）；定时增量采集推荐 3~5

	TitleSel    string `json:"title_sel"`     // 详情页标题选择器（回退 <title>）
	CoverSel    string `json:"cover_sel"`     // 封面选择器（取 src；回退 og:image）
	ContentSel  string `json:"content_sel"`   // 正文容器选择器（取 innerHTML）
	VideoSrcSel string `json:"video_src_sel"` // 播放源选择器（video/iframe src）→ extends[video_sources]
	GallerySel  string `json:"gallery_sel"`   // 图集图片选择器 → extends[gallery_images]

	Extra []spiderExtra `json:"extra"` // 通用 extends 键值提取（任意 key）

	ContentType string `json:"content_type"` // 入库类型：standard/novel/image/video
	CategoryID  int    `json:"category_id"`  // 入库分类 ID
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
		About: "通用无头采集插件（多任务）：tasks 配置 JSON 任务数组，" +
			"每任务独立选择器/翻页/链接过滤/extends 提取/入库类型；浏览器全局配置共享。支持定时增量采集（stop_when_exists）。" +
			"取链可组合：link_selector/link_attr/link_extract_regex/link_url_template（href、data-*、onclick 内嵌地址、data-id 拼URL）；" +
			"link_mode=click 模拟点击劫持 window.open 适配 javascript:void(0) 站（叠加属性提取、默认仅同源防广告）",
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

	h.ctx.Log.Info("开始无头采集", zap.Int("tasks", enabled))
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
			t.MaxPages = 1
		}
		if t.Mode == "" {
			t.Mode = "detail"
		}
		if t.SourceURL == "" || t.ListSelector == "" {
			return nil, fmt.Errorf("任务[%d]%s 缺少 source_url 或 list_selector", i+1, t.Name)
		}
		if t.LinkMode != "" && t.LinkMode != "href" && t.LinkMode != "click" {
			return nil, fmt.Errorf("任务[%d]%s link_mode 无效（仅支持 href/click）: %q", i+1, t.Name, t.LinkMode)
		}
		if t.LinkExtractRegex != "" {
			if _, err := compileLinkRegex(t.LinkExtractRegex); err != nil {
				return nil, fmt.Errorf("任务[%d]%s link_extract_regex 无效: %w", i+1, t.Name, err)
			}
		}
	}
	return tasks, nil
}

// spiderLink 列表页提取的详情链接及其标题（用于预查重，已存在则无需打开详情页）
type spiderLink struct {
	URL   string
	Title string
}

func (h *HeadlessSpider) runTask(browser *rod.Browser, t *spiderTask) (collected, skipped, failed int) {
	consecExists := 0
	for page := 1; page <= t.MaxPages; page++ {
		pageURL := h.pageURL(t, page)
		h.ctx.Log.Info("采集列表页", zap.String("task", t.Name), zap.Int("page", page), zap.String("url", pageURL))

		links, title, err := h.extractLinks(browser, t, pageURL)
		if err != nil {
			h.ctx.Log.Error("列表页采集失败", zap.String("url", pageURL), zap.Error(err))
			failed++
			continue
		}
		h.ctx.Log.Info("列表页渲染完成", zap.String("task", t.Name), zap.String("page_title", title), zap.Int("links", len(links)))
		if len(links) == 0 {
			h.ctx.Log.Warn("未提取到链接——请核对 list_selector/wait_selector/link_include 配置")
			break
		}

		for _, link := range links {
			// 预查重：列表页标题可用时直接比对 slug，已存在则不打开详情页
			if link.Title != "" {
				slug := hashSlug(link.URL, link.Title)
				if exists, err := service.Article.ExistsSlug(slug); err == nil && exists {
					skipped++
					consecExists++
					h.ctx.Log.Debug("文章已存在（预查），跳过", zap.String("url", link.URL))
					if t.StopWhenExists > 0 && consecExists >= t.StopWhenExists {
						h.ctx.Log.Info("连续多篇已存在，提前结束任务", zap.String("task", t.Name),
							zap.Int("consecutive", consecExists), zap.Int("page", page))
						return
					}
					continue
				}
			}
			article, err := h.fetchArticle(browser, t, link.URL)
			if err != nil {
				h.ctx.Log.Error("采集失败", zap.String("url", link.URL), zap.Error(err))
				failed++
				continue
			}
			if article == nil {
				skipped++
				consecExists++
				if t.StopWhenExists > 0 && consecExists >= t.StopWhenExists {
					h.ctx.Log.Info("连续多篇已存在，提前结束任务", zap.String("task", t.Name),
						zap.Int("consecutive", consecExists), zap.Int("page", page))
					return
				}
				continue
			}
			consecExists = 0
			if err := service.Article.Create(article); err != nil {
				h.ctx.Log.Error("创建文章失败", zap.String("title", article.Title), zap.Error(err))
				failed++
				continue
			}
			collected++
			h.ctx.Log.Info("成功采集", zap.String("task", t.Name), zap.String("title", article.Title), zap.String("slug", article.Slug))
			time.Sleep(time.Duration(h.Interval) * time.Second)
		}
	}
	return
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

// newPage 创建未导航页面，注入隐身脚本（须在 Navigate 之前注册）并挂超时
func (h *HeadlessSpider) newPage(browser *rod.Browser) (*rod.Page, error) {
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, err
	}
	_, _ = page.EvalOnNewDocument(stealthJS)
	return page.Timeout(time.Duration(h.Timeout) * time.Second), nil
}

func (h *HeadlessSpider) pageURL(t *spiderTask, page int) string {
	if page <= 1 || t.PageURLPattern == "" {
		return t.SourceURL
	}
	return strings.ReplaceAll(t.PageURLPattern, "{page}", fmt.Sprintf("%d", page))
}

// extractLinks 打开列表页，等待渲染后提取详情链接：
// href 模式走属性提取通道（extractLinksByHref）；click 模式双通道（属性提取+点击拦截）合并去重
func (h *HeadlessSpider) extractLinks(browser *rod.Browser, t *spiderTask, pageURL string) (links []spiderLink, pageTitle string, err error) {
	page, err := h.newPage(browser)
	if err != nil {
		return nil, "", err
	}
	defer page.MustClose()
	if err := page.Navigate(pageURL); err != nil {
		return nil, "", err
	}
	if err := page.WaitLoad(); err != nil {
		return nil, "", err
	}
	if t.WaitSelector != "" {
		if _, err := page.Element(t.WaitSelector); err != nil {
			return nil, "", fmt.Errorf("等待元素 %s 超时（渲染判据未出现）: %w", t.WaitSelector, err)
		}
	} else if err := page.WaitStable(time.Second * 2); err != nil {
		return nil, "", err
	}

	info, err := page.Info()
	if err == nil {
		pageTitle = info.Title
	}
	baseURL := pageURL
	if info != nil {
		baseURL = info.URL
	}
	base, parseErr := url.Parse(baseURL)
	if parseErr != nil {
		base, _ = url.Parse(pageURL)
	}

	if t.LinkMode == "click" {
		// click 模式 = 属性提取 + 点击拦截 双通道合并去重（部分条目有真实链接、部分靠 JS 跳转的混合站也能全覆盖）
		hrefLinks, herr := h.extractLinksByHref(page, t, base)
		clickLinks, cerr := h.extractLinksByClick(page, t, base)
		if cerr != nil && len(hrefLinks) == 0 {
			return nil, pageTitle, cerr
		}
		if cerr != nil {
			h.ctx.Log.Warn("click 拦截失败，仅使用属性提取结果", zap.Error(cerr))
		}
		if herr != nil {
			h.ctx.Log.Debug("属性提取通道失败", zap.Error(herr))
		}
		seenURL := make(map[string]struct{}, len(hrefLinks))
		for _, l := range hrefLinks {
			seenURL[l.URL] = struct{}{}
			links = append(links, l)
		}
		for _, l := range clickLinks {
			if _, dup := seenURL[l.URL]; !dup {
				links = append(links, l)
			}
		}
		return links, pageTitle, nil
	}

	links, err = h.extractLinksByHref(page, t, base)
	return links, pageTitle, err
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
		linkEl := el
		if t.LinkSelector != "" {
			sub, subErr := el.Element(t.LinkSelector)
			if subErr != nil {
				continue // 条目内无链接元素
			}
			linkEl = sub
		}
		raw := t.linkValue(linkEl)
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
// 匹配的每个元素派发 click（含元素内的 a/button，选择器填条目容器或可点元素均可），
// 捕获 Vue @click 等跳转处理器生成的详情 URL——适用于 href="javascript:void(0)" 的站点。
// 处理器同步或 800ms 内异步均能捕获；标题仅在捕获数与条目数一致时按下标配对（仅预查重优化，
// 不一致时留空，去重仍由详情页真实标题兜底）。
func (h *HeadlessSpider) extractLinksByClick(page *rod.Page, t *spiderTask, base *url.URL) ([]spiderLink, error) {
	js := `(listSel, titleSel) => {
		const captured = [];
		const origOpen = window.open;
		window.open = function (u) { if (u) captured.push(String(u)); return null; };
		// 捕获阶段取消默认行为：防止条目内的真实 <a href> 被 el.click() 触发页面导航，
		// 导致后续 setTimeout 不执行、结果 Promise 悬挂（Vue 的 @click 监听不受 preventDefault 影响）
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
		els.forEach(el => {
			el.click();
			el.querySelectorAll('a, button, [role="button"]').forEach(c => c.click());
		});
		return new Promise(resolve => setTimeout(() => {
			window.open = origOpen;
			document.removeEventListener('click', stopNav, true);
			resolve(JSON.stringify({ urls: captured, titles: titles }));
		}, 800));
	}`
	res, err := page.Eval(js, t.ListSelector, t.ListTitleSel)
	if err != nil {
		return nil, fmt.Errorf("click 模式执行失败: %w", err)
	}
	var payload struct {
		URLs   []string `json:"urls"`
		Titles []string `json:"titles"`
	}
	if err := json.Unmarshal([]byte(res.Value.Str()), &payload); err != nil {
		return nil, fmt.Errorf("click 模式返回解析失败: %w", err)
	}
	pairTitles := len(payload.Titles) == len(payload.URLs)

	var links []spiderLink
	crossDropped := 0
	seen := map[string]struct{}{}
	for i, u := range payload.URLs {
		u = strings.TrimSpace(u)
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
		if pairTitles {
			title = payload.Titles[i]
		}
		links = append(links, spiderLink{URL: abs, Title: title})
	}
	if crossDropped > 0 {
		h.ctx.Log.Info("click 捕获跳转过滤完成", zap.String("task", t.Name),
			zap.Int("captured", len(payload.URLs)), zap.Int("cross_origin_dropped", crossDropped),
			zap.Int("kept", len(links)))
	}
	return links, nil
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
func (h *HeadlessSpider) fetchArticle(browser *rod.Browser, t *spiderTask, link string) (*entity.Article, error) {
	page, err := h.newPage(browser)
	if err != nil {
		return nil, err
	}
	defer page.MustClose()
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
		if el, err := page.Element(t.TitleSel); err == nil {
			title = strings.TrimSpace(el.MustText())
		}
	}
	if title == "" {
		if info, err := page.Info(); err == nil && info.Title != "" {
			title = strings.TrimSpace(info.Title)
		}
	}
	if title == "" {
		return nil, fmt.Errorf("标题提取失败（核对 title_sel）")
	}

	slug := hashSlug(link, title)
	exists, err := service.Article.ExistsSlug(slug)
	if err != nil {
		return nil, err
	}
	if exists {
		h.ctx.Log.Debug("文章已存在，跳过", zap.String("slug", slug))
		return nil, nil
	}

	item := &entity.Article{
		ArticleBase: entity.ArticleBase{
			Slug:        slug,
			Title:       title,
			CategoryID:  t.CategoryID,
			Status:      true,
			CreateTime:  time.Now().Unix(),
			ContentType: t.ContentType,
		},
	}

	// 封面：优先选择器元素 src，回退 og:image meta content
	if t.CoverSel != "" {
		if el, err := page.Element(t.CoverSel); err == nil {
			if src, err := el.Attribute("src"); err == nil && src != nil {
				item.Thumbnail = *src
			}
		}
	}
	if item.Thumbnail == "" {
		if el, err := page.Element(`meta[property="og:image"]`); err == nil {
			if content, err := el.Attribute("content"); err == nil && content != nil {
				item.Thumbnail = *content
			}
		}
	}

	// 正文
	contentHTML := ""
	if t.ContentSel != "" {
		if el, err := page.Element(t.ContentSel); err == nil {
			contentHTML = el.MustHTML()
		}
	}
	item.Content = contentHTML
	item.Description = plainSummary(contentHTML, 120)

	// 类型专属数据：视频源 / 图集（写入 extends，对接 wolves 消费页）
	var extends vo.Extends
	if t.VideoSrcSel != "" {
		if srcs, err := h.extractAttrList(page, t.VideoSrcSel, "src"); err == nil && len(srcs) > 0 {
			sources := make([]map[string]any, 0, len(srcs))
			for i, src := range srcs {
				sources = append(sources, map[string]any{
					"label": fmt.Sprintf("第%02d集", i+1), "url": src, "embed": false,
				})
			}
			extends = append(extends, vo.ExtendsItem{Key: "video_sources", Value: sources})
		}
	}
	if t.GallerySel != "" {
		if srcs, err := h.extractAttrList(page, t.GallerySel, "src"); err == nil && len(srcs) > 0 {
			extends = append(extends, vo.ExtendsItem{Key: "gallery_images", Value: srcs})
		}
	}

	// 通用 extends 键值提取（任意 key：作者/版本/语言/自定义...）
	for _, ex := range t.Extra {
		if ex.Key == "" || ex.Selector == "" {
			continue
		}
		if ex.Multiple {
			if vals, err := h.extractAttrList(page, ex.Selector, ex.Attr); err == nil && len(vals) > 0 {
				extends = append(extends, vo.ExtendsItem{Key: ex.Key, Value: vals})
			}
		} else if el, err := page.Element(ex.Selector); err == nil {
			if v := elementValue(el, ex.Attr); v != "" {
				extends = append(extends, vo.ExtendsItem{Key: ex.Key, Value: v})
			}
		}
	}
	item.Extends = extends
	return item, nil
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

// hashSlug 以 源URL+标题 生成稳定去重 slug（多源防冲突）
func hashSlug(link, title string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(link) + "|" + strings.TrimSpace(title)))
	return hex.EncodeToString(sum[:8])
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
