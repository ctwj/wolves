package plugins

import "testing"

// 回归：dedup_by=title 曾被校验器误拒（条件漏加 title 分支）
func TestDedupByTitleAccepted(t *testing.T) {
	httpT := &httpTask{Name: "t", SourceURL: "https://a.com", ListSelector: ".l", Mode: "detail", DedupBy: "title"}
	if err := normalizeHTTPTask(httpT, "任务[1]"); err != nil {
		t.Fatalf("httpTask dedup_by=title 被误拒: %v", err)
	}
	if got := httpT.dedupSlug("https://a.com/x?ts=1", "标题A"); got != hashSlug("", "标题A") {
		t.Fatalf("httpTask title 模式键不符: %s", got)
	}
	spiderT := &spiderTask{Name: "t", SourceURL: "https://a.com", ListSelector: ".l", Mode: "detail", DedupBy: "title"}
	if err := normalizeSpiderTask(spiderT, "任务[1]"); err != nil {
		t.Fatalf("spiderTask dedup_by=title 被误拒: %v", err)
	}
}
