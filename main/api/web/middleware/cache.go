package middleware

import (
	"moss/domain/config"
	"moss/infrastructure/support/cache"
	"moss/infrastructure/support/log"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func Cache(ctx *fiber.Ctx) error {

	if !config.Config.Cache.Enable || ctx.Method() != "GET" {
		return ctx.Next()
	}

	name := ctx.Route().Name
	rawKey := ctx.Path()
	// 小说章节分页：chapter 查询参数并入缓存 key，避免不同章互相污染；
	// chapter=1 视同无参数（与 PreBuildArticleCache 预构建的裸 path key 保持一致）
	if ch := ctx.Query("chapter"); ch != "" && ch != "1" {
		if _, err := strconv.Atoi(ch); err == nil {
			rawKey += "?chapter=" + ch
		}
	}
	option := config.Config.Cache.GetOption(name)

	if option == nil || !option.Enable {
		return ctx.Next()
	}

	// Normalize the cache key to ensure consistency
	key := cache.NormalizeCacheKey(rawKey)

	// 默认不打印错误，否则找不到文件错误会爆满
	if val, err := cache.Get(name, key); err == nil {
		log.Debug("cache middleware hit", zap.String("route", name), zap.String("key", key))
		return ctx.Type("html").Send(val)
	}

	log.Debug("cache middleware miss", zap.String("route", name), zap.String("key", key))

	next := ctx.Next()

	if ctx.Response().StatusCode() == 200 {
		if err := cache.Set(name, key, ctx.Response().Body(), option.TTL.Duration()); err != nil {
			log.Warn("set cache error", log.Err(err))
		} else {
			log.Debug("cache middleware stored", zap.String("route", name), zap.String("key", key))
		}
	}

	return next
}
