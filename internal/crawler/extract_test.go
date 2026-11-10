package crawler

import (
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestWikidichBodyExcludesPageChrome(t *testing.T) {
	fixture, err := os.ReadFile("../../testdata/crawler/wikidich-chapter.html")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	want := "Sáng hôm ấy, người khách rời ngôi làng.\n\nAnh nhìn con đường phía trước và nói: “Chương trước của cuốn sách còn dang dở.”\n\nMột người bạn đưa tờ giấy ghi https://example.test và dặn anh giữ kỹ."
	before, _ := doc.Html()
	for _, selector := range []string{"", ".truyen", "#content"} {
		source := DefaultSource()
		source.Content = selector
		if got := Extract(doc, source); got != want {
			t.Fatalf("selector %q:\n%s", selector, got)
		}
	}
	after, _ := doc.Html()
	if before != after {
		t.Fatal("extract mutated the input document")
	}
}

func TestExtractionBoundariesAndConservativeLinkRemoval(t *testing.T) {
	cases := []struct{ name, html, want string }{
		{"specific body beats wrapper", `<main><h1>Book title</h1><p>Long wrapper text that must never win over the smaller chapter body.</p><article class="chapter-content"><p>Thân truyện.</p></article></main>`, "Thân truyện."},
		{"nested body beats parent", `<div class="reading-content">Outer title<div class="chapter-content">Thân truyện.</div>Outer footer</div>`, "Thân truyện."},
		{"fallback navigation", `<article><h1>Chương 7: Lên đường</h1><a href="/prev">《 Chương trước</a><p>Đoạn một.</p><a href="/next" rel="next">Tiếp tục đọc</a><p>Đoạn hai.</p></article>`, "Đoạn một.\n\nĐoạn hai."},
		{"inline and block boundaries", `<article>Đầu <em>câu</em>.<div>Đoạn hai.</div>Đoạn ba.<br>Đoạn bốn.</article>`, "Đầu câu.\n\nĐoạn hai.\n\nĐoạn ba.\n\nĐoạn bốn."},
		{"preserve prose and subheadings", `<article><h3>Buổi sáng</h3><p>Quảng cáo là nghề của anh.</p><p><a href="/reference">Chương trước kể về một hành trình.</a></p><p>Nguồn: lời người kể.</p></article>`, "Buổi sáng\n\nQuảng cáo là nghề của anh.\n\nChương trước kể về một hành trình.\n\nNguồn: lời người kể."},
		{"HTML source whitespace", "<article><p>Một\n\nngười khách.</p><p>Người thứ hai.</p></article>", "Một người khách.\n\nNgười thứ hai."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(tc.html))
			if err != nil {
				t.Fatal(err)
			}
			if got := Extract(doc, DefaultSource()); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExplicitContentSelectorDoesNotFallBack(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(`<article>Wrong body</article><div id="selected">Right body<span class="remove-me">unwanted</span></div>`))
	source := DefaultSource()
	source.Content, source.Remove = "#selected", ".remove-me"
	if got := Extract(doc, source); got != "Right body" {
		t.Fatal(got)
	}
	source.Content = "#missing"
	if got := Extract(doc, source); got != "" {
		t.Fatal("missing selector unexpectedly fell back", got)
	}
}
