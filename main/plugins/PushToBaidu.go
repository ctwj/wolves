package plugins

import (
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

type PushToBaidu struct {
	EnableOnCreate bool   `json:"enable_on_create"` // 创建时执行
	EnableOnUpdate bool   `json:"enable_on_update"` // 更新时执行
	ApiURL         string `json:"api_url"`          // api地址

	CronWithinHours int `json:"cron_within_hours"` // 定时任务：处理过去多少小时内的文章，默认1小时

	ctx *pluginEntity.Plugin
}

func NewPushToBaidu() *PushToBaidu {
	return &PushToBaidu{
		CronWithinHours: 1, // 默认处理过去1小时内的文章
	}
}

func (p *PushToBaidu) Info() *pluginEntity.PluginInfo {
	return &pluginEntity.PluginInfo{
		ID:         "PushToBaidu",
		About:      "push article to baidu when created or updated or via cron",
		RunEnable:  true,
		CronEnable: true,
		PluginInfoPersistent: pluginEntity.PluginInfoPersistent{
			CronStart: false,
			CronExp:   "@every 1h",
		},
	}
}
func (p *PushToBaidu) Run(ctx *pluginEntity.Plugin) error {
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

func (p *PushToBaidu) Load(ctx *pluginEntity.Plugin) error {
	p.ctx = ctx

	// 如果数据库中的 CronExp 为空，设置默认值
	if ctx.Info.CronEnable && ctx.Info.CronExp == "" {
		ctx.Info.CronExp = "@every 1h"
	}

	service.Article.AddCreateAfterEvents(p)
	service.Article.AddUpdateAfterEvents(p)
	return nil
}
func (p *PushToBaidu) ArticleCreateAfter(item *entity.Article) {
	if p.EnableOnCreate {
		p.pushArticle(item, "create")
	}
}

func (p *PushToBaidu) ArticleUpdateAfter(item *entity.Article) {
	if p.EnableOnUpdate {
		p.pushArticle(item, "update")
	}
}

func (p *PushToBaidu) pushArticle(item *entity.Article, action string) {
	p.push("article", item.FullURL(), zap.Any("article", map[string]interface{}{"id": item.ID, "title": item.Title}), zap.String("action", action))
}

func (p *PushToBaidu) push(title, uri string, logs ...zap.Field) {
	res, err := p.PushURL(uri)
	if err != nil {
		p.ctx.Log.Error(title+" push error!", zap.Error(err))
		return
	}
	var logAll = append([]zap.Field{zap.String("url", uri), zap.Any("result", res)}, logs...)
	if res.Success == 0 {
		p.ctx.Log.Error(title+" push error!", append(logAll, zap.Error(err))...)
		return
	}
	p.ctx.Log.Info("article push success.", logAll...)
}

// PushURL 推送url
func (p *PushToBaidu) PushURL(uri ...string) (*PushToBaiduResult, error) {
	if config.Config.Site.URL == "" {
		return nil, errors.New("site url undefined")
	}
	if p.ApiURL == "" {
		return nil, errors.New("api url undefined")
	}
	if len(uri) == 0 {
		return nil, errors.New("uri is required")
	}
	var val = strings.Join(uri, "\n")
	body, err := request.New().PostReturnBody(p.ApiURL, strings.NewReader(val))
	if err != nil {
		return nil, err
	}
	var res PushToBaiduResult
	if err = json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// PushToBaiduResult 百度提交结果
type PushToBaiduResult struct {
	Success     int      `json:"success"`       // 成功推送的url条数
	Remain      int      `json:"remain"`        // 当天剩余的可推送url条数
	NotSameSite []string `json:"not_same_site"` // 由于不是本站url而未处理的url列表
	NotValid    []string `json:"not_valid"`     // 不合法的url列表
	Error       int      `json:"error"`         // 错误码，与状态码相同
	Message     string   `json:"message"`       // 错误描述
}
