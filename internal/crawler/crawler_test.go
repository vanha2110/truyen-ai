package crawler

import (
	"github.com/PuerkitoBio/goquery"
	"net/url"
	"strings"
	"testing"
)

func TestConfigurableHTMLExtraction(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<html><h1>Truyện thử</h1><a href="/chapter/1">Chương 1: Đầu tiên</a><a href="/chapter/2">Chương 2: Tiếp theo</a><a href="https://other.test/ad">Chương 3</a><a href="/login">Đăng nhập</a><article><p>Lâm Phong bước lên núi.</p><div class="ads">Quảng cáo rác</div><p>Hắn nhìn quanh.<br>Nàng vẫn ở đó.</p><script>alert(1)</script></article></html>`))
	if err != nil {
		t.Fatal(err)
	}
	base, _ := url.Parse("https://example.test/story")
	source := DefaultSource()
	links := ParseLinks(doc, base, source)
	if len(links) != 2 || links[1].Index != 2 || links[0].SourceURL != "https://example.test/chapter/1" {
		t.Fatal(links)
	}
	text := Extract(doc, source)
	if strings.Contains(text, "Quảng cáo") || strings.Contains(text, "alert") || !strings.Contains(text, "Lâm Phong") || !strings.Contains(text, "\n\n") {
		t.Fatal(text)
	}
}
func TestRejectNonWebURLs(t *testing.T) {
	for _, raw := range []string{"file:///C:/secret", "javascript:alert(1)", "http://user:pass@example.com"} {
		if _, err := parseURL(raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
