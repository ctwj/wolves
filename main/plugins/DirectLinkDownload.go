package plugins

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime/multipart"
	"moss/domain/core/entity"
	"moss/domain/core/repository"
	"moss/domain/core/vo"
	pluginEntity "moss/domain/support/entity"
	"moss/infrastructure/utils/request"
	"moss/plugins/utils"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// downloadTask 下载任务
type downloadTask struct {
	TaskID    string               // 任务ID
	ArticleID int                  // 文章ID
	URL       string               // 直链URL
	FileName  string               // 文件名
	Result    chan *downloadResult // 结果通道
	Retries   int                  // 重试次数
	CreatedAt time.Time            // 创建时间
}

// downloadResult 下载结果
type downloadResult struct {
	SavedURL   string        // 上传后的URL
	Error      error         // 错误信息
	Completed  bool          // 是否完成
	Retried    int           // 重试次数
	FileName   string        // 文件名
	FileSize   int64         // 文件大小
}

// DirectLinkSavedItem 保存的直链记录（只保存 type 和上传后的新 url）
type DirectLinkSavedItem struct {
	Type string `json:"type"` // "直链"
	URL  string `json:"url"`  // 上传后的新链接
}

type DirectLinkDownload struct {
	// 基础配置
	ArticleID         int    `json:"article_id"`          // 指定文章ID，为0时处理所有文章
	AllowedExtensions string `json:"allowed_extensions"` // 逗号分隔，如 ".zip,.rar,.7z"
	MaxFileSizeMB     int    `json:"max_file_size_mb"`
	AllowedDomains    string `json:"allowed_domains"` // 逗号分隔

	// 下载配置
	DownRetry   int    `json:"down_retry"`
	DownProxy   string `json:"down_proxy"`
	DownTimeout int    `json:"down_timeout"`
	DownReferer string `json:"down_referer"` // 下载时使用的 referer 映射，每行 domain=referer，支持子域名匹配

	// 运行时字段（不持久化）
	ctx         *pluginEntity.Plugin
	rateLimiter *rate.Limiter
	refererMap  map[string]string // 解析后的域名-referer 映射

	// API 上传配置
	APIUploadURL        string `json:"api_upload_url"`       // 图床API地址
	APIFileField        string `json:"api_file_field"`       // 图床文件字段名
	APIHeaders          string `json:"api_headers"`          // 图床请求头(每行 key: value)
	APIFormData         string `json:"api_form_data"`        // 图床附加表单(每行 key=value)
	APIURLPath          string `json:"api_url_path"`         // 图床返回图片URL路径(如 data.url)
	APISuccessPath      string `json:"api_success_path"`     // 图床返回成功标识路径(可选)
	APISuccessValue     string `json:"api_success_value"`    // 图床返回成功标识值
	APITimeout          int    `json:"api_timeout"`          // 图床上传超时(秒)
	APIProxy            string `json:"api_proxy"`            // 图床上传代理
	APIRateLimitPerMin  int    `json:"api_rate_limit_per_minute"` // API每分钟调用限制
	APIMaxQueueSize     int    `json:"api_max_queue_size"`   // API上传队列最大长度
	APIQueueTimeout     int    `json:"api_queue_timeout"`    // 队列任务超时时间(秒)

	// 压缩包处理
	RePackage       bool   `json:"re_package"`
	DeleteFiles     string `json:"delete_files"`      // 逗号分隔，支持模糊匹配
	AddFiles        string `json:"add_files"`         // 逗号分隔（本地路径）
	FileNameReplace string `json:"file_name_replace"` // 每行 old=new
	ZipPassword     string `json:"zip_password"`      // 压缩包密码，为空则不加密
	NoEncryptFiles  string `json:"no_encrypt_files"`  // 不加密的文件列表，逗号分隔，支持模糊匹配

	// 运行时字段（不持久化）
	uploadQueue chan *downloadTask
	workerPool    *ants.PoolWithFunc
	queueCtx      context.Context
	queueCancel   context.CancelFunc
	wg            sync.WaitGroup
	uploadMutex   sync.Mutex
	uploadResults map[string]*downloadResult
	resultMutex   sync.Mutex
}

func NewDirectLinkDownload() *DirectLinkDownload {
	return &DirectLinkDownload{
		DownRetry:          3,
		MaxFileSizeMB:      100,
		DownTimeout:        60,
		APIFileField:       "file",
		APIURLPath:         "data.url",
		APITimeout:         30,
		APIRateLimitPerMin: 10,
		APIMaxQueueSize:    1000,
		RePackage:          false,
	}
}

func (p *DirectLinkDownload) Info() *pluginEntity.PluginInfo {
	return &pluginEntity.PluginInfo{
		ID:         "DirectLinkDownload",
		About:      "直链下载转存插件（支持定时任务和手动批量处理）",
		RunEnable:  true,
		CronEnable: true,
		PluginInfoPersistent: pluginEntity.PluginInfoPersistent{
			CronStart: false,
			CronExp:   "@every 24h",
		},
	}
}

func (p *DirectLinkDownload) Load(ctx *pluginEntity.Plugin) error {
	p.ctx = ctx

	// 如果数据库中的 CronExp 为空，设置默认值
	if ctx.Info.CronEnable && ctx.Info.CronExp == "" {
		ctx.Info.CronExp = "@every 24h"
	}

	// 调试：打印原始配置
	ctx.Log.Debug("加载插件配置",
		zap.String("down_referer", p.DownReferer),
	)

	// 解析 referer 映射
	p.parseRefererMap()

	// 初始化频率限制和队列系统
	if err := p.initRateLimiter(); err != nil {
		return fmt.Errorf("init rate limiter failed: %w", err)
	}

	if err := p.initUploadQueue(); err != nil {
		return fmt.Errorf("init upload queue failed: %w", err)
	}

	ctx.Log.Info("DirectLinkDownload plugin loaded",
		zap.String("allowed_extensions", p.AllowedExtensions),
		zap.Int("max_file_size_mb", p.MaxFileSizeMB),
		zap.Bool("re_package", p.RePackage),
		zap.String("down_referer", p.DownReferer),
		zap.Int("referer_map_count", len(p.refererMap)),
	)

	return nil
}

func (p *DirectLinkDownload) Run(ctx *pluginEntity.Plugin) error {
	if p.ctx == nil {
		p.ctx = ctx
	}

	// 统计信息
	processedCount := 0
	skippedCount := 0
	updatedCount := 0
	errorCount := 0

	// 如果指定了文章ID，只处理该文章
	if p.ArticleID > 0 {
		ctx.Log.Info("处理指定文章", zap.Int("article_id", p.ArticleID))

		article, err := repository.Article.Get(p.ArticleID)
		if err != nil {
			ctx.Log.Error("获取文章失败", zap.Int("id", p.ArticleID), zap.Error(err))
			return err
		}

		// 检查文章是否需要处理直链
		if !p.checkNeedsProcess(article) {
			ctx.Log.Info("文章无需处理直链", zap.Int("id", article.ID))
			return nil
		}

		// 处理文章直链
		if err := p.Save(article); err != nil {
			ctx.Log.Error("处理文章直链失败",
				zap.Int("id", article.ID),
				zap.String("title", article.Title),
				zap.Error(err),
			)
			return err
		}

		// 更新文章到数据库
		if err := repository.Article.Update(article); err != nil {
			ctx.Log.Error("更新文章失败",
				zap.Int("id", article.ID),
				zap.Error(err),
			)
			return err
		}

		ctx.Log.Info("文章直链处理成功", zap.Int("id", article.ID), zap.String("title", article.Title))
		return nil
	}

	// 未指定文章ID，批量处理所有文章
	ctx.Log.Info("开始批量处理直链下载...")

	// 查询最新的 10000 篇文章
	queryCtx := &repoContext{
		Limit: 10000,
		Order: "id desc",
	}

	articles, err := repository.Article.List(queryCtx)
	if err != nil {
		ctx.Log.Error("查询文章列表失败", zap.Error(err))
		return err
	}

	ctx.Log.Info("共查询到文章数量", zap.Int("count", len(articles)))

	// 遍历每篇文章
	for i, articleBase := range articles {
		// 每处理 10 篇文章输出一次进度
		if (i+1)%10 == 0 || i == len(articles)-1 {
			ctx.Log.Info("处理进度",
				zap.Int("processed", i+1),
				zap.Int("total", len(articles)),
				zap.Int("skipped", skippedCount),
				zap.Int("updated", updatedCount),
				zap.Int("error", errorCount),
			)
		}

		// 获取文章详情（包含内容）
		article, err := repository.Article.Get(articleBase.ID)
		if err != nil {
			ctx.Log.Error("获取文章详情失败",
				zap.Int("id", articleBase.ID),
				zap.String("title", articleBase.Title),
				zap.Error(err),
			)
			errorCount++
			continue
		}

		processedCount++

		// 检查文章是否需要处理直链
		needsProcess := p.checkNeedsProcess(article)

		if !needsProcess {
			skippedCount++
			continue
		}

		// 处理文章直链
		if err := p.Save(article); err != nil {
			ctx.Log.Error("处理文章直链失败",
				zap.Int("id", article.ID),
				zap.String("title", article.Title),
				zap.Error(err),
			)
			errorCount++
			continue
		}

		// 更新文章到数据库
		if err := repository.Article.Update(article); err != nil {
			ctx.Log.Error("更新文章失败",
				zap.Int("id", article.ID),
				zap.String("title", article.Title),
				zap.Error(err),
			)
			errorCount++
			continue
		}

		updatedCount++
		ctx.Log.Info("文章直链处理成功",
			zap.Int("id", article.ID),
			zap.String("title", article.Title),
		)
	}

	ctx.Log.Info("批量处理完成",
		zap.Int("processed", processedCount),
		zap.Int("skipped", skippedCount),
		zap.Int("updated", updatedCount),
		zap.Int("error", errorCount),
	)

	return nil
}

// checkNeedsProcess 检查文章是否需要处理直链
func (p *DirectLinkDownload) checkNeedsProcess(article *entity.Article) bool {
	// 解析 download_links
	directLinks := p.extractDirectLinks(article.Res)
	if len(directLinks) == 0 {
		return false
	}

	// 检查是否已保存
	saved := p.parseSavedLinks(article.Res)
	for _, link := range directLinks {
		if !p.isProcessed(link.URL, saved) {
			return true
		}
	}

	return false
}

// Save 处理单篇文章的直链
func (p *DirectLinkDownload) Save(article *entity.Article) error {
	// 解析 download_links
	directLinks := p.extractDirectLinks(article.Res)
	if len(directLinks) == 0 {
		return nil
	}

	// 解析已保存的记录
	saved := p.parseSavedLinks(article.Res)

	// 处理每个直链
	for _, link := range directLinks {
		// 检查是否已处理
		if p.isProcessed(link.URL, saved) {
			p.ctx.Log.Debug("直链已处理，跳过",
				zap.String("url", link.URL),
				zap.Int("article_id", article.ID),
			)
			continue
		}

		// 检查域名是否允许
		allowedDomains := utils.ParseAllowedDomains(p.AllowedDomains)
		if len(allowedDomains) > 0 && !utils.IsAllowedDomain(link.URL, allowedDomains) {
			p.ctx.Log.Info("域名不在允许列表，直接保存原始链接",
				zap.String("url", link.URL),
			)
			// 直接保存原始链接到saved中
			saved = p.updateSavedLinks(saved, &DirectLinkSavedItem{
				Type: "直链",
				URL:  link.URL,
			})
			continue
		}

		// 处理直链
		savedItem, err := p.processDirectLink(link)
		if err != nil {
			p.ctx.Log.Error("处理直链失败",
				zap.String("url", link.URL),
				zap.Error(err),
			)
			continue
		}

		// 更新保存记录
		saved = p.updateSavedLinks(saved, savedItem)
	}

	// 更新 article.Res
	p.updateArticleRes(article, saved)

	return nil
}

// extractDirectLinks 从 article.Res 中提取直链
func (p *DirectLinkDownload) extractDirectLinks(res vo.Extends) []DirectLinkSavedItem {
	var links []DirectLinkSavedItem

	for _, item := range res {
		if item.Key == "download_links" {
			if value, ok := item.Value.([]any); ok {
				for _, v := range value {
					if linkMap, ok := v.(map[string]any); ok {
						linkType, _ := linkMap["type"].(string)
						linkURL, _ := linkMap["url"].(string)

						// 只提取直链
						if linkType == "直链" && linkURL != "" {
							links = append(links, DirectLinkSavedItem{
								Type: "直链",
								URL:  linkURL,
							})
						}
					}
				}
			}
		}
	}

	return links
}

// parseSavedLinks 解析已保存的记录
func (p *DirectLinkDownload) parseSavedLinks(res vo.Extends) []DirectLinkSavedItem {
	var saved []DirectLinkSavedItem

	for _, item := range res {
		if item.Key == "saved" {
			if value, ok := item.Value.([]any); ok {
				for _, v := range value {
					if savedMap, ok := v.(map[string]any); ok {
						savedType, _ := savedMap["type"].(string)
						if savedType == "直链" {
							savedItem := DirectLinkSavedItem{
								Type: savedType,
								URL:  toString(savedMap["url"]),
							}
							saved = append(saved, savedItem)
						}
					}
				}
			}
		}
	}

	return saved
}

// isProcessed 检查直链是否已处理（saved 中存在 type="直链" 的记录即表示已处理成功）
func (p *DirectLinkDownload) isProcessed(url string, saved []DirectLinkSavedItem) bool {
	for _, item := range saved {
		if item.Type == "直链" {
			return true
		}
	}
	return false
}

// updateSavedLinks 更新保存记录（添加或更新），返回更新后的切片
func (p *DirectLinkDownload) updateSavedLinks(saved []DirectLinkSavedItem, newItem *DirectLinkSavedItem) []DirectLinkSavedItem {
	// 查找是否已存在 type="直链" 的记录
	for i, item := range saved {
		if item.Type == "直链" {
			saved[i] = *newItem
			return saved
		}
	}
	// 不存在则添加
	return append(saved, *newItem)
}

// updateArticleRes 更新 article.Res
func (p *DirectLinkDownload) updateArticleRes(article *entity.Article, saved []DirectLinkSavedItem) {
	// 转换为 []any 格式（只保存 type 和 url）
	var savedAny []any
	for _, item := range saved {
		savedAny = append(savedAny, map[string]any{
			"type": item.Type,
			"url":  item.URL,
		})
	}

	// 查找或创建 saved 字段
	found := false
	for i, item := range article.Res {
		if item.Key == "saved" {
			article.Res[i].Value = savedAny
			found = true
			break
		}
	}

	if !found {
		article.Res = append(article.Res, vo.ExtendsItem{
			Key:   "saved",
			Value: savedAny,
		})
	}
}

// validateFile 验证文件是否允许下载
func (p *DirectLinkDownload) validateFile(urlStr string, size int64) error {
	// 从 URL 中提取文件名
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	filename := parsedURL.Path
	if idx := strings.LastIndex(filename, "/"); idx != -1 {
		filename = filename[idx+1:]
	}

	// 检查文件后缀
	allowedExtensions := utils.ParseFileExtensions(p.AllowedExtensions)
	if len(allowedExtensions) > 0 {
		if !utils.IsAllowedExtension(filename, allowedExtensions) {
			return fmt.Errorf("file extension not allowed: %s", filename)
		}
	}

	// 检查文件大小
	if p.MaxFileSizeMB > 0 && size > int64(p.MaxFileSizeMB)*1024*1024 {
		return fmt.Errorf("file size %d bytes exceeds limit %d MB", size, p.MaxFileSizeMB)
	}

	// 检查域名
	allowedDomains := utils.ParseAllowedDomains(p.AllowedDomains)
	if len(allowedDomains) > 0 {
		if !utils.IsAllowedDomain(urlStr, allowedDomains) {
			return fmt.Errorf("domain not allowed: %s", urlStr)
		}
	}

	return nil
}

// parseRefererMap 解析 referer 映射配置
// 配置格式：每行 domain=referer，支持子域名匹配
// 例如：itmopcdn.com=https://www.itmop.com
func (p *DirectLinkDownload) parseRefererMap() {
	p.refererMap = make(map[string]string)
	if p.DownReferer == "" {
		return
	}

	for _, line := range strings.Split(p.DownReferer, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			domain := strings.TrimSpace(parts[0])
			referer := strings.TrimSpace(parts[1])
			if domain != "" && referer != "" {
				p.refererMap[domain] = referer
			}
		}
	}
}

// getRefererForURL 根据 URL 域名获取对应的 referer
// 匹配规则：支持子域名匹配，如配置 itmopcdn.com 可匹配 down3.itmopcdn.com
func (p *DirectLinkDownload) getRefererForURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		p.ctx.Log.Warn("解析URL失败", zap.String("url", rawURL), zap.Error(err))
		return ""
	}

	host := parsed.Host
	// 移除端口号
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	p.ctx.Log.Debug("获取referer",
		zap.String("url", rawURL),
		zap.String("host", host),
		zap.Any("referer_map", p.refererMap),
	)

	// 完全匹配
	if referer, ok := p.refererMap[host]; ok {
		p.ctx.Log.Info("referer完全匹配", zap.String("host", host), zap.String("referer", referer))
		return referer
	}

	// 子域名匹配：从后往前匹配域名后缀
	// 例如：down3.itmopcdn.com 匹配 itmopcdn.com
	parts := strings.Split(host, ".")
	for i := 1; i < len(parts); i++ {
		domain := strings.Join(parts[i:], ".")
		if referer, ok := p.refererMap[domain]; ok {
			p.ctx.Log.Info("referer子域名匹配",
				zap.String("host", host),
				zap.String("matched_domain", domain),
				zap.String("referer", referer))
			return referer
		}
	}

	// 未匹配到，从 URL 自动提取
	referer := p.extractRefererFromURL(rawURL)
	p.ctx.Log.Warn("referer未匹配到配置，使用URL自动提取",
		zap.String("host", host),
		zap.String("referer", referer))
	return referer
}

// extractRefererFromURL 从 URL 中提取 referer
func (p *DirectLinkDownload) extractRefererFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	// 提取协议和域名作为 referer
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
}

// processDirectLink 处理单个直链
func (p *DirectLinkDownload) processDirectLink(link DirectLinkSavedItem) (*DirectLinkSavedItem, error) {
	p.ctx.Log.Info("开始处理直链",
		zap.String("url", link.URL),
	)

	// 根据 URL 域名获取对应的 referer
	referer := p.getRefererForURL(link.URL)

	// 1. 获取文件大小
	fileSize, err := p.getFileSize(link.URL, referer)
	if err != nil {
		p.ctx.Log.Warn("获取文件大小失败，尝试下载",
			zap.String("url", link.URL),
			zap.Error(err))
		// 如果获取大小失败，继续下载流程
		fileSize = 0
	}

	// 2. 验证文件
	if err := p.validateFile(link.URL, fileSize); err != nil {
		p.ctx.Log.Warn("文件验证失败",
			zap.String("url", link.URL),
			zap.Error(err))
		return nil, err
	}

	// 3. 下载文件
	data, err := p.down(link.URL, referer)
	if err != nil {
		p.ctx.Log.Error("下载文件失败",
			zap.String("url", link.URL),
			zap.Error(err))
		return nil, err
	}

	// 更新实际文件大小
	fileSize = int64(len(data))

	// 检查下载的内容是否是 HTML（可能是错误页面）
	contentType := http.DetectContentType(data)
	p.ctx.Log.Info("下载文件类型检测",
		zap.String("url", link.URL),
		zap.String("content_type", contentType),
		zap.Int("size", len(data)))

	if strings.HasPrefix(contentType, "text/html") {
		// 如果返回的是 HTML，可能是防盗链错误页面
		p.ctx.Log.Error("下载返回HTML页面，可能是防盗链或referer错误",
			zap.String("url", link.URL),
			zap.String("referer", referer),
			zap.String("preview", string(data[:minInt(len(data), 500)])))
		return nil, fmt.Errorf("download returned HTML page, possible referer error")
	}

	// 4. 从 URL 中提取文件名
	parsedURL, err := url.Parse(link.URL)
	if err != nil {
		p.ctx.Log.Error("解析URL失败", zap.String("url", link.URL), zap.Error(err))
		return nil, err
	}
	urlFilename := parsedURL.Path
	if idx := strings.LastIndex(urlFilename, "/"); idx != -1 {
		urlFilename = urlFilename[idx+1:]
	}

	filename := utils.GetFileNameWithoutExt(urlFilename)
	if filename == "" {
		filename = fmt.Sprintf("file_%d", time.Now().Unix())
	}

	// 应用文件名替换规则
	if p.FileNameReplace != "" {
		rules := strings.Split(p.FileNameReplace, "\n")
		filename = utils.ApplyFileNameRules(filename, rules)
	}

	filename = utils.SanitizeFileName(filename)
	ext := utils.GetFileExtension(urlFilename)
	fullName := filename + ext

	// 5. 压缩包处理
	if p.RePackage && utils.IsZipFile(data) {
		p.ctx.Log.Info("开始重新打包压缩包",
			zap.String("filename", fullName),
		)

		deleteFiles := strings.Split(p.DeleteFiles, ",")
		addFiles := strings.Split(p.AddFiles, ",")
		renameRules := strings.Split(p.FileNameReplace, "\n")
		noEncryptFiles := strings.Split(p.NoEncryptFiles, ",")

		// 记录删除规则
		p.ctx.Log.Debug("压缩包重新打包配置",
			zap.Strings("delete_files", deleteFiles),
			zap.Strings("add_files", addFiles),
			zap.Bool("has_password", p.ZipPassword != ""),
			zap.Strings("no_encrypt_files", noEncryptFiles),
		)

		result, err := utils.RepackageZip(data, deleteFiles, addFiles, renameRules, p.ZipPassword, noEncryptFiles)
		if err != nil {
			p.ctx.Log.Warn("重新打包失败，使用原始文件",
				zap.String("filename", fullName),
				zap.Error(err))
		} else {
			originalSize := len(data)
			data = result.Data
			fileSize = int64(len(data))
			
			// 记录详细结果
			logFields := []zap.Field{
				zap.String("filename", fullName),
				zap.Int("original_size", originalSize),
				zap.Int("repackaged_size", len(result.Data)),
				zap.Int("deleted_count", result.DeletedCount),
				zap.Int("kept_count", result.KeptCount),
				zap.Int("added_count", result.AddedCount),
				zap.Bool("encrypted", p.ZipPassword != ""),
			}
			
			if len(result.AddErrors) > 0 {
				logFields = append(logFields, zap.Strings("add_errors", result.AddErrors))
			}
			
			p.ctx.Log.Info("压缩包重新打包成功", logFields...)
		}
	}

	// 6. 上传到 API
	savedURL, err := p.uploadByAPI(fullName, ext, data)
	if err != nil {
		p.ctx.Log.Error("上传文件失败",
			zap.String("filename", fullName),
			zap.Error(err))
		return nil, err
	}

	p.ctx.Log.Info("直链处理成功",
		zap.String("url", link.URL),
		zap.String("saved_url", savedURL),
		zap.String("filename", fullName),
		zap.Int64("file_size", fileSize),
	)

	return &DirectLinkSavedItem{
		Type: "直链",
		URL:  savedURL, // 保存上传后的新链接
	}, nil
}

// 初始化频率限制器
func (p *DirectLinkDownload) initRateLimiter() error {
	limit := p.APIRateLimitPerMin
	if limit <= 0 {
		limit = 10 // 默认每分钟10次
	}

	p.rateLimiter = rate.NewLimiter(rate.Limit(limit)/60, limit)
	p.ctx.Log.Info("rate limiter initialized", zap.Int("limit_per_minute", limit))
	return nil
}

// 初始化上传队列
func (p *DirectLinkDownload) initUploadQueue() error {
	queueSize := p.APIMaxQueueSize
	if queueSize <= 0 {
		queueSize = 1000 // 默认队列长度1000
	}

	p.uploadQueue = make(chan *downloadTask, queueSize)
	p.uploadResults = make(map[string]*downloadResult)

	p.queueCtx, p.queueCancel = context.WithCancel(context.Background())

	poolSize := 5 // 默认5个工作协程
	pool, err := ants.NewPoolWithFunc(poolSize, p.processUploadTask, ants.WithNonblocking(true))
	if err != nil {
		return fmt.Errorf("create worker pool failed: %w", err)
	}
	p.workerPool = pool

	p.wg.Add(1)
	go p.queueProcessor()

	p.ctx.Log.Info("upload queue initialized",
		zap.Int("queue_size", queueSize),
		zap.Int("worker_count", poolSize))
	return nil
}

// queueProcessor 队列处理器
func (p *DirectLinkDownload) queueProcessor() {
	defer p.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.queueCtx.Done():
			p.ctx.Log.Info("queue processor stopped")
			return
		case <-ticker.C:
			p.processQueueItems()
		}
	}
}

// processQueueItems 处理队列中的任务
func (p *DirectLinkDownload) processQueueItems() {
	for {
		select {
		case task := <-p.uploadQueue:
			if p.rateLimiter.Allow() {
				err := p.workerPool.Invoke(task)
				if err != nil {
					p.ctx.Log.Error("failed to submit upload task to worker pool",
						zap.String("task_id", task.TaskID),
						zap.Error(err))
					task.Result <- &downloadResult{
						Error:     fmt.Errorf("failed to submit task: %w", err),
						Completed: true,
					}
				}
			} else {
				select {
				case p.uploadQueue <- task:
				default:
					p.ctx.Log.Warn("upload queue is full, task rejected",
						zap.String("task_id", task.TaskID))
					task.Result <- &downloadResult{
						Error:     fmt.Errorf("upload queue is full"),
						Completed: true,
					}
				}
				break
			}
		default:
			return
		}
	}
}

// processUploadTask 处理单个上传任务
func (p *DirectLinkDownload) processUploadTask(taskData interface{}) {
	task, ok := taskData.(*downloadTask)
	if !ok {
		p.ctx.Log.Error("invalid task type", zap.Any("task_data", taskData))
		return
	}

	// 等待频率限制
	if err := p.rateLimiter.Wait(context.Background()); err != nil {
		p.ctx.Log.Error("rate limiter wait failed",
			zap.String("task_id", task.TaskID),
			zap.Error(err))
		task.Result <- &downloadResult{
			Error:     fmt.Errorf("rate limiter wait failed: %w", err),
			Completed: true,
		}
		return
	}

	// TODO: 执行上传
	// 这个方法将在后续步骤中实现
	task.Result <- &downloadResult{
		Error:     fmt.Errorf("not implemented yet"),
		Completed: true,
	}
}

// 辅助函数
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	default:
		return fmt.Sprint(val)
	}
}

// createHTTPClient 创建 HTTP 客户端
func (p *DirectLinkDownload) createHTTPClient(proxy string, timeout int) *http.Client {
	if timeout <= 0 {
		timeout = 60
	}

	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	if proxy != "" {
		if proxyURL, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return &http.Client{
		Timeout:   time.Duration(timeout) * time.Second,
		Transport: transport,
	}
}

// getFileSize 获取文件大小（通过 HEAD 请求）
func (p *DirectLinkDownload) getFileSize(urlStr string, referer string) (int64, error) {
	client := p.createHTTPClient(p.DownProxy, p.DownTimeout)

	req, err := http.NewRequest("HEAD", urlStr, nil)
	if err != nil {
		return 0, err
	}

	// 设置 referer
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return 0, fmt.Errorf("Content-Length not found")
	}

	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return 0, err
	}

	return size, nil
}

// down 下载文件
func (p *DirectLinkDownload) down(url string, referer string) ([]byte, error) {
	p.ctx.Log.Info("开始下载文件",
		zap.String("url", url),
		zap.String("referer", referer))

	file, err := request.New().
		SetRetry(p.DownRetry).
		SetProxyURLStr(p.DownProxy).
		SetReferer(referer).
		GetBody(url)
	if err != nil {
		p.ctx.Log.Warn("down file error",
			zap.String("url", url),
			zap.Error(err))
		return nil, err
	}

	p.ctx.Log.Info("下载文件成功",
		zap.String("url", url),
		zap.Int("size", len(file)))
	return file, nil
}

// uploadByAPI 通过 API 上传文件
func (p *DirectLinkDownload) uploadByAPI(name, ext string, file []byte) (string, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	// 添加文件字段
	part, err := writer.CreateFormFile(p.APIFileField, name)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, bytes.NewReader(file)); err != nil {
		return "", err
	}

	// 添加其他表单字段（APIFormData）
	if p.APIFormData != "" {
		for _, line := range strings.Split(p.APIFormData, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				writer.WriteField(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}
		}
	}

	// 结束 multipart 写入
	if err := writer.Close(); err != nil {
		return "", err
	}

	// 创建请求
	req, err := http.NewRequest("POST", p.APIUploadURL, body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "moss-direct-link-download/1.0")

	// 添加自定义请求头（APIHeaders）
	if p.APIHeaders != "" {
		for _, line := range strings.Split(p.APIHeaders, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}
		}
	}

	// 记录请求详情
	p.ctx.Log.Info("API上传请求详情",
		zap.String("url", p.APIUploadURL),
		zap.String("method", "POST"),
		zap.String("content_type", writer.FormDataContentType()),
		zap.Int("body_size", body.Len()),
		zap.String("file_name", name),
		zap.Int("file_size", len(file)),
		zap.String("file_field", p.APIFileField),
		zap.Any("headers", req.Header),
	)

	// 发送请求
	client := p.createHTTPClient(p.APIProxy, p.APITimeout)
	resp, err := client.Do(req)
	if err != nil {
		p.ctx.Log.Error("API上传请求失败", zap.Error(err))
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 记录响应详情
	p.ctx.Log.Info("API上传响应详情",
		zap.Int("status_code", resp.StatusCode),
		zap.Any("response_headers", resp.Header),
		zap.String("response_body", string(respBody[:minInt(len(respBody), 500)])),
	)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("api upload status %d: %s", resp.StatusCode, string(respBody[:minInt(len(respBody), 180)]))
	}

	// 解析响应
	js, err := simplejson.NewJson(respBody)
	if err != nil {
		return "", err
	}

	// 检查成功标识（APISuccessPath、APISuccessValue）
	if strings.TrimSpace(p.APISuccessPath) != "" {
		successVal := p.jsonPathString(js, p.APISuccessPath)
		if !strings.EqualFold(strings.TrimSpace(successVal), strings.TrimSpace(p.APISuccessValue)) {
			return "", fmt.Errorf("api upload success check failed, path=%s value=%s", p.APISuccessPath, successVal)
		}
	}

	// 提取 URL（APIURLPath）
	urlPath := strings.TrimSpace(p.APIURLPath)
	if urlPath == "" {
		urlPath = "data.url"
	}
	uploadURL := strings.TrimSpace(p.jsonPathString(js, urlPath))
	if uploadURL == "" {
		return "", fmt.Errorf("api upload url not found at path=%s", urlPath)
	}

	// 处理以 // 开头的 URL
	if strings.HasPrefix(uploadURL, "//") {
		uploadURL = "https:" + uploadURL
	}

	return uploadURL, nil
}

// jsonPathString 从 JSON 中提取字符串值
func (p *DirectLinkDownload) jsonPathString(js *simplejson.Json, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	node := js.GetPath(strings.Split(path, ".")...)
	if node == nil {
		return ""
	}
	val := node.Interface()
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprint(v)
	}
}