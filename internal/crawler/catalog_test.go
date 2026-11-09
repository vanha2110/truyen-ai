package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

func testCatalogServer(t *testing.T, pages map[string]string) (*Crawler, string, func() map[string]int) {
	t.Helper()
	var mu sync.Mutex
	requests := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests[r.URL.RequestURI()]++
		mu.Unlock()
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "User-agent: *\nAllow: /\n")
			return
		}
		body, ok := pages[r.URL.RequestURI()]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	crawler := New()
	crawler.http = server.Client()
	crawler.interval = 0
	counts := func() map[string]int {
		mu.Lock()
		defer mu.Unlock()
		out := map[string]int{}
		for k, v := range requests {
			out[k] = v
		}
		return out
	}
	return crawler, server.URL, counts
}
func TestAjaxCatalogWikidichShape(t *testing.T) {
	first, err := os.ReadFile("../../testdata/crawler/wikidich-page1.html")
	if err != nil {
		t.Fatal(err)
	}
	envelope := func(s string) string {
		b, err := json.Marshal(struct {
			Data string `json:"data"`
		}{s})
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	second := `<ul><li><a href="/story/chuong-4-four">Chương 4: Bốn</a></li><li><a href="/story/chuong-3-three">Chương 3: Ba</a></li><li><a href="/story/chuong-2-two?utm_source=duplicate#top">Chương 2: Lặp ở ranh giới trang</a></li></ul><div class="paging"><a href="javascript:void(0)" onclick="page(66163,1);">Đầu</a><a href="javascript:void(0)" onclick="page(66163,3);">3</a></div>`
	third := `<ul><li><a href="/story/chuong-5-five">Chương 5: Năm</a></li></ul><div class="paging"><a href="javascript:void(0)" onclick="page(66163,2);">Trước</a><a class="disabled" href="javascript:void(0)" onclick="page(66163,4);">Tiếp</a></div>`
	for _, selector := range []string{"a[href]", "#chapter-list a"} {
		t.Run(selector, func(t *testing.T) {
			c, base, counts := testCatalogServer(t, map[string]string{"/story": string(first), "/get/listchap/66163?page=2": envelope(second), "/get/listchap/66163?page=3": envelope(third)})
			source := DefaultSource()
			source.ChapterLinks = selector
			preview, err := c.Preview(context.Background(), base+"/story", source)
			if err != nil {
				t.Fatal(err)
			}
			if preview.Pages != 3 || len(preview.Chapters) != 5 {
				t.Fatalf("incomplete catalog: pages=%d chapters=%d", preview.Pages, len(preview.Chapters))
			}
			for i, ch := range preview.Chapters {
				if ch.Index != i+1 {
					t.Fatalf("wrong chapter order: %+v", preview.Chapters)
				}
			}
			if preview.Chapters[0].Title != "Chương 01: Mở đầu" {
				t.Fatalf("picked header link instead of catalog: %s", preview.Chapters[0].Title)
			}
			for path, count := range counts() {
				if count != 1 || strings.Contains(path, "page=1") || strings.Contains(path, "page=4") {
					t.Fatalf("duplicate or invalid request %s: %d", path, count)
				}
			}
		})
	}
}
func TestNumberedPaginationStartingFromMiddle(t *testing.T) {
	pager := `<nav class="pagination"><a href="?page=3">3</a><a href="?page=1">1</a><a href="?page=2">2</a><a href="https://foreign.test/story?page=4">4</a></nav>`
	pages := map[string]string{}
	for i := 1; i <= 3; i++ {
		pages[fmt.Sprintf("/story?page=%d", i)] = fmt.Sprintf(`<h1>Test</h1><a href="/story/chuong-%d">Chương %d</a>`, i, i) + pager
	}
	c, base, counts := testCatalogServer(t, pages)
	preview, err := c.Preview(context.Background(), base+"/story?page=2", DefaultSource())
	if err != nil {
		t.Fatal(err)
	}
	if preview.Pages != 3 || len(preview.Chapters) != 3 {
		t.Fatal(preview)
	}
	for i, ch := range preview.Chapters {
		if ch.Index != i+1 {
			t.Fatal(preview.Chapters)
		}
	}
	for _, n := range counts() {
		if n != 1 {
			t.Fatal("page fetched twice")
		}
	}
}
func TestUnnumberedChaptersAssignedAfterAllPages(t *testing.T) {
	pages := map[string]string{
		"/story":        `<h1>Test</h1><div class="entries"><a href="/story/a">Mở đầu</a><a href="/story/b">Ngày mới</a></div><a rel="next" href="?page=2">Tiếp</a>`,
		"/story?page=2": `<div class="entries"><a href="/story/b#top">Ngày mới</a><a href="/story/c">Người mới</a><a href="/story/d">Kết thúc</a></div><a rel="prev" href="/story">Trước</a>`,
	}
	c, base, _ := testCatalogServer(t, pages)
	source := DefaultSource()
	source.ChapterLinks = ".entries a"
	preview, err := c.Preview(context.Background(), base+"/story", source)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Chapters) != 4 || preview.Pages != 2 {
		t.Fatal(preview)
	}
	for i, ch := range preview.Chapters {
		if ch.Index != i+1 {
			t.Fatal(preview.Chapters)
		}
	}
}
func TestPaginationFailuresDoNotReturnSuccess(t *testing.T) {
	t.Run("missing page", func(t *testing.T) {
		c, base, _ := testCatalogServer(t, map[string]string{"/story": `<a href="/story/chuong-1">Chương 1</a><a rel="next" href="?page=2">Tiếp</a>`})
		if _, err := c.Preview(context.Background(), base+"/story", DefaultSource()); err == nil {
			t.Fatal("partial list silently accepted")
		}
	})
	t.Run("conflicting chapter", func(t *testing.T) {
		c, base, _ := testCatalogServer(t, map[string]string{"/story": `<a href="/story/chuong-1-a">Chương 1</a><a href="/story/chuong-1-b">Chương 1</a>`})
		p, err := c.Preview(context.Background(), base+"/story", DefaultSource())
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Chapters) != 2 || p.Numbering != SequentialNumbering || len(p.Warnings) == 0 {
			t.Fatal("source numbering conflict must be preserved and explained", p)
		}
		if p.Chapters[0].Index != 1 || p.Chapters[1].Index != 2 || p.Chapters[0].SourceURL == p.Chapters[1].SourceURL || p.Chapters[1].Flags == "" {
			t.Fatal(p.Chapters)
		}
	})
	t.Run("json empty", func(t *testing.T) {
		c, base, _ := testCatalogServer(t, map[string]string{"/story": `{"data":""}`})
		if _, err := c.Preview(context.Background(), base+"/story", DefaultSource()); err == nil {
			t.Fatal("empty AJAX response accepted")
		}
	})
}
func TestPaginationSelectAndDisabledNext(t *testing.T) {
	c, base, _ := testCatalogServer(t, map[string]string{
		"/story":        `<a href="/story/chuong-1">Chương 1</a><select name="page"><option value="2">2</option></select>`,
		"/story?page=2": `<a href="/story/chuong-2">Chương 2</a><div class="pagination"><span class="disabled"><a href="?page=3">Tiếp</a></span></div>`,
	})
	preview, err := c.Preview(context.Background(), base+"/story", DefaultSource())
	if err != nil {
		t.Fatal(err)
	}
	if preview.Pages != 2 || len(preview.Chapters) != 2 {
		t.Fatal(preview)
	}
}
func TestParseChapterNumbersAndCanonicalURLs(t *testing.T) {
	for title, want := range map[string]int{"Chương 01: Mở đầu": 1, "Chương 1.234: Tiếp": 1234, "/story/chuong-1367-abc": 1367} {
		if got := chapterIndex(title); got != want {
			t.Fatalf("%s: %d != %d", title, got, want)
		}
	}
	u, _ := url.Parse("https://example.test/story?z=1&utm_source=x&a=2#list")
	if got := canonicalURL(u); got != "https://example.test/story?a=2&z=1" {
		t.Fatal(got)
	}
}
