package plugins

import "testing"

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
