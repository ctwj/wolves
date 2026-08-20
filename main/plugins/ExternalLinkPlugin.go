package plugins

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	pluginEntity "moss/domain/support/entity"
	"regexp"
	"strings"
	"time"
)

type ExternalLinkPlugin struct {
	Enable bool `json:"enable"` // 是否启用

	// 域名配置
	Domain string `json:"domain"` // 目标域名，用于替换****

	// URL 模板列表（每行一个，**** 会被替换为域名）
	URLTemplates string `json:"url_templates"` // URL 模板列表

	// 浏览器模拟参数
	UserAgent string `json:"user_agent"` // User-Agent
	Referer   string `json:"referer"`   // Referer（留空则自动生成）
	Cookies   string `json:"cookies"`   // Cookies (格式: name1=value1; name2=value2)

	// 请求配置
	Timeout int `json:"timeout"` // 请求超时时间（秒），默认 30
	Delay   int `json:"delay"`   // 请求间隔（毫秒），默认 1000

	ctx *pluginEntity.Plugin
}

func NewExternalLinkPlugin() *ExternalLinkPlugin {
	return &ExternalLinkPlugin{
		Enable: true,
		Domain: "",
		URLTemplates: `https://example1.com/search?q=****
https://example2.com/api?url=****
https://example3.com/track?target=****`,
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Referer:   "",
		Cookies:   "",
		Timeout:   30,
		Delay:     1000,
	}
}

// Info 返回插件信息
func (p *ExternalLinkPlugin) Info() *pluginEntity.PluginInfo {
	return &pluginEntity.PluginInfo{
		ID:        "ExternalLinkPlugin",
		About:     "外链插件：模拟浏览器请求外链地址，提升网站外部链接数量",
		RunEnable: true,
		CronEnable: true,
		NoOptions: false,
	}
}

// Load 插件加载
func (p *ExternalLinkPlugin) Load(ctx *pluginEntity.Plugin) error {
	p.ctx = ctx
	return nil
}

// Run 插件执行（定时任务或手动触发）
func (p *ExternalLinkPlugin) Run(ctx *pluginEntity.Plugin) error {
	p.ctx = ctx

	if !p.Enable {
		p.ctx.Log.Warn("ExternalLinkPlugin is disabled")
		return nil
	}

	// 验证配置
	if err := p.validateConfig(); err != nil {
		p.ctx.Log.Error("Configuration error", zap.Error(err))
		return err
	}

	p.ctx.Log.Info("ExternalLinkPlugin started",
		zap.String("domain", p.Domain),
		zap.Int("timeout", p.Timeout),
		zap.Int("delay", p.Delay),
	)

	// 解析 URL 模板
	urls, err := p.parseURLTemplates()
	if err != nil {
		p.ctx.Log.Error("Failed to parse URL templates", zap.Error(err))
		return err
	}

	if len(urls) == 0 {
		p.ctx.Log.Warn("No URLs to process")
		return nil
	}

	p.ctx.Log.Info("Processing external links", zap.Int("total", len(urls)))

	// 请求每个 URL
	successCount := 0
	failCount := 0

	for i, targetURL := range urls {
		p.ctx.Log.Info("Requesting URL",
			zap.Int("index", i+1),
			zap.Int("total", len(urls)),
			zap.String("url", targetURL))

		if err := p.requestURL(targetURL); err != nil {
			failCount++
			p.ctx.Log.Error("Request failed",
				zap.Int("index", i+1),
				zap.String("url", targetURL),
				zap.Error(err))
		} else {
			successCount++
			p.ctx.Log.Info("Request succeeded",
				zap.Int("index", i+1),
				zap.String("url", targetURL))
		}

		// 延迟，避免请求过快
		if i < len(urls)-1 && p.Delay > 0 {
			time.Sleep(time.Duration(p.Delay) * time.Millisecond)
		}
	}

	p.ctx.Log.Info("ExternalLinkPlugin completed",
		zap.Int("success", successCount),
		zap.Int("failed", failCount),
		zap.Int("total", len(urls)))

	return nil
}

// validateConfig 验证配置
func (p *ExternalLinkPlugin) validateConfig() error {
	if p.Domain == "" {
		return errors.New("domain is required")
	}

	// 验证域名格式
	if !isValidDomain(p.Domain) {
		return fmt.Errorf("invalid domain format: %s", p.Domain)
	}

	if p.URLTemplates == "" {
		return errors.New("url_templates is required")
	}

	if p.Timeout <= 0 {
		p.Timeout = 30
	}

	return nil
}

// isValidDomain 验证域名格式
func isValidDomain(domain string) bool {
	// 简单的域名验证
	if domain == "" {
		return false
	}

	// 去除协议前缀
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "www.")

	// 基本格式检查
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9](\.[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9])+$`)
	return domainRegex.MatchString(domain)
}

// parseURLTemplates 解析 URL 模板
func (p *ExternalLinkPlugin) parseURLTemplates() ([]string, error) {
	lines := strings.Split(p.URLTemplates, "\n")
	var urls []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 替换 **** 为域名
		targetURL := strings.ReplaceAll(line, "****", p.Domain)

		// 验证 URL 格式
		if _, err := url.Parse(targetURL); err != nil {
			p.ctx.Log.Warn("Invalid URL format, skipping",
				zap.String("template", line),
				zap.String("target", targetURL),
				zap.Error(err))
			continue
		}

		urls = append(urls, targetURL)
	}

	return urls, nil
}

// requestURL 请求 URL
func (p *ExternalLinkPlugin) requestURL(targetURL string) error {
	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: time.Duration(p.Timeout) * time.Second,
		// 禁止自动重定向
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 创建请求
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	// 设置 User-Agent
	if p.UserAgent != "" {
		req.Header.Set("User-Agent", p.UserAgent)
	}

	// 设置 Referer（优先使用配置的，否则智能生成）
	referer := p.Referer
	if referer == "" {
		referer = p.generateSmartReferer(targetURL)
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	// 设置 Cookies
	if p.Cookies != "" {
		req.Header.Set("Cookie", p.Cookies)
	}

	// 设置其他浏览器常见的头部
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体（确保连接完全关闭）
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	// 检查状态码
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP status code: %d", resp.StatusCode)
	}

	return nil
}

// generateSmartReferer 智能生成 Referer
func (p *ExternalLinkPlugin) generateSmartReferer(targetURL string) string {
	// 解析目标 URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		p.ctx.Log.Warn("Failed to parse URL for Referer generation", zap.String("url", targetURL), zap.Error(err))
		return ""
	}

	// 构造 Referer：协议 + 域名 + 端口（如果有）
	referer := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	// 如果端口不是默认端口，则包含端口
	if (parsedURL.Scheme == "http" && parsedURL.Port() != "80") ||
		(parsedURL.Scheme == "https" && parsedURL.Port() != "443") {
		if parsedURL.Port() != "" {
			referer = fmt.Sprintf("%s://%s:%s", parsedURL.Scheme, parsedURL.Hostname(), parsedURL.Port())
		}
	}

	p.ctx.Log.Debug("Generated smart Referer",
		zap.String("target_url", targetURL),
		zap.String("referer", referer))

	return referer
}

// GetTestURLs 获取测试 URL（用于测试配置）
func (p *ExternalLinkPlugin) GetTestURLs() ([]string, error) {
	if err := p.validateConfig(); err != nil {
		return nil, err
	}

	return p.parseURLTemplates()
}

// TestRequest 测试单个 URL 请求
func (p *ExternalLinkPlugin) TestRequest(targetURL string) (map[string]interface{}, error) {
	startTime := time.Now()

	err := p.requestURL(targetURL)
	duration := time.Since(startTime)

	result := map[string]interface{}{
		"url":      targetURL,
		"success":  err == nil,
		"duration": duration.String(),
	}

	if err != nil {
		result["error"] = err.Error()
	}

	return result, nil
}