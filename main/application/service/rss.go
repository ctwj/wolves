package service

import (
	"moss/domain/config"
	"moss/domain/core/entity"
	"moss/domain/core/repository/context"
	"moss/domain/core/service"
	"strings"
	"time"
)

// RSS RSS/Atom 数据服务
var RSS = new(rssService)

type rssService struct{}

// IsEnable 检查 RSS 功能是否启用
func (s *rssService) IsEnable() bool {
	return config.Config.AISEO.RSSEnable
}

// ArticleList 获取文章列表用于 RSS/Atom
func (s *rssService) ArticleList() ([]entity.ArticleBase, error) {
	limit := config.Config.AISEO.RSSLimit
	if limit <= 0 {
		limit = 50
	}
	ctx := context.NewContext(limit, "id desc")
	ctx.Where = &context.Where{Field: "status", Operator: context.WhereOperatorEqualTrue}
	return service.Article.List(ctx)
}

// GenerateRSS 生成 RSS 2.0 XML
func (s *rssService) GenerateRSS(articles []entity.ArticleBase, siteURL, siteName, siteDesc string) string {
	var sb strings.Builder
	now := time.Now().Format(time.RFC1123Z)

	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">` + "\n")
	sb.WriteString(`  <channel>` + "\n")
	sb.WriteString(`    <title><![CDATA[` + siteName + `]]></title>` + "\n")
	sb.WriteString(`    <link>` + siteURL + `</link>` + "\n")
	sb.WriteString(`    <description><![CDATA[` + siteDesc + `]]></description>` + "\n")
	sb.WriteString(`    <language>zh-CN</language>` + "\n")
	sb.WriteString(`    <lastBuildDate>` + now + `</lastBuildDate>` + "\n")
	sb.WriteString(`    <generator>Moss CMS</generator>` + "\n")
	sb.WriteString(`    <atom:link href="` + siteURL + `/rss.xml" rel="self" type="application/rss+xml"/>` + "\n")

	for _, article := range articles {
		sb.WriteString(`    <item>` + "\n")
		sb.WriteString(`      <title><![CDATA[` + article.Title + `]]></title>` + "\n")
		sb.WriteString(`      <link>` + siteURL + article.URL() + `</link>` + "\n")
		sb.WriteString(`      <guid isPermaLink="true">` + siteURL + article.URL() + `</guid>` + "\n")
		if article.Description != "" {
			sb.WriteString(`      <description><![CDATA[` + article.Description + `]]></description>` + "\n")
		}
		sb.WriteString(`      <pubDate>` + time.Unix(article.CreateTime, 0).Format(time.RFC1123Z) + `</pubDate>` + "\n")
		sb.WriteString(`    </item>` + "\n")
	}

	sb.WriteString(`  </channel>` + "\n")
	sb.WriteString(`</rss>`)

	return sb.String()
}

// GenerateAtom 生成 Atom 1.0 XML
func (s *rssService) GenerateAtom(articles []entity.ArticleBase, siteURL, siteName, siteDesc string) string {
	var sb strings.Builder
	now := time.Now().Format(time.RFC3339)

	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom">` + "\n")
	sb.WriteString(`  <title>` + siteName + `</title>` + "\n")
	sb.WriteString(`  <link href="` + siteURL + `"/>` + "\n")
	sb.WriteString(`  <link href="` + siteURL + `/atom.xml" rel="self"/>` + "\n")
	sb.WriteString(`  <id>` + siteURL + `/</id>` + "\n")
	sb.WriteString(`  <updated>` + now + `</updated>` + "\n")
	sb.WriteString(`  <subtitle><![CDATA[` + siteDesc + `]]></subtitle>` + "\n")

	for _, article := range articles {
		sb.WriteString(`  <entry>` + "\n")
		sb.WriteString(`    <title>` + article.Title + `</title>` + "\n")
		sb.WriteString(`    <link href="` + siteURL + article.URL() + `"/>` + "\n")
		sb.WriteString(`    <id>` + siteURL + article.URL() + `</id>` + "\n")
		sb.WriteString(`    <updated>` + time.Unix(article.CreateTime, 0).Format(time.RFC3339) + `</updated>` + "\n")
		sb.WriteString(`    <published>` + time.Unix(article.CreateTime, 0).Format(time.RFC3339) + `</published>` + "\n")
		if article.Description != "" {
			sb.WriteString(`    <summary><![CDATA[` + article.Description + `]]></summary>` + "\n")
		}
		sb.WriteString(`  </entry>` + "\n")
	}

	sb.WriteString(`</feed>`)

	return sb.String()
}