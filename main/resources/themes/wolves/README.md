# wolves 主题

基于 08rj 主题定制改造的多媒体内容主题：**小说 / 图片 / 视频**。

## 布局机制（按内容类型分支）

`category.html` 与 `article.html` 按 **content_type** 分支 include 子模板，未标记类型走 08rj 原默认布局：

| content_type | 分类页（template/component/category/） | 详情页（template/component/article/） |
|--------------|----------------------------------------|----------------------------------------|
| `video`      | video.html — 海报封面卡片网格          | video.html — 播放器（直链 video / 嵌入 iframe）+ 选集栏 |
| `novel`      | novel.html — 书籍条目列表              | novel.html — 章节分页阅读 + 上/下章导航 |
| `image`      | image.html — 缩略图画廊网格            | image.html — 图集网格 + 灯箱放大 |
| 空/standard  | 默认紧凑卡片网格（08rj 原样）          | 默认文章页（下载卡/侧栏） |

## 类型专属数据（存于文章 Extends）

- `video_sources`：`[{"label":"第01集","url":"https://...","embed":false}]` 有序数组；`embed=true` 为第三方播放页（iframe 嵌入 + 新窗口降级），false 为直链 mp4
- `gallery_images`：`["url1","url2",...]` 有序图片数组，首图兼作画廊封面候选
- 小说正文即文章 `Content`，章节间单独一行插入分隔符 **`===chapter===`**，每章开头建议用标题标签（作为章节名）；前台按 `?chapter=N` 服务端分页输出（越界自动钳制）

> 分隔符采用纯文本 `===chapter===` 而非 HTML 注释，因 ArticleSanitizer 插件的 bluemonday UGCPolicy 清洗会剥离 HTML 注释。

## 首页三分区

`index.html` 读取 slug 为 `video` / `novel` / `image` 的分类（运营约定，可在后台任意改 slug 后调整模板内 `GetBySlug` 参数），各展示最新条目；分类不存在时板块自动隐藏。

## 模板开发注意（Jet v6 陷阱）

- **空值判断**：对可空对象/any 一律用 truthiness（`{{if x}}`）或 `{{if len(x) > 0}}`，**不要用 `{{if x != nil}}`**（对 typed-nil 指针与 any 包装的空切片会误判为真，曾导致空库 500）
- 注释语法为 `{* comment *}`（单花括号）
- 列表查询：`query.Article.ListByCategoryID` 等仅返回 `ArticleBase`（无 Extends/Detail）；需要 Extends 时用 `Widget.CategoryPageList(catID, page).List`

## 样式

新布局样式集中追加在 `public/style.css`（分区注释 `category: video|novel|image`、`article: ...`、移动端 @media）。
**`tailwind.css` 是预编译产物**——模板中新写的 Tailwind 原子类不会生效，新样式一律写自定义类进 style.css。

### 无障碍与触控规范（ui-ux-pro-max 审查后固化）

- 辅助文字色不低于 `#6b7280`（4.8:1）；勿再用 `#9ca3af` 作正文/元信息色
- 所有可交互元素（含 `<button>`）显式 `cursor: pointer`，键盘焦点环 2px `#00aaff`（outline-offset 3px）
- 触控目标：选集按钮 min-height 40px、章节导航 44px；新增按钮遵循同标准
- 动效一律补 `prefers-reduced-motion: reduce` 降级（hover 位移置 none）
- 当前选集以 `▶` 前缀 + 填色双重指示（不依赖颜色单一通道）
- 小说正文限宽 44em 控制行长

## 开发期同步

源码目录 `main/resources/themes/wolves/` 经 go:embed 打包（新部署自动释放）；
本机运行目录 `./themes` 已存在时种子不释放，执行：

```powershell
# 在 main/ 下
../scripts/sync-wolves-theme.ps1
```

注意：sync 脚本删除重建目录后 fsnotify 热重载不可靠，建议重启进程。

规格与设计文档：`specs/001-wolves-multimedia-theme/`（spec / plan / research / data-model / contracts / tasks）。
