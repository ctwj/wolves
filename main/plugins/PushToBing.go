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

type PushToBing struct {
	EnableOnCreate    bool   `json:"enable_on_create"`     // 创建时执行
	EnableOnUpdate    bool   `json:"enable_on_update"`     // 更新时执行
	ApiKey            string `json:"api_key"`              // api地址
	CronWithinHours   int    `json:"cron_within_hours"`   // 定时任务：处理过去多少小时内的文章，默认1小时

	ctx *pluginEntity.Plugin
}

func NewPushToBing() *PushToBing {
	return &PushToBing{
		CronWithinHours: 1, // 默认处理过去1小时内的文章
	}
}

func (p *PushToBing) Info() *pluginEntity.PluginInfo {
	return &pluginEntity.PluginInfo{
		ID:         "PushToBing",
		About:      "push article to bing when created or updated or via cron",
		RunEnable:  true,
		CronEnable: true,
		PluginInfoPersistent: pluginEntity.PluginInfoPersistent{
			CronStart: false,
			CronExp:   "@every 1h",
		},
	}
}
func (p *PushToBing) Run(ctx *pluginEntity.Plugin) error {
	p.ctx = ctx

	// 处理过去CronWithinHours小时内的文章，默认1小时
	withinHours := p.CronWithinHours
	if withinHours <= 0 {
		withinHours = 1
	}
	p.ctx.Log.Info("cron task started", zap.Int("within_hours", withinHours))

	withinTime := time.Now().Add(-time.Duration(withinHours) * time.Hour).Unix()
	articles, err := service.Article.ListAfterCreateTime(&context.Context{
		Select: "id, slug, status",
		Where: &context.Where{
			Field:    "status",
			Operator: context.WhereOperatorEqualTrue,
		},
	}, withinTime)
	if err != nil {
		p.ctx.Log.Error("query articles error", zap.Error(err))
		return err
	}

	if len(articles) == 0 {
		p.ctx.Log.Info("no articles to push", zap.Int("within_hours", withinHours))
		return nil
	}

	// 收集URL
	var urls []string
	for _, article := range articles {
		urls = append(urls, article.FullURL())
	}

	p.ctx.Log.Info("found articles to push", zap.Int("count", len(urls)), zap.Int("within_hours", withinHours))

	// 批量推送
	res, err := p.PushURL(urls...)
	if err != nil {
		p.ctx.Log.Error("cron push error", zap.Error(err), zap.Int("count", len(urls)))
		return err
	}
	p.ctx.Log.Info("cron push result", zap.Any("result", res))

	return nil
}

func (p *PushToBing) Load(ctx *pluginEntity.Plugin) error {
	p.ctx = ctx

	// 如果数据库中的 CronExp 为空，设置默认值
	if ctx.Info.CronEnable && ctx.Info.CronExp == "" {
		ctx.Info.CronExp = "@every 1h"
	}

	service.Article.AddCreateAfterEvents(p)
	service.Article.AddUpdateAfterEvents(p)
	return nil
}
func (p *PushToBing) ArticleCreateAfter(item *entity.Article) {
	if p.EnableOnCreate {
		p.pushArticle(item, "create")
	}
}

func (p *PushToBing) ArticleUpdateAfter(item *entity.Article) {
	if p.EnableOnUpdate {
		p.pushArticle(item, "update")
	}
}

func (p *PushToBing) pushArticle(item *entity.Article, action string) {
	p.push("article", item.FullURL(), zap.Any("article",
		map[string]interface{}{"id": item.ID, "title": item.Title}),
		zap.String("action", action))
}

func (p *PushToBing) push(title, uri string, logs ...zap.Field) {
	res, err := p.PushURL(uri)
	if err != nil {
		p.ctx.Log.Error(title+" push error!", zap.Error(err))
		return
	}
	var logAll = append([]zap.Field{zap.String("url", uri), zap.Any("result", res)}, logs...)
	if res.StatusCode != 200 {
		p.ctx.Log.Error(title+" push error!", append(logAll, zap.Error(err))...)
		return
	}
	p.ctx.Log.Info("article push success.", logAll...)
}

// PushURL 推送url
func (p *PushToBing) PushURL(uri ...string) (*PushToBingResult, error) {
	if config.Config.Site.URL == "" {
		return nil, errors.New("site url undefined")
	}
	if p.ApiKey == "" {
		return nil, errors.New("api key undefined")
	}
	if len(uri) == 0 {
		return nil, errors.New("uri is required")
	}
	var push pushToBingReq
	push.Host = strings.ReplaceAll(strings.ReplaceAll(
		config.Config.Site.URL, "https://", ""), "http://", "")
	push.Key = p.ApiKey
	push.KeyLocation = config.Config.Site.URL + "/" + p.ApiKey + ".txt"
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
	return &PushToBingResult{
		StatusCode: body.StatusCode,
		Message:    getResultMessage(body.StatusCode),
	}, nil
}

// pushToBingReq 必应提交
type pushToBingReq struct {
	Host        string   `json:"host"`
	Key         string   `json:"key"`
	KeyLocation string   `json:"keyLocation"`
	URLList     []string `json:"urlList"`
}

// PushToBingResult 必应提交结果
type PushToBingResult struct {
	StatusCode int    `json:"status_code"` // HTTP 状态码
	Message    string `json:"message"`    // 状态码对应的消息
}

// getResultMessage 根据状态码获取对应的消息
func getResultMessage(statusCode int) string {
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
