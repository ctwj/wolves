package plugins

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
	"go.uber.org/zap"
	"moss/domain/core/entity"
	"moss/domain/core/service"
	pluginEntity "moss/domain/support/entity"
	"strings"
)

type ArticleSanitizer struct {
	EnableOnCreate                      bool `json:"enable_on_create"`                          // 创建时执行
	EnableOnUpdate                      bool `json:"enable_on_update"`                          // 更新时执行
	AllowRelativeURLs                   bool `json:"allow_relative_urls"`                       // 禁止本地URL
	RequireNoFollowOnLinks              bool `json:"require_no_follow_on_links"`                // 所有a标签 都添加 rel="nofollow"
	AddTargetBlankToFullyQualifiedLinks bool `json:"add_target_blank_to_fully_qualified_links"` // a标签增加 _blank
	RemoveLinks                         bool `json:"remove_links"`                              // 删除a标签
	RemoveLinksHoldLength               int  `json:"remove_links_hold_length"`                  // 删除a标签时，保留内容的最小字符数，字符数小于此设置的，去掉a标签但保留内容
	EnableTextReplace                   bool `json:"enable_text_replace"`                       // 是否启用文本替换
	TextReplacements                    []TextReplacement `json:"text_replacements"`               // 文本替换规则列表
	ctx                                 *pluginEntity.Plugin
}

// TextReplacement 文本替换规则
type TextReplacement struct {
	Source string `json:"source"` // 要替换的文本
	Target string `json:"target"` // 替换后的文本
}

func NewArticleSanitizer() *ArticleSanitizer {
	return &ArticleSanitizer{
		EnableOnCreate:                      true,
		EnableOnUpdate:                      false,
		AllowRelativeURLs:                   false,
		AddTargetBlankToFullyQualifiedLinks: true,
		RequireNoFollowOnLinks:              true,
		RemoveLinks:                         false,
		RemoveLinksHoldLength:               6,
		EnableTextReplace:                   false,
		TextReplacements:                    []TextReplacement{},
	}
}

func (a *ArticleSanitizer) Info() *pluginEntity.PluginInfo {
	return &pluginEntity.PluginInfo{
		ID:    "ArticleSanitizer",
		About: "to scrub content of XSS when created or updated",
	}
}

func (a *ArticleSanitizer) Load(ctx *pluginEntity.Plugin) error {
	a.ctx = ctx
	service.Article.AddCreateBeforeEvents(a)
	service.Article.AddUpdateBeforeEvents(a)
	return nil
}

func (a *ArticleSanitizer) ArticleCreateBefore(item *entity.Article) error {
	if a.EnableOnCreate {
		return a.sanitize(item, "create")
	}
	return nil
}

func (a *ArticleSanitizer) ArticleUpdateBefore(item *entity.Article) error {
	if a.EnableOnUpdate {
		return a.sanitize(item, "update")
	}
	return nil
}

func (a *ArticleSanitizer) sanitize(item *entity.Article, action string) error {

	if a.RemoveLinks {
		if err := a.removeLinks(item); err != nil {
			return err
		}
	}

	p := bluemonday.UGCPolicy()
	p.AllowDataURIImages()                                                       // 验证base64图片的合法性
	p.RequireParseableURLs(true)                                                 // 过滤非法url
	p.AddTargetBlankToFullyQualifiedLinks(a.AddTargetBlankToFullyQualifiedLinks) // a标签增加 _blank
	p.RequireNoFollowOnLinks(a.RequireNoFollowOnLinks)                           // 所有a标签 都添加 rel="nofollow"
	p.AllowRelativeURLs(a.AllowRelativeURLs)                                     // 禁止本地url
	// 代码块标签允许class属性
	p.AllowAttrs("class").OnElements("code", "pre")
	//p.AllowURLSchemes("mailto", "http", "https")       // 指定url协议头
	item.Content = p.Sanitize(item.Content)

	item.Content = a.injectP(item.Content)

	// 文本替换（对内容、描述、关键字都生效）
	if a.EnableTextReplace && len(a.TextReplacements) > 0 {
		item.Content = a.replaceText(item.Content)
		item.Description = a.replaceText(item.Description)
		item.Keywords = a.replaceText(item.Keywords)
		a.ctx.Log.Debug("应用文本替换规则",
			zap.Int("rules", len(a.TextReplacements)),
			zap.String("action", action))
	}

	//a.ctx.Log.Info(fmt.Sprintf("%s sanitize success", item.Title))
	return nil
}

// 把\n 换行 替换成p标签换行
func (a *ArticleSanitizer) injectP(content string) string {
	if strings.Contains(content, "<p") {
		return content
	}
	var newContent string
	var arr = strings.Split(content, "\n")
	if len(arr) == 0 {
		return content
	}
	for _, val := range arr {
		newContent += "<p>" + val + "</p>"
	}
	return newContent
}

func (a *ArticleSanitizer) removeLinks(item *entity.Article) error {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(item.Content))
	if err != nil {
		a.ctx.Log.Error("format html document error", zap.Error(err), zap.String("title", item.Title))
		return err
	}

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		val, _ := s.Html()
		text := s.Text()
		if strings.Contains(val, "<img") {
			s.AfterHtml(val)
			s.Remove()
			return
		}
		if a.RemoveLinksHoldLength > 0 && len([]rune(text)) <= a.RemoveLinksHoldLength {
			s.AfterHtml(val)
		}
		s.Remove()
	})

	content, err := doc.Find("body").Html()
	if err != nil {
		a.ctx.Log.Error("get html code error", zap.Error(err), zap.String("title", item.Title))
		return err
	}
	item.Content = content
	return nil
}

// replaceText 替换文本内容（简单的字符串替换）
func (a *ArticleSanitizer) replaceText(content string) string {
	if len(a.TextReplacements) == 0 {
		return content
	}

	// 直接在内容上进行字符串替换
	result := content
	for _, repl := range a.TextReplacements {
		if repl.Source != "" && repl.Source != repl.Target {
			result = strings.ReplaceAll(result, repl.Source, repl.Target)
		}
	}

	return result
}

func (a *ArticleSanitizer) Run(ctx *pluginEntity.Plugin) (err error) {
	return nil
}
