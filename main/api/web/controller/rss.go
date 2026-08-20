package controller

import (
	"github.com/gofiber/fiber/v2"
	"moss/application/service"
	"moss/domain/config"
)

var RSS = new(RSSController)

type RSSController struct{}

// RSS RSS 2.0 订阅
func (c RSSController) RSS(ctx *fiber.Ctx) error {
	if !service.RSS.IsEnable() {
		return ctx.Status(404).SendString("Not Found")
	}

	articles, err := service.RSS.ArticleList()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}

	siteURL := config.Config.Site.URL
	siteName := config.Config.Site.Name
	siteDesc := config.Config.Site.Description

	xml := service.RSS.GenerateRSS(articles, siteURL, siteName, siteDesc)
	return ctx.Type("xml").SendString(xml)
}

// Atom Atom 1.0 订阅
func (c RSSController) Atom(ctx *fiber.Ctx) error {
	if !service.RSS.IsEnable() {
		return ctx.Status(404).SendString("Not Found")
	}

	articles, err := service.RSS.ArticleList()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}

	siteURL := config.Config.Site.URL
	siteName := config.Config.Site.Name
	siteDesc := config.Config.Site.Description

	xml := service.RSS.GenerateAtom(articles, siteURL, siteName, siteDesc)
	return ctx.Type("xml").SendString(xml)
}
