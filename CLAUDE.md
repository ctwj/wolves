# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

定制版 Moss CMS(Go 模块名 `moss`,fork 自开源 Moss CMS,上游 remote 为 github.com/ctwj/moss)。Go 后端(Fiber + GORM + Zap + Viper)+ Vue 3 管理后台 + Jet 模板主题,支持 sqlite/mysql/postgresql。本仓库为完整可构建工程;同机 `G:\server\moss` 是原始工作副本(含 conf.toml、moss.db 等运行时数据,两边 git 独立,改代码后需手动同步)。

历史背景:本仓库最初只提交了相对上游的定制文件(2026-08 从 moss 补齐核心文件后成为完整工程),因此全部插件、`08rj` 主题、模板引擎/缓存/上传等处的改动都是本项目定制内容,与上游 moss 不同。

## 常用命令

```bash
task dev                  # 完整开发环境:后端 Air 热重载 (:9008) + 前端 Vite (:3000/admin/)
task run                  # 仅启动后端(无热重载)
task build                # 生产构建:前端 + Linux AMD64 后端(产物在 dist/)
task build-admin          # 仅构建前端

cd main && go test ./...              # 全部后端测试
cd main && go test ./plugins/...      # 插件测试
cd main && go test -v -run TestXxx ./plugins/   # 单个测试函数
```

- **前端包管理器是 pnpm**(`admin/pnpm-lock.yaml`,CI 也用 pnpm;Taskfile 里写的 `npm run` 同样可用,因为走的都是 package.json scripts)。依赖安装:`cd admin && pnpm install`。
- **重要**:`go:embed` 嵌入的 `main/resources/admin/` 只有 `.empty`(点开头文件不参与 embed),**必须先构建前端才能编译后端**(`main/resources/admin/*` 已被 git 忽略,只跟踪 `.empty`)。`task build` 和 CI 都遵循"先前端后后端"的顺序;全新克隆后直接 `go build` 会报 `cannot embed directory admin`。
- 运行时配置 `main/conf.toml`(git 忽略,仓库中没有):`addr`/`db`/`dsn` 三项,dsn 示例见 README.md;可从 `G:\server\moss\main\conf.toml` 参考(内含数据库连接串,勿提交)。
- 二进制命令行参数:`--username` / `--password` / `--adminpath` / `--config` 可重置管理员、后台路径或指定配置文件(见 `main/startup/startup.go`)。
- CI(`.github/workflows/release.yml`):推送 `v*` tag 或手动触发 → 构建 admin 前端 → Go 1.23 六平台交叉编译(UPX 压缩)→ 创建 GitHub Release,默认端口 9008。
- 换行符:仓库索引为 LF(`core.autocrlf=true`);从 `G:\server\moss`(工作区 CRLF)复制文件时注意 `diff` 会全量报差异,先做换行符归一化再比。

## 架构

### 后端(Go,分层 DDD)

请求流:`cmd/web/main.go`(入口)→ `api/web/`(Fiber controller/router/middleware)→ `application/service`(应用服务)→ `domain/`(`core` 为文章/栏目/标签/链接的实体与服务,`config` 为全局配置聚合,启动时经 `domain/config/config.go` 的 init 同步入库)→ `infrastructure/`(`persistent` 数据库与存储驱动、`support` 缓存/模板/上传/日志、`utils` 工具)。

### 插件系统(主要定制开发区域)

- 每个插件是 `main/plugins/` 下一个实现 `PluginEntry` 接口的导出结构体(定义于 `domain/support/entity/plugin.go`):
  ```go
  Info() *PluginInfo      // ID、About、RunEnable/CronEnable、CronExp 等
  Load(ctx *Plugin) error // 装载;可在此向领域服务注册事件钩子
  Run(ctx *Plugin) error  // 手动/定时(cron)执行的任务
  ```
- 插件结构体的导出字段即配置项(json tag),持久化到数据库并可在后台编辑。
- **必须**在 `main/startup/startup.go` 的 `initPlugins()` 中注册(`appService.PluginInit(plugins.NewXxx(), ...)`),未注册的插件(如 `NewDidiAuto`、`PushToBaidu`、`PushToBing`)处于注释停用状态。
- 事件钩子:`domain/core/event/` 定义 `ArticleCreateBefore`/`ArticleUpdateBefore` 等接口,插件在 `Load()` 里自注册,如 `service.Article.AddCreateBeforeEvents(s)`(见 `SaveArticleImages.go`、`ArticleSanitizer.go`)。文章保存时这些钩子会被领域服务回调。
- 插件分类:内容处理(ArticleSanitizer、GenerateSlug、SaveArticleImages、DetectLinks、PreBuildArticleCache、MakeCarousel、PostStore 定时从仓库发布文章)、SEO 推送(PushToSearchEngine 统一百度+Bing,已取代 PushToBaidu/PushToBing)、网盘转存(BaiduCloudTransfer、QuarkCloudTransfer、DirectLinkDownload)、采集(GnDownSpider)、下载限流(DownloadLimit)、AI SEO(AISeoPlugin)、外链处理(ExternalLinkPlugin)。百度相关公共逻辑在 `main/plugins/baidu_utils/`。
- 已知问题(继承自 moss,未修复):`go vet` 报 `AISeoPlugin.go` 多处复制含 `sync.Mutex` 的 `APIConfig` 结构体值。

### 主题与模板

- 模板引擎为 Jet(CloudyKit/jet v6),封装在 `infrastructure/support/template/`;`template/query/` 下 article/category/tag/link 提供模板可调用的数据查询方法,`widget/` 提供小组件。
- 主题放在 `main/resources/themes/<name>/`,含 `theme.json`(元数据)、`template/`(index/article/category/tag/layout 等)、`public/`(静态资源)、`page/`(独立页面)。通过 `main/resources/resource.go` 的 `go:embed` 打进二进制。
- `germ` 为上游默认主题(Tailwind CSS),`08rj` 为本项目定制主题("零八软件",软件资源下载站,配套 DownloadLimit/DirectLinkDownload 等插件的 download.html 下载模板)。
- `wolves` 为多媒体内容主题(基于 08rj 复制改造,小说/图片/视频):文章与分类以 `content_type` 字段(novel/image/video/空=普通)驱动 `category.html`/`article.html` 内的类型分支子模板(`template/component/{category,article}/`);视频选集存 `Extends["video_sources"]`、图集存 `Extends["gallery_images"]`,小说正文用 `===chapter===` 分隔符分章(render 层按 `?chapter=N` 服务端分页,缓存 key 已并入章号)。纯函数与测试在 `main/application/service/contenttype/`。详见主题内 README 与 `specs/001-wolves-multimedia-theme/`。Jet v6 陷阱:空值判断用 truthiness 或 `len()`,勿用 `!= nil`。

### 前端(admin)

Vue 3 + Vite + Arco Design(部分 Naive UI)+ Pinia + Vue I18n(12 种语言),源码在 `admin/`。`admin/src/views/config/module/more.vue` 通过 `inject('data')` 向后台"更多设置"页追加自定义配置项(article_views_pool、fast_offset_min_page 等);`tailwind.08rj.config.cjs`/`tailwind-08rj.css` 为 08rj 主题定制的后台样式。构建产物直接输出到 `main/resources/admin`(vite.config.js 的 outDir)并被后端嵌入。

### 其他

- `docs/`:docker/docker-compose 与宝塔进程守护部署指南、模板与主题开发 README。
- `AGENTS.md` / `DEVELOPMENT.md`:另一套开发规范文档(与本文档内容重叠,侧重点不同)。
- `extras/火车头发布模块/`:火车头采集器对接 Moss 的文章发布模块(.wpm 为二进制,未跟踪)。
- 测试文件与插件同目录(`*_test.go`);部分测试 import `domain/config`,其 init 会访问数据库,直接 `go test` 可能生成/读取本地 moss.db。
- `.gitignore` 有意排除:conf.toml、*.db、main/public、main/themes 与 main/resources/admin 构建产物(运行时生成)、tmp/、openspec/、.claude/、docs/plans/、scripts/。

<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:
`specs/001-wolves-multimedia-theme/plan.md` (wolves 多媒体内容主题:
spec.md / research.md / data-model.md / contracts/ / quickstart.md)
<!-- SPECKIT END -->
