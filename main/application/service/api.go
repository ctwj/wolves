package service

import (
	"encoding/json"
	"moss/domain/config"
	"moss/domain/core/entity"
	"moss/domain/core/repository/context"
	"moss/domain/core/service"
	"time"
)

// API API 端点服务
var API = new(apiService)

type apiService struct{}

// IsEnable 检查 API 功能是否启用
func (s *apiService) IsEnable() bool {
	return config.Config.AISEO.APIEnable
}

// SiteInfo 网站基本信息
type SiteInfo struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// ArticleInfo 文章信息
type ArticleInfo struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail"`
	URL         string `json:"url"`
	CreateTime  string `json:"create_time"`
	Views       int    `json:"views"`
}

// CategoryInfo 分类信息
type CategoryInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	URL  string `json:"url"`
}

// TagInfo 标签信息
type TagInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	URL  string `json:"url"`
}

// GenerateAPIJSON 生成 API JSON 数据
func (s *apiService) GenerateAPIJSON() (string, error) {
	siteURL := config.Config.Site.URL
	siteName := config.Config.Site.Name
	siteDesc := config.Config.Site.Description

	// 网站信息
	siteInfo := SiteInfo{
		Name:        siteName,
		URL:         siteURL,
		Description: siteDesc,
	}

	// 分类
	categories, _ := service.Category.List(context.NewContext(20, "id asc"))
	var categoryList []CategoryInfo
	for _, cat := range categories {
		categoryList = append(categoryList, CategoryInfo{
			ID:   int64(cat.ID),
			Name: cat.Name,
			Slug: cat.Slug,
			URL:  siteURL + cat.URL(),
		})
	}

	// 标签
	tags, _ := service.Tag.List(context.NewContext(30, "id desc"))
	var tagList []TagInfo
	for _, tag := range tags {
		tagList = append(tagList, TagInfo{
			ID:   int64(tag.ID),
			Name: tag.Name,
			Slug: tag.Slug,
			URL:  siteURL + tag.URL(),
		})
	}

	// 最新文章
	articles, _ := s.ArticleList()
	var articleList []ArticleInfo
	for _, article := range articles {
		articleList = append(articleList, ArticleInfo{
			ID:          int64(article.ID),
			Title:       article.Title,
			Slug:        article.Slug,
			Description: article.Description,
			Thumbnail:   article.Thumbnail,
			URL:         siteURL + article.URL(),
			CreateTime:  time.Unix(article.CreateTime, 0).Format("2006-01-02 15:04:05"),
			Views:       article.Views,
		})
	}

	// 构建响应结构
	response := map[string]interface{}{
		"site":      siteInfo,
		"categories": categoryList,
		"tags":      tagList,
		"articles":  articleList,
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// ArticleList 获取文章列表
func (s *apiService) ArticleList() ([]entity.ArticleBase, error) {
	limit := config.Config.AISEO.APILimit
	if limit <= 0 {
		limit = 20
	}
	ctx := context.NewContext(limit, "id desc")
	ctx.Where = &context.Where{Field: "status", Operator: context.WhereOperatorEqualTrue}
	return service.Article.List(ctx)
}
