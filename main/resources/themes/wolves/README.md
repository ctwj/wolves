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

## 子菜单（子分类）显示场景

子分类（后台把分类的父级设为顶级类型分类）自动出现在四个场景，无需任何配置：

| 场景 | 位置 | 形态 |
|------|------|------|
| 主导航下拉 | 顶栏菜单 hover | 子项带类型色图标（film/book/images） |
| 移动端菜单 | 汉堡菜单 | 父项图标 + 缩进左色条列表 |
| **分类页筛选 tab 条** | 标题横幅下方 | 胶囊 tab、当前分类高亮、**类型色激活**（video=红/novel=青/image=绿）；浏览子分类时显示兄弟分类 + "全部"回父级；移动端横向滑动 |
| 首页类型入口卡 | 三入口大卡内 | 最多 4 个子分类直达小链接 |

子分类的 `content_type` 建议与父级一致（后台手动设置），列表布局随类型走。

## 模板开发注意（Jet v6 陷阱）

- **空值判断**：对可空对象/any 一律用 truthiness（`{{if x}}`）或 `{{if len(x) > 0}}`，**不要用 `{{if x != nil}}`**（对 typed-nil 指针与 any 包装的空切片会误判为真，曾导致空库 500）
- 注释语法为 `{* comment *}`（单花括号）
- 列表查询：`query.Article.ListByCategoryID` 等仅返回 `ArticleBase`（无 Extends/Detail）；需要 Extends 时用 `Widget.CategoryPageList(catID, page).List`

## 样式

**Cinema Dark 设计系统（v2，ui-ux-pro-max）**——暗色影院皮肤，全站自定义于 `public/style.css`：

| Token | 值 | 用途 |
|-------|----|------|
| 页面底 | `#0b0d12` | body 背景 |
| 卡片面 | `#151823`（`w-surface`） | 卡片/面板 |
| 悬浮面 | `#1c2030`（`w-sunken`） | 凹陷区/输入底 |
| 边框 | `#262b3d`（`w-border`） | 分隔线 |
| 主文字 | `#f1f3f7`（`w-text`） | 标题 |
| 次文字 | `#aeb6c8`（`w-text-2`） | 正文 |
| 弱文字 | `#8b93a7`（`w-text-3`） | 元信息（≥4.5:1） |
| Play Red | `#e11d48`（CTA）/ `#fb7185`（链接 `w-link`） | 品牌主色 |
| 类型色 | video=rose `#fb7185` · novel=sky `#7dd3fc` · image=emerald `#6ee7b7` | 全站内容类型编码 |

**模板类名约定**：模板中亮色 Tailwind 类已替换为语义类（`w-surface/w-text/w-text-2/w-text-3/w-border/w-tag/w-link/w-sunken/w-gradient-hero`），新模板请继续使用语义类，勿引入 `bg-white`/`text-gray-*` 等亮色类。小说阅读区用 `novel-paper`（亮纸 `#faf8f4`，长文对比 13:1）。

**`tailwind.css` 是预编译产物**——新 Tailwind 原子类不会生效，新样式一律写自定义类进 style.css。

### 无障碍与触控规范

- 弱文字不低于 `#8b93a7`（4.5:1）；勿再用更浅色作正文
- 所有可交互元素显式 `cursor: pointer`，键盘焦点环 2px `#fb7185`（outline-offset 3px）
- 触控目标：选集按钮 min-height 40px、章节导航 44px
- 动效一律补 `prefers-reduced-motion: reduce` 降级
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
