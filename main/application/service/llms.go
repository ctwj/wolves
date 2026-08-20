package service

import (
	"moss/domain/config"
	"moss/domain/core/entity"
	"moss/domain/core/repository/context"
	"moss/domain/core/service"
	"strings"
	"time"
)

// LLMs llms.txt 服务
var LLMs = new(llmsService)

type llmsService struct{}

// IsEnable 检查 llms 功能是否启用
func (s *llmsService) IsEnable() bool {
	return config.Config.AISEO.LLMsEnable
}

// GenerateLLMsTxt 生成 llms.txt 内容
func (s *llmsService) GenerateLLMsTxt() string {
	var sb strings.Builder

	siteURL := config.Config.Site.URL
	siteName := config.Config.Site.Name
	siteDesc := config.Config.Site.Description

	// 头部信息
	sb.WriteString("# " + siteName + "\n\n")
	sb.WriteString("## 站点信息\n\n")
	sb.WriteString("- **网站名称**: " + siteName + "\n")
	sb.WriteString("- **网站地址**: " + siteURL + "\n")
	sb.WriteString("- **网站描述**: " + siteDesc + "\n\n")

	// 分类
	sb.WriteString("## 分类\n\n")
	categories, _ := service.Category.List(context.NewContext(20, "id asc"))
	for _, cat := range categories {
		sb.WriteString("- [" + cat.Name + "](" + siteURL + cat.URL() + ")\n")
	}
	sb.WriteString("\n")

	// 标签
	sb.WriteString("## 标签\n\n")
	tags, _ := service.Tag.List(context.NewContext(30, "id desc"))
	for _, tag := range tags {
		sb.WriteString("- [" + tag.Name + "](" + siteURL + tag.URL() + ")\n")
	}
	sb.WriteString("\n")

	// 最新文章
	sb.WriteString("## 最新文章\n\n")
	articles, _ := s.ArticleList()
	for _, article := range articles {
		sb.WriteString("### " + article.Title + "\n\n")
		sb.WriteString("- 地址: " + siteURL + article.URL() + "\n")
		if article.Description != "" {
			sb.WriteString("- 描述: " + article.Description + "\n")
		}
		sb.WriteString("- 发布时间: " + time.Unix(article.CreateTime, 0).Format("2006-01-02") + "\n")
		sb.WriteString("\n")
	}

	// 底部信息
	sb.WriteString("---\n\n")
	sb.WriteString("此文件由 www.08rj.com 自动生成，供 AI 爬虫和 LLM 读取。\n")
	sb.WriteString("更新时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")

	return sb.String()
}

// ArticleList 获取文章列表
func (s *llmsService) ArticleList() ([]entity.ArticleBase, error) {
	limit := config.Config.AISEO.LLMsLimit
	if limit <= 0 {
		limit = 20
	}
	ctx := context.NewContext(limit, "id desc")
	ctx.Where = &context.Where{Field: "status", Operator: context.WhereOperatorEqualTrue}
	return service.Article.List(ctx)
}