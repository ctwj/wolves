package plugins

// QuickSearchLinks 首页快速搜索快捷入口：配置一组「文案 + 搜索词 + 图标 + 色系」，
// wolves 主题轮播下方渲染为胶囊按钮组，点击直达站内搜索结果页。
// 仅 wolves 主题消费（Widget.QuickSearch），其他主题不受影响。

import (
	"go.uber.org/zap"
	pluginEntity "moss/domain/support/entity"
)

type QuickSearchLinks struct {
	Enable bool                           `json:"enable"`
	Title  string                         `json:"title"`
	Items  []pluginEntity.QuickSearchItem `json:"items"`

	ctx *pluginEntity.Plugin
}

func NewQuickSearchLinks() *QuickSearchLinks {
	return &QuickSearchLinks{
		Enable: true,
		Title:  "快速搜索",
		Items: []pluginEntity.QuickSearchItem{
			{Label: "微信多开", Keyword: "微信", Icon: "fas fa-comments", Color: "green"},
			{Label: "Photoshop", Keyword: "Photoshop", Icon: "fas fa-image", Color: "blue"},
			{Label: "Office", Keyword: "Office", Icon: "fas fa-file-word", Color: "amber"},
			{Label: "IDM 下载", Keyword: "IDM", Icon: "fas fa-download", Color: "violet"},
			{Label: "视频播放", Keyword: "播放器", Icon: "fas fa-play", Color: "pink"},
			{Label: "图片查看", Keyword: "图片", Icon: "fas fa-images", Color: "cyan"},
			{Label: "系统清理", Keyword: "清理", Icon: "fas fa-broom", Color: "green"},
			{Label: "压缩解压", Keyword: "解压", Icon: "fas fa-file-archive", Color: "blue"},
		},
	}
}

func (p *QuickSearchLinks) Info() *pluginEntity.PluginInfo {
	return &pluginEntity.PluginInfo{
		ID:        "QuickSearchLinks",
		About:     "首页快速搜索快捷入口（wolves 主题）：轮播下方展示一组可点击的搜索胶囊按钮，文案与搜索词在此配置",
		RunEnable: true,
	}
}

func (p *QuickSearchLinks) Load(ctx *pluginEntity.Plugin) error {
	p.ctx = ctx
	if p.Title == "" {
		p.Title = "快速搜索"
	}
	return nil
}

// Run 手动运行仅输出当前配置概况（本插件是配置型插件，无采集动作）
func (p *QuickSearchLinks) Run(ctx *pluginEntity.Plugin) (err error) {
	p.ctx = ctx
	p.ctx.Log.Info("quick search links", zap.Bool("enable", p.Enable), zap.Int("items", len(p.Items)))
	return
}

// QuickSearchData 供模板 Widget 读取当前配置（接口断言调用，避免包间依赖）
func (p *QuickSearchLinks) QuickSearchData() (enable bool, title string, items []pluginEntity.QuickSearchItem) {
	return p.Enable, p.Title, p.Items
}
