package plugins

import (
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// ts 构造本地时区的期望时间戳
func ts(y, mo, d, hh, mm, ss int) int64 {
	return time.Date(y, time.Month(mo), d, hh, mm, ss, 0, time.Local).Unix()
}

func TestParsePublishTime(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"2024-05-01 12:30:45", ts(2024, 5, 1, 12, 30, 45)},
		{"发布于 2024-05-01", ts(2024, 5, 1, 0, 0, 0)},
		{"2024/5/1", ts(2024, 5, 1, 0, 0, 0)},
		{"2024.05.01", ts(2024, 5, 1, 0, 0, 0)},
		{"2024年5月1日", ts(2024, 5, 1, 0, 0, 0)},
		{"2024年5月1日 08:05", ts(2024, 5, 1, 8, 5, 0)},
		{"更新: 2024-05-01 23:59:59 发布", ts(2024, 5, 1, 23, 59, 59)},
		{"no date here", 0},
		{"", 0},
		{"13月40日 9999-99-99", 0},                  // 非法月日不解析
		{"0002-05-01", 0},                         // 年份越界
		{"2024-02-30", 0},                         // 不存在的日期归一化失败
		{"992024-05-01", ts(2024, 5, 1, 0, 0, 0)}, // 从长数字串中截取合法日期
	}
	for _, c := range cases {
		if got := parsePublishTime(c.in); got != c.want {
			t.Errorf("parsePublishTime(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestBuildVideoSources(t *testing.T) {
	// 无标签:回退「第NN集」编号
	got := buildVideoSources([]string{"a.mp4", "b.mp4"}, nil, false, 0)
	if len(got) != 2 || got[0]["label"] != "第01集" || got[1]["label"] != "第02集" {
		t.Fatalf("fallback labels wrong: %v", got)
	}
	if got[0]["embed"] != false || got[0]["url"] != "a.mp4" {
		t.Fatalf("fields wrong: %v", got[0])
	}

	// 有标签:按下标对应,多余标签忽略,空标签回退编号
	got = buildVideoSources([]string{"a.mp4", "b.mp4"}, []string{"正片", "", "多余"}, false, 0)
	if got[0]["label"] != "正片" || got[1]["label"] != "第02集" {
		t.Fatalf("label pairing wrong: %v", got)
	}

	// idxOffset 跨通道续编号;embed 标记透传
	got = buildVideoSources([]string{"c.html"}, nil, true, 2)
	if got[0]["label"] != "第03集" || got[0]["embed"] != true {
		t.Fatalf("offset/embed wrong: %v", got)
	}

	// 空输入返回空切片
	if got := buildVideoSources(nil, nil, false, 0); len(got) != 0 {
		t.Fatalf("empty input should give empty slice, got %v", got)
	}
}

func TestParseCookies(t *testing.T) {
	// 空/空白配置:无 cookie 无错误
	if cs, err := parseCookies("", "https://a.tv/"); cs != nil || err != nil {
		t.Fatalf("empty config: cs=%v err=%v", cs, err)
	}

	cs, err := parseCookies(
		`[{"name":"token","value":"abc"},{"name":"uid","value":"9","domain":".example.tv","path":"/v"}]`,
		"https://www.a.tv/list")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(cs) != 2 {
		t.Fatalf("want 2 cookies, got %d", len(cs))
	}
	if cs[0].Name != "token" || cs[0].Value != "abc" {
		t.Fatalf("cookie0 wrong: %+v", cs[0])
	}
	if cs[0].URL != "https://www.a.tv/list" {
		t.Fatalf("cookie0 should bind to page URL, got %q", cs[0].URL)
	}
	if cs[1].Domain != ".example.tv" || cs[1].Path != "/v" {
		t.Fatalf("cookie1 domain/path wrong: %+v", cs[1])
	}

	// 非法 JSON 报错;缺 name 报错
	if _, err := parseCookies(`{bad`, "https://a.tv/"); err == nil {
		t.Fatal("invalid json should error")
	}
	if _, err := parseCookies(`[{"value":"x"}]`, "https://a.tv/"); err == nil {
		t.Fatal("missing name should error")
	}
	_ = proto.NetworkCookieParam{} // 保持 proto 依赖(实现返回类型)
}

func TestTaskDedupSlug(t *testing.T) {
	def := &spiderTask{}
	if got := def.dedupSlug("https://a/1", "标题"); got != hashSlug("https://a/1", "标题") {
		t.Fatalf("default dedup should hash url+title, got %q", got)
	}
	if def.dedupSlug("https://a/1", "标题") == def.dedupSlug("https://a/1", "改标题") {
		t.Fatal("default mode: title change should change slug")
	}

	byURL := &spiderTask{DedupBy: "url"}
	s1 := byURL.dedupSlug("https://a/1", "标题")
	s2 := byURL.dedupSlug("https://a/1", "别的标题")
	if s1 != s2 {
		t.Fatalf("url mode: title change should keep slug, %q vs %q", s1, s2)
	}
	if s1 == def.dedupSlug("https://a/1", "标题") {
		t.Fatal("url mode slug should differ from url_title mode slug")
	}

	// 显式 url_title 与默认一致
	expl := &spiderTask{DedupBy: "url_title"}
	if expl.dedupSlug("https://a/1", "标题") != hashSlug("https://a/1", "标题") {
		t.Fatal("explicit url_title should match default")
	}
}

func TestBuildLinkURL(t *testing.T) {
	cases := []struct{ template, value, want string }{
		{"", "https://a.tv/video/1", "https://a.tv/video/1"},                  // 无模板原样返回
		{"https://a.tv/video/{value}", "155598", "https://a.tv/video/155598"}, // {value} 占位替换
		{"https://a.tv/video/", "155598", "https://a.tv/video/155598"},        // 无占位符视为前缀拼接
		{"https://a.tv/v/{value}.html", "12", "https://a.tv/v/12.html"},       // 占位在中间
		{"https://a.tv/video/{value}", "", "https://a.tv/video/"},             // 空值照常替换（上游会先拦空值）
	}
	for _, c := range cases {
		if got := buildLinkURL(c.template, c.value); got != c.want {
			t.Errorf("buildLinkURL(%q, %q) = %q, want %q", c.template, c.value, got, c.want)
		}
	}
}

func TestCompileLinkRegex(t *testing.T) {
	// /re/ 包裹与裸正则等价
	re, err := compileLinkRegex(`/video/(\d+)/`)
	if err != nil {
		t.Fatalf("compile /re/ wrapped: %v", err)
	}
	if m := re.FindStringSubmatch(`location.href='/video/123'`); m == nil || m[1] != "123" {
		t.Errorf("capture group want 123, got %v", m)
	}
	re2, err := compileLinkRegex(`\d+`)
	if err != nil {
		t.Fatalf("compile bare: %v", err)
	}
	if m := re2.FindStringSubmatch("abc456"); m == nil || m[0] != "456" {
		t.Errorf("whole match want 456, got %v", m)
	}
	if _, err := compileLinkRegex(`(`); err == nil {
		t.Error("invalid pattern should return error")
	}
}

func TestSameSite(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"example.tv", "example.tv", true},
		{"www.example.tv", "example.tv", true},   // www 前缀
		{"m.example.tv", "example.tv", true},     // 子域
		{"example.tv", "www.example.tv", true},   // 反向
		{"WWW.Example.TV", "example.tv", true},   // 大小写
		{"notexample.tv", "example.tv", false},   // 后缀碰瓷不算同站
		{"evil-example.tv", "example.tv", false}, // 连字符不算
		{"a.x.com", "b.x.com", false},            // 平级子域互不认同
		{"ad.com", "example.tv", false},
	}
	for _, c := range cases {
		if got := sameSite(c.a, c.b); got != c.want {
			t.Errorf("sameSite(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
