package controller

import (
	"github.com/gofiber/fiber/v2"
	"moss/application/service"
)

var LLMs = new(LLMsController)

type LLMsController struct{}

// LLMsTxt llms.txt 供 AI 爬虫使用
func (c LLMsController) LLMsTxt(ctx *fiber.Ctx) error {
	if !service.LLMs.IsEnable() {
		return ctx.Status(404).SendString("Not Found")
	}

	content := service.LLMs.GenerateLLMsTxt()
	ctx.Set("Content-Type", "text/plain; charset=utf-8")
	return ctx.SendString(content)
}

// API JSON API 端点
func (c LLMsController) API(ctx *fiber.Ctx) error {
	if !service.API.IsEnable() {
		return ctx.Status(404).SendString("Not Found")
	}

	jsonData, err := service.API.GenerateAPIJSON()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}

	return ctx.Type("json").SendString(jsonData)
}
