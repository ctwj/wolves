package plugins

// NovelMerger 小说章节合并插件：
// 采集来的小说每章被存成独立文章（标题含小说名+章节号），本插件按标题关键词
// 匹配章节 → 后台面板预览（匹配数/章节顺序/识别结果）→ 确认后合并为一篇
// ===chapter=== 分章的 novel 文章（wolves 主题按 ?chapter=N 分章阅读），
// 并硬删除原章节文章。
// 破坏性操作不走 Run/cron（RunEnable=false），只经由后台专用端点
// novelPreview（只读预览）/ novelMerge（执行合并）两步交互完成。

import (
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"strings"
	"sync"
	"time"

	"moss/application/service/contenttype"
	"moss/domain/config"
	"moss/domain/core/entity"
	"moss/domain/core/repository"
	"moss/domain/core/repository/context"
	"moss/domain/core/service"
	pluginEntity "moss/domain/support/entity"
	"moss/infrastructure/support/cache"
)

// 扫描上限：单次预览最多处理的命中文章数，超出置 Truncated 提示收窄关键词
const novelMergeScanLimit = 5000

var errNoCaptureGroup = errors.New("自定义正则缺少捕获组：第一个捕获组须为章节号，示例 (?:第)?(\\d+)\\s*章")

type NovelMerger struct {
	Keyword        string `json:"keyword"`         // 便捷默认值；预览/合并均以请求参数为准
	ChapterPattern string `json:"chapter_pattern"` // 自定义章节号正则（第一捕获组=章节数字），空=内置识别
	mu             sync.Mutex
	ctx            *pluginEntity.Plugin
}

func NewNovelMerger() *NovelMerger {
	return &NovelMerger{}
}

func (n *NovelMerger) Info() *pluginEntity.PluginInfo {
	return &pluginEntity.PluginInfo{
		ID:         "NovelMerger",
		About:      "按标题关键词匹配小说章节，预览确认后合并为一篇多章小说文章（===chapter=== 分章），并删除原章节文章",
		RunEnable:  false, // 破坏性操作禁止 Run/cron 误触
		CronEnable: false,
	}
}

func (n *NovelMerger) Load(ctx *pluginEntity.Plugin) error {
	n.ctx = ctx
	return nil
}

func (n *NovelMerger) Run(ctx *pluginEntity.Plugin) error {
	n.ctx = ctx
	n.ctx.Log.Info("NovelMerger 为交互式插件：请在 插件→小说章节合并 配置页中预览确认后操作")
	return nil
}

// PreviewMerge 预览匹配结果（只读，不入库）：标题含关键词的文章列表、
// 章节号识别与排序、排除项与预检警告。流程级失败填 res.Error 不返回 error。
func (n *NovelMerger) PreviewMerge(raw string) (interface{}, error) {
	start := time.Now()
	req, err := parseNovelMergeRequest(raw)
	if err != nil {
		return nil, err
	}
	res := newNovelMergePreviewResult(req.Keyword, req.Pattern)

	bases, err := n.matchArticles(req.Keyword)
	if err != nil {
		res.Error = fmt.Sprintf("查询匹配文章失败: %v", err)
		res.CostMS = time.Since(start).Milliseconds()
		return res, nil
	}
	if len(bases) >= novelMergeScanLimit {
		res.Truncated = true
	}

	items := make([]NovelMergeChapterItem, 0, len(bases))
	for _, b := range bases {
		if b.ContentType == contenttype.TypeNovel {
			res.ExcludedList = append(res.ExcludedList, NovelMergeExcludedItem{
				ID: b.ID, Title: b.Title, Reason: "已是多章小说文章（不参与合并）",
			})
			continue
		}
		no, matchedBy, ok := extractChapterNo(b.Title, req.Pattern)
		items = append(items, NovelMergeChapterItem{
			ID: b.ID, Title: b.Title, CreateTime: b.CreateTime,
			CategoryID: b.CategoryID, Status: b.Status,
			ChapterNo: no, MatchedBy: matchedBy, Recognized: ok,
		})
	}

	sortMergeChapters(items)
	res.Total = len(bases)
	res.Included = len(items)
	res.Excluded = len(res.ExcludedList)
	for _, it := range items {
		if it.Recognized {
			res.Recognized++
		} else {
			res.Unrecognized++
		}
	}
	res.Chapters = items
	res.DuplicateNos = duplicateChapterNos(items)
	res.Target = NovelMergeTarget{
		Title:      req.Keyword,
		Slug:       buildNovelSlug(req.Keyword),
		CategoryID: majorityCategoryID(items),
	}

	// 同名/slug 预检：同名存在则执行会被整单拒绝（若同名文章本身就在合并集合内——
	// 如小说简介页标题恰好等于书名——不算冲突，其标题将随合并被替换掉）；
	// slug 冲突执行时自动加后缀
	if id, err := repository.Article.GetIdByTitle(req.Keyword); err == nil && id > 0 {
		inSet := false
		for _, it := range items {
			if it.ID == id {
				inSet = true
				break
			}
		}
		res.TitleExists = !inSet
	}
	if exists, err := service.Article.ExistsSlug(res.Target.Slug); err == nil {
		res.SlugTaken = exists
	}

	switch {
	case res.Total == 0:
		res.Error = fmt.Sprintf("没有找到标题包含「%s」的文章", req.Keyword)
	case res.Included == 0:
		res.Error = "命中的文章全部被排除，无可合并的章节"
	}
	if res.Error == "" {
		switch {
		case res.Unrecognized > 0:
			// 未识别不算流程失败：已按时间排尾，仍可确认合并（顺序所见即所得）
			res.Warning = fmt.Sprintf("有 %d 篇未识别出章节号（已按时间排在末尾）；可填写自定义正则重新预览，或直接确认合并",
				res.Unrecognized)
		case res.Truncated:
			res.Warning = fmt.Sprintf("命中文章超过 %d 篇已截断，建议收窄关键词", novelMergeScanLimit)
		}
	}
	res.CostMS = time.Since(start).Milliseconds()

	if n.ctx != nil && n.ctx.Log != nil {
		n.ctx.Log.Info("预览小说合并",
			zap.String("keyword", req.Keyword),
			zap.Int("total", res.Total), zap.Int("included", res.Included),
			zap.Int("unrecognized", res.Unrecognized), zap.Int("excluded", res.Excluded))
	}
	return res, nil
}

// ExecuteMerge 执行合并（破坏性）：按预览确认的 ids 顺序取文 → 全部预检通过后
// Create 合并文章 → 逐篇删除原章节。Create 失败则一篇未删；单篇删除失败记录
// 并继续其余（不回滚，内容双存是安全方向）。
func (n *NovelMerger) ExecuteMerge(raw string) (interface{}, error) {
	start := time.Now()
	req, err := parseNovelMergeRequest(raw)
	if err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errors.New("缺少待合并的文章 ids")
	}
	if !n.mu.TryLock() {
		return nil, errors.New("合并任务进行中，请勿重复提交")
	}
	defer n.mu.Unlock()

	articles, err := n.loadArticlesInOrder(req.IDs)
	if err != nil {
		return nil, err
	}

	// 预检（零副作用，任一失败整单拒绝）
	// 同名冲突：命中 id 不在待合并集合内才拒绝（集合内的同名简介页会随合并被删除）
	if id, e := repository.Article.GetIdByTitle(req.Keyword); e == nil && id > 0 && !containsInt(req.IDs, id) {
		return nil, fmt.Errorf("已存在标题为「%s」的文章（可能此前已合并过），请先手动处理", req.Keyword)
	}
	for _, a := range articles {
		if a.ContentType == contenttype.TypeNovel {
			return nil, fmt.Errorf("文章 [ID:%d] %s 已是多章小说文章，不能作为章节合并", a.ID, a.Title)
		}
		if strings.TrimSpace(a.Content) == "" {
			return nil, fmt.Errorf("文章 [ID:%d] %s 正文为空，合并会丢失该章，请先处理", a.ID, a.Title)
		}
	}

	// slug 冲突自动加后缀
	slug := buildNovelSlug(req.Keyword)
	for i := 1; ; i++ {
		exists, e := service.Article.ExistsSlug(slug)
		if e != nil || !exists {
			break
		}
		slug = fmt.Sprintf("%s-%d", buildNovelSlug(req.Keyword), i)
	}

	// 组装合并文章：标题=关键词，分类沿用章节文章（多数决），时间取最早章节
	article := &entity.Article{
		ArticleBase: entity.ArticleBase{
			Slug:        slug,
			Title:       req.Keyword,
			CategoryID:  majorityCategoryID(chapterItemsOf(articles)),
			Thumbnail:   firstNonEmptyThumbnail(articles),
			Status:      true,
			ContentType: contenttype.TypeNovel,
			CreateTime:  earliestCreateTime(articles),
		},
		ArticleDetail: entity.ArticleDetail{
			Keywords: articles[0].Keywords,
			Content:  buildNovelContent(articles),
			// 沿用第一章（多为简介页，按确认顺序）的 Extends：
			// wolves 小说侧栏的 language/file_size 等元数据存于此
			Extends: articles[0].Extends,
		},
	}

	if err := service.Article.Create(article); err != nil {
		return nil, fmt.Errorf("创建合并文章失败（原章节未删除）: %w", err)
	}
	if n.ctx != nil && n.ctx.Log != nil {
		n.ctx.Log.Info("创建合并文章成功",
			zap.Int("newID", article.ID), zap.String("title", article.Title),
			zap.Int("chapters", len(articles)))
	}

	// 逐篇删除原章节；单篇失败记录并继续
	res := &NovelMergeResult{
		NewID: article.ID, Title: article.Title, Slug: article.Slug,
		URL: article.URL(), ChapterCount: len(articles),
		FailedDeletes: make([]NovelMergeFailedDelete, 0),
	}
	for _, a := range articles {
		if err := service.Article.Delete(a.ID); err != nil {
			res.FailedDeletes = append(res.FailedDeletes,
				NovelMergeFailedDelete{ID: a.ID, Title: a.Title, Error: err.Error()})
			if n.ctx != nil && n.ctx.Log != nil {
				n.ctx.Log.Error("删除原章节文章失败（需手动处理）",
					zap.Int("id", a.ID), zap.String("title", a.Title), zap.Error(err))
			}
			continue
		}
		res.DeletedCount++
	}

	// 缓存清理：原文章 + 新文章 + 首页（原章节是单页文章，无 ?chapter= 惰性 key）
	if config.Config.Cache.Enable {
		for _, a := range articles {
			if err := cache.InvalidateArticleCache(a.URL()); err != nil && n.ctx != nil && n.ctx.Log != nil {
				n.ctx.Log.Warn("清理原文章缓存失败", zap.Int("id", a.ID), zap.Error(err))
			}
		}
		_ = cache.InvalidateArticleCache(article.URL())
		if err := cache.InvalidateHomePageCache(); err != nil && n.ctx != nil && n.ctx.Log != nil {
			n.ctx.Log.Warn("清理首页缓存失败", zap.Error(err))
		}
	}

	res.CostMS = time.Since(start).Milliseconds()
	if n.ctx != nil && n.ctx.Log != nil {
		n.ctx.Log.Info("小说合并完成",
			zap.String("keyword", req.Keyword), zap.Int("newID", res.NewID),
			zap.Int("chapters", res.ChapterCount), zap.Int("deleted", res.DeletedCount),
			zap.Int("failedDeletes", len(res.FailedDeletes)), zap.Int64("costMS", res.CostMS))
	}
	return res, nil
}

// matchArticles 按标题包含关键词查询（like 自动两侧包 %），只查 base 不拉正文
func (n *NovelMerger) matchArticles(keyword string) ([]entity.ArticleBase, error) {
	ctx := context.NewContext(novelMergeScanLimit+1, "id asc")
	ctx.Where = &context.Where{Field: "title", Operator: context.WhereOperatorLike, Value: keyword}
	return service.Article.List(ctx)
}

// loadArticlesInOrder 按 ids 顺序取完整文章（ListByIds 不保证按入参顺序，需重排）
func (n *NovelMerger) loadArticlesInOrder(ids []int) ([]entity.Article, error) {
	ctx := context.NewContext(len(ids), "id asc")
	bases, err := service.Article.ListByIds(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("查询文章失败: %w", err)
	}
	details, err := service.Article.ListDetailByIds(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("查询文章详情失败: %w", err)
	}
	byID := make(map[int]entity.Article, len(bases))
	for _, a := range service.Article.MergeBaseListAndDetailList(bases, details) {
		byID[a.ID] = a
	}
	articles := make([]entity.Article, 0, len(ids))
	var missing []int
	for _, id := range ids {
		if a, ok := byID[id]; ok {
			articles = append(articles, a)
			continue
		}
		missing = append(missing, id)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("以下文章已不存在: %v", missing)
	}
	return articles, nil
}

// parseNovelMergeRequest 解析并校验入参（keyword 必填、自定义正则可编译且有捕获组）
func parseNovelMergeRequest(raw string) (*novelMergeRequest, error) {
	var req novelMergeRequest
	if raw == "" {
		return nil, errors.New("请求体为空")
	}
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return nil, fmt.Errorf("解析请求体失败: %w", err)
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	if req.Keyword == "" {
		return nil, errors.New("请填写小说名关键词")
	}
	if _, err := compileChapterPattern(strings.TrimSpace(req.Pattern)); err != nil {
		return nil, err
	}
	req.Pattern = strings.TrimSpace(req.Pattern)
	return &req, nil
}

// chapterItemsOf 文章列表 → 预览章节项（majorityCategoryID 复用）
func chapterItemsOf(articles []entity.Article) []NovelMergeChapterItem {
	items := make([]NovelMergeChapterItem, 0, len(articles))
	for _, a := range articles {
		items = append(items, NovelMergeChapterItem{
			ID: a.ID, CreateTime: a.CreateTime, CategoryID: a.CategoryID,
		})
	}
	return items
}

func containsInt(list []int, v int) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func firstNonEmptyThumbnail(articles []entity.Article) string {
	for _, a := range articles {
		if strings.TrimSpace(a.Thumbnail) != "" {
			return a.Thumbnail
		}
	}
	return ""
}

func earliestCreateTime(articles []entity.Article) int64 {
	if len(articles) == 0 {
		return 0
	}
	min := articles[0].CreateTime
	for _, a := range articles[1:] {
		if a.CreateTime < min {
			min = a.CreateTime
		}
	}
	return min
}
