package plugins

import (
	"bytes"
	"encoding/json"
	"errors"
	"go.uber.org/zap"
	"moss/domain/config"
	"moss/domain/core/entity"
	"moss/domain/core/repository/context"
	"moss/domain/core/service"
	pluginEntity "moss/domain/support/entity"
	"moss/infrastructure/utils/request"
	"strings"
	"time"
)

type PushToSearchEngine struct {
	// 百度推送配置
	BaiduEnable         bool   `json:"baidu_enable"`          // 百度总开关
	BaiduEnableOnCreate bool   `json:"baidu_enable_on_create"` // 创建时推送
	BaiduEnableOnUpdate bool   `json:"baidu_enable_on_update"` // 更新时推送
	BaiduApiURL         string `json:"baidu_api_url"`          // 百度 API 地址
	BaiduCronWithinHours int   `json:"baidu_cron_within_hours"` // 定时处理过去N小时文章

	// Bing 推送配置
	BingEnable         bool   `json:"bing_enable"`           // Bing 总开关
	BingEnableOnCreate bool   `json:"bing_enable_on_create"` // 创建时推送
	BingEnableOnUpdate bool   `json:"bing_enable_on_update"` // 更新时推送
	BingApiKey         string `json:"bing_api_key"`          // Bing API Key
	BingCronWithinHours int   `json:"bing_cron_within_hours"` // 定时处理过去N小时文章

	ctx *pluginEntity.Plugin
}

func NewPushToSearchEngine() *PushToSearchEngine {
	return &PushToSearchEngine{
		BaiduCronWithinHours: 1, // 默认处理过去1小时内的文章
		BingCronWithinHours:  1,
	}
}

func (p *PushToSearchEngine) Info() *pluginEntity.PluginInfo {
	return &pluginEntity.PluginInfo{
		ID:         "PushToSearchEngine",
		About:      "push article to baidu and bing search engine when created or updated or via cron",
		RunEnable:  true,
		CronEnable: true,
		PluginInfoPersistent: pluginEntity.PluginInfoPersistent{
			CronStart: false,
			CronExp:   "@every 1h",
		},
	}
}

func (p *PushToSearchEngine) Run(ctx *pluginEntity.Plugin) error {
	p.ctx = ctx

	// 百度推送定时任务
	if p.BaiduEnable {
		p.runBaiduPush()
	}

	// Bing 推送定时任务
	if p.BingEnable {
		p.runBingPush()
	}

	return nil
}

func (p *PushToSearchEngine) runBaiduPush() {
	withinHours := p.BaiduCronWithinHours
	if withinHours <= 0 {
		withinHours = 1
	}
	p.ctx.Log.Info("baidu cron task started", zap.Int("within_hours", withinHours))

	withinTime := time.Now().Add(-time.Duration(withinHours) * time.Hour).Unix()
	articles, err := service.Article.ListAfterCreateTime(&context.Context{
		Select: "id, slug, status",
		Where: &context.Where{
			Field:    "status",
			Operator: context.WhereOperatorEqualTrue,
		},
	}, withinTime)
	if err != nil {
		p.ctx.Log.Error("baidu query articles error", zap.Error(err))
		return
	}

	if len(articles) == 0 {
		p.ctx.Log.Info("baidu no articles to push", zap.Int("within_hours", withinHours))
		return
	}

	// 收集URL
	var urls []string
	for _, article := range articles {
		urls = append(urls, article.FullURL())
	}

	p.ctx.Log.Info("baidu found articles to push", zap.Int("count", len(urls)), zap.Int("within_hours", withinHours))

	// 批量推送
	res, err := p.PushURLToBaidu(urls...)
	if err != nil {
		p.ctx.Log.Error("baidu cron push error", zap.Error(err), zap.Int("count", len(urls)))
		return
	}
	p.ctx.Log.Info("baidu cron push result", zap.Any("result", res))
}

func (p *PushToSearchEngine) runBingPush() {
	withinHours := p.BingCronWithinHours
	if withinHours <= 0 {
		withinHours = 1
	}
	p.ctx.Log.Info("bing cron task started", zap.Int("within_hours", withinHours))

	withinTime := time.Now().Add(-time.Duration(withinHours) * time.Hour).Unix()
	articles, err := service.Article.ListAfterCreateTime(&context.Context{
		Select: "id, slug, status",
		Where: &context.Where{
			Field:    "status",
			Operator: context.WhereOperatorEqualTrue,
		},
	}, withinTime)
	if err != nil {
		p.ctx.Log.Error("bing query articles error", zap.Error(err))
		return
	}

	if len(articles) == 0 {
		p.ctx.Log.Info("bing no articles to push", zap.Int("within_hours", withinHours))
		return
	}

	// 收集URL
	var urls []string
	for _, article := range articles {
		urls = append(urls, article.FullURL())
	}

	p.ctx.Log.Info("bing found articles to push", zap.Int("count", len(urls)), zap.Int("within_hours", withinHours))

	// 批量推送
	res, err := p.PushURLToBing(urls...)
	if err != nil {
		p.ctx.Log.Error("bing cron push error", zap.Error(err), zap.Int("count", len(urls)))
		return
	}
	p.ctx.Log.Info("bing cron push result", zap.Any("result", res))
}

func (p *PushToSearchEngine) Load(ctx *pluginEntity.Plugin) error {
	p.ctx = ctx

	// 如果数据库中的 CronExp 为空，设置默认值
	if ctx.Info.CronEnable && ctx.Info.CronExp == "" {
		ctx.Info.CronExp = "@every 1h"
	}

	service.Article.AddCreateAfterEvents(p)
	service.Article.AddUpdateAfterEvents(p)
	return nil
}

func (p *PushToSearchEngine) ArticleCreateAfter(item *entity.Article) {
	// 百度推送
	if p.BaiduEnable && p.BaiduEnableOnCreate {
		p.pushToBaidu(item, "create")
	}
	// Bing 推送
	if p.BingEnable && p.BingEnableOnCreate {
		p.pushToBing(item, "create")
	}
}

func (p *PushToSearchEngine) ArticleUpdateAfter(item *entity.Article) {
	// 百度推送
	if p.BaiduEnable && p.BaiduEnableOnUpdate {
		p.pushToBaidu(item, "update")
	}
	// Bing 推送
	if p.BingEnable && p.BingEnableOnUpdate {
		p.pushToBing(item, "update")
	}
}

func (p *PushToSearchEngine) pushToBaidu(item *entity.Article, action string) {
	res, err := p.PushURLToBaidu(item.FullURL())
	if err != nil {
		p.ctx.Log.Error("baidu push error", zap.Error(err), zap.Int("article_id", item.ID), zap.String("title", item.Title), zap.String("action", action))
		return
	}
	logFields := []zap.Field{
		zap.String("url", item.FullURL()),
		zap.Any("result", res),
		zap.Int("article_id", item.ID),
		zap.String("title", item.Title),
		zap.String("action", action),
	}
	if res.Success == 0 {
		p.ctx.Log.Error("baidu push failed", logFields...)
		return
	}
	p.ctx.Log.Info("baidu push success", logFields...)
}

func (p *PushToSearchEngine) pushToBing(item *entity.Article, action string) {
	res, err := p.PushURLToBing(item.FullURL())
	if err != nil {
		p.ctx.Log.Error("bing push error", zap.Error(err), zap.Int("article_id", item.ID), zap.String("title", item.Title), zap.String("action", action))
		return
	}
	logFields := []zap.Field{
		zap.String("url", item.FullURL()),
		zap.Any("result", res),
		zap.Int("article_id", item.ID),
		zap.String("title", item.Title),
		zap.String("action", action),
	}
	if res.StatusCode != 200 {
		p.ctx.Log.Error("bing push failed", logFields...)
		return
	}
	p.ctx.Log.Info("bing push success", logFields...)
}

// PushURLToBaidu 推送url到百度
func (p *PushToSearchEngine) PushURLToBaidu(uri ...string) (*SearchEngineBaiduResult, error) {
	if config.Config.Site.URL == "" {
		return nil, errors.New("site url undefined")
	}
	if p.BaiduApiURL == "" {
		return nil, errors.New("baidu api url undefined")
	}
	if len(uri) == 0 {
		return nil, errors.New("uri is required")
	}
	var val = strings.Join(uri, "\n")
	body, err := request.New().PostReturnBody(p.BaiduApiURL, strings.NewReader(val))
	if err != nil {
		return nil, err
	}
	var res SearchEngineBaiduResult
	if err = json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// PushURLToBing 推送url到Bing
func (p *PushToSearchEngine) PushURLToBing(uri ...string) (*SearchEngineBingResult, error) {
	if config.Config.Site.URL == "" {
		return nil, errors.New("site url undefined")
	}
	if p.BingApiKey == "" {
		return nil, errors.New("bing api key undefined")
	}
	if len(uri) == 0 {
		return nil, errors.New("uri is required")
	}
	var push searchEngineBingReq
	push.Host = strings.ReplaceAll(strings.ReplaceAll(
		config.Config.Site.URL, "https://", ""), "http://", "")
	push.Key = p.BingApiKey
	push.KeyLocation = config.Config.Site.URL + "/" + p.BingApiKey + ".txt"
	push.URLList = uri
	reqBody, err := json.Marshal(push)
	if err != nil {
		return nil, err
	}
	r := request.New()
	r.AddHeader("Content-Type", "application/json; charset=utf-8")
	body, err := r.Post("https://api.indexnow.org/IndexNow", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	return &SearchEngineBingResult{
		StatusCode: body.StatusCode,
		Message:    getSearchEngineResultMessage(body.StatusCode),
	}, nil
}

// searchEngineBingReq 必应提交请求
type searchEngineBingReq struct {
	Host        string   `json:"host"`
	Key         string   `json:"key"`
	KeyLocation string   `json:"keyLocation"`
	URLList     []string `json:"urlList"`
}

// SearchEngineBaiduResult 百度提交结果
type SearchEngineBaiduResult struct {
	Success     int      `json:"success"`       // 成功推送的url条数
	Remain      int      `json:"remain"`        // 当天剩余的可推送url条数
	NotSameSite []string `json:"not_same_site"` // 由于不是本站url而未处理的url列表
	NotValid    []string `json:"not_valid"`     // 不合法的url列表
	Error       int      `json:"error"`         // 错误码，与状态码相同
	Message     string   `json:"message"`       // 错误描述
}

// SearchEngineBingResult 必应提交结果
type SearchEngineBingResult struct {
	StatusCode int    `json:"status_code"` // HTTP 状态码
	Message    string `json:"message"`     // 状态码对应的消息
}

// getSearchEngineResultMessage 根据状态码获取对应的消息
func getSearchEngineResultMessage(statusCode int) string {
	switch statusCode {
	case 200:
		return "Ok"
	case 400:
		return "Bad request"
	case 403:
		return "Forbidden"
	case 422:
		return "Unprocessable Entity"
	case 429:
		return "Too Many Requests"
	default:
		return "Unknown error"
	}
}
