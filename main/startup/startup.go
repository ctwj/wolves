package startup

import (
	appService "moss/application/service"
	"moss/infrastructure/general/command"
	"moss/plugins"
	"os"
	"time"

	"github.com/gookit/color"
)

// expireAt 程序有效期:2027-01-01 起禁止启动
var expireAt = time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local)

func init() {
	checkExpire()
	executeCommand()
	initPlugins()
}

// checkExpire 启动时间限制:已到期直接退出;未到期则注册定时器,让运行中的实例在到期时刻自动退出
func checkExpire() {
	now := time.Now()
	if !now.Before(expireAt) {
		color.Red.Println("program expired on 2027-01-01, contact the administrator for a new version")
		os.Exit(1)
	}
	time.AfterFunc(expireAt.Sub(now), func() {
		color.Red.Println("program expired, exiting")
		os.Exit(1)
	})
}

func executeCommand() {
	if command.AdminPath != "" {
		if err := appService.AdminPathUpdate(command.AdminPath); err != nil {
			panic(err)
		}
		color.Green.Println("admin path updated successfully\n")
	}
	if command.AdminUsername != "" {
		if err := appService.AdminUsernameUpdate(command.AdminUsername); err != nil {
			panic(err)
		}
		color.Green.Println("admin username updated successfully\n")
	}
	if command.AdminPassword != "" {
		if err := appService.AdminPasswordUpdate(command.AdminPassword); err != nil {
			panic(err)
		}
		color.Green.Println("admin password updated successfully\n")
	}
	if command.AdminPath != "" || command.AdminUsername != "" || command.AdminPassword != "" {
		os.Exit(0)
	}
}

func initPlugins() {
	appService.PluginInit(
		plugins.NewGenerateSlug(),
		plugins.NewArticleSanitizer(),
		plugins.NewSaveArticleImages(),
		plugins.NewDetectLinks(),
		// plugins.NewGenerateDescription(),
		plugins.NewPreBuildArticleCache(),
		// plugins.NewPushToBaidu(),
		// plugins.NewPushToBing(),
		plugins.NewPushToSearchEngine(), // 统一的搜索引擎推送插件（百度+Bing）
		plugins.NewMakeCarousel(),
		plugins.NewPostStore(),
		// plugins.NewDidiAuto(),
		plugins.NewGnDownSpider(),
		plugins.NewHeadlessSpider(),
		plugins.NewDownloadLimit(),
		plugins.NewBaiduCloudTransfer(),
		plugins.NewQuarkCloudTransfer(),
		plugins.NewAISeoPlugin(),
		plugins.NewExternalLinkPlugin(),
		plugins.NewDirectLinkDownload(),
	)

}
