package crawler

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"convertfix/internal/domain"
	"convertfix/internal/textutil"
	"github.com/PuerkitoBio/goquery"
)

const maxCatalogPages = 100
const maxCatalogChapters = 10000
const paginationContainers = `.pagination,.pager,.paging,.paginate,.phan-trang,[data-pagination],nav[aria-label="Pagination"],nav[aria-label="pagination"],nav[aria-label="Phân trang"]`

var chapterNumber = regexp.MustCompile(`(?i)(?:chương|chuong|chapter)[\s:#._-]*(\d+(?:[.,]\d{3})*)(?:\b|[^\d])`)
var pageSuffix = regexp.MustCompile(`(?i)/(?:page|trang)(?:/|-|_)(\d+)/?$`)

func chapterIndex(s string) int {
	match := chapterNumber.FindStringSubmatch(textutil.Normalize(s))
	if len(match) < 2 {
		return 0
	}
	digits := strings.NewReplacer(".", "", ",", "").Replace(match[1])
	index, err := strconv.Atoi(digits)
	if err != nil || index < 1 {
		return 0
	}
	return index
}
func canonicalURL(u *url.URL) string {
	copy := *u
	copy.Fragment = ""
	copy.RawFragment = ""
	copy.Host = strings.ToLower(copy.Host)
	copy.Scheme = strings.ToLower(copy.Scheme)
	query := copy.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "fbclid" || lower == "gclid" {
			query.Del(key)
		}
	}
	copy.RawQuery = query.Encode()
	return copy.String()
}
func resolveLink(base *url.URL, href string) (*url.URL, bool) {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return nil, false
	}
	ref, err := url.Parse(href)
	if err != nil {
		return nil, false
	}
	u := base.ResolveReference(ref)
	if u.User != nil || u.Scheme != base.Scheme || !strings.EqualFold(u.Host, base.Host) {
		return nil, false
	}
	normalized, err := url.Parse(canonicalURL(u))
	return normalized, err == nil
}
func isDisabled(s *goquery.Selection) bool {
	disabled := false
	s.AddSelection(s.Parents()).Each(func(_ int, node *goquery.Selection) {
		aria, _ := node.Attr("aria-disabled")
		_, attribute := node.Attr("disabled")
		if attribute || strings.EqualFold(aria, "true") || node.HasClass("disabled") {
			disabled = true
		}
	})
	return disabled
}
func chapterSelection(doc *goquery.Document, selector string) *goquery.Selection {
	if selector != "a[href]" {
		return doc.Find(selector)
	}
	var best *goquery.Selection
	score := 0
	doc.Find(`.chapter-list,.list-chapter,.list-chapters,.chapters-list,#chapter-list,#list-chapter,.listchap,[data-role="chapters"]`).Each(func(_ int, container *goquery.Selection) {
		count := 0
		container.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
			href, _ := a.Attr("href")
			if chapterIndex(a.Text()) > 0 || chapterIndex(href) > 0 {
				count++
			}
		})
		if count > score {
			best = container
			score = count
		}
	})
	if best != nil {
		return best.Find("a[href]")
	}
	return doc.Find(selector)
}

// Unnumbered links keep index zero until all catalog pages have been merged.
func ParseLinks(doc *goquery.Document, base *url.URL, source Source) []domain.Chapter {
	chapters := []domain.Chapter{}
	seen := map[string]bool{}
	chapterSelection(doc, source.ChapterLinks).Each(func(_ int, s *goquery.Selection) {
		if isDisabled(s) || s.Closest(paginationContainers).Length() > 0 {
			return
		}
		href, ok := s.Attr("href")
		if !ok {
			return
		}
		u, ok := resolveLink(base, href)
		if !ok || canonicalURL(u) == canonicalURL(base) || seen[u.String()] {
			return
		}
		title := textutil.Normalize(s.Text())
		if title == "" {
			title = textutil.Normalize(s.AttrOr("title", ""))
		}
		index := chapterIndex(title)
		if index == 0 {
			index = chapterIndex(u.Path)
		}
		if index == 0 && source.ChapterLinks == "a[href]" {
			return
		}
		seen[u.String()] = true
		chapters = append(chapters, domain.Chapter{Index: index, Title: title, SourceURL: u.String(), Status: domain.Pending})
	})
	return chapters
}

type catalogPage struct {
	url    *url.URL
	number int
	order  int
}
type ajaxPagination struct{ prefix, id, container string }

var ajaxCall = regexp.MustCompile(`^\s*(?:return\s+)?page\(\s*(\d+)\s*,\s*(\d+)\s*\)\s*;?\s*$`)
var ajaxFunction = regexp.MustCompile(`function\s+page\s*\(\s*([A-Za-z_$][\w$]*)\s*,\s*([A-Za-z_$][\w$]*)\s*\)\s*\{`)
var ajaxEndpoint = regexp.MustCompile(`url\s*:\s*["'](/[^"'\s]*)["']\s*\+\s*([A-Za-z_$][\w$]*)\s*\+\s*["']\?page=["']\s*\+\s*([A-Za-z_$][\w$]*)`)
var ajaxContainer = regexp.MustCompile(`\$\(\s*["']#([A-Za-z][\w-]*)["']\s*\)\.html\(`)

// Read a narrow, literal GET URL pattern; never evaluate JavaScript from a source.
func detectAjaxPagination(doc *goquery.Document) *ajaxPagination {
	id := ""
	doc.Find(paginationContainers).Find("[onclick]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		match := ajaxCall.FindStringSubmatch(s.AttrOr("onclick", ""))
		if len(match) == 3 {
			id = match[1]
			return false
		}
		return true
	})
	if id == "" {
		return nil
	}
	var result *ajaxPagination
	doc.Find("script:not([src])").EachWithBreak(func(_ int, script *goquery.Selection) bool {
		body := script.Text()
		fn := ajaxFunction.FindStringSubmatchIndex(body)
		if len(fn) == 0 {
			return true
		}
		fragment := body[fn[1]:]
		if len(fragment) > 8192 {
			fragment = fragment[:8192]
		}
		endpoint := ajaxEndpoint.FindStringSubmatch(fragment)
		if len(endpoint) != 4 || endpoint[2] != body[fn[2]:fn[3]] || endpoint[3] != body[fn[4]:fn[5]] {
			return true
		}
		container := ""
		if match := ajaxContainer.FindStringSubmatch(fragment); len(match) == 2 {
			container = match[1]
		}
		result = &ajaxPagination{prefix: endpoint[1], id: id, container: container}
		return false
	})
	return result
}
func (a *ajaxPagination) pageURL(base *url.URL, page int) (*url.URL, bool) {
	return resolveLink(base, a.prefix+a.id+"?page="+strconv.Itoa(page))
}
func pageNumber(u *url.URL) int {
	for key, values := range u.Query() {
		switch strings.ToLower(key) {
		case "page", "p", "trang", "pageindex", "pagenumber":
			if len(values) > 0 {
				n, err := strconv.Atoi(values[0])
				if err == nil && n > 0 {
					return n
				}
			}
		}
	}
	match := pageSuffix.FindStringSubmatch(u.Path)
	if len(match) > 1 {
		n, _ := strconv.Atoi(match[1])
		return n
	}
	return 0
}
func catalogPath(u *url.URL) string {
	path := strings.TrimRight(pageSuffix.ReplaceAllString(u.Path, ""), "/")
	for _, suffix := range []string{"/danh-sach-chuong", "/chapters", "/muc-luc"} {
		path = strings.TrimSuffix(path, suffix)
	}
	return path
}
func sameCatalog(a, b *url.URL) bool {
	if !strings.EqualFold(a.Host, b.Host) || a.Scheme != b.Scheme || catalogPath(a) != catalogPath(b) {
		return false
	}
	// Query-based novel identifiers must stay fixed while only pagination changes.
	for key, values := range a.Query() {
		switch strings.ToLower(key) {
		case "id", "novel", "novelid", "novel_id", "story", "storyid", "story_id":
			if b.Query().Get(key) != strings.Join(values, ",") {
				return false
			}
		}
	}
	return true
}
func paginationLinks(doc *goquery.Document, current, root *url.URL, source Source, ajax *ajaxPagination) ([]catalogPage, bool) {
	pages := []catalogPage{}
	seen := map[string]bool{}
	hasControls := false
	add := func(s *goquery.Selection, href string, explicit bool) {
		if isDisabled(s) {
			return
		}
		u, ok := resolveLink(current, href)
		if !ok || seen[u.String()] {
			return
		}
		if !explicit && (!sameCatalog(root, u) || chapterIndex(u.Path) > 0) {
			return
		}
		number := pageNumber(u)
		if number == 0 {
			number, _ = strconv.Atoi(strings.TrimSpace(s.Text()))
		}
		if number == 0 && catalogPath(u) == strings.TrimRight(u.Path, "/") {
			number = 1
		}
		seen[u.String()] = true
		pages = append(pages, catalogPage{url: u, number: number})
	}
	if source.Next != "" {
		doc.Find(source.Next).Each(func(_ int, s *goquery.Selection) {
			if isDisabled(s) {
				return
			}
			hasControls = true
			href, ok := s.Attr("href")
			if !ok {
				href = s.AttrOr("data-url", s.AttrOr("value", ""))
			}
			add(s, href, true)
		})
	}
	doc.Find(paginationContainers).Each(func(_ int, container *goquery.Selection) {
		hasControls = true
		container.Find("a[href]").Each(func(_ int, a *goquery.Selection) { add(a, a.AttrOr("href", ""), false) })
	})
	doc.Find(`a[rel],link[rel],a[href]`).Each(func(_ int, a *goquery.Selection) {
		href := a.AttrOr("href", "")
		u, ok := resolveLink(current, href)
		if !ok {
			return
		}
		rel := strings.Fields(strings.ToLower(a.AttrOr("rel", "")))
		navigation := false
		for _, value := range rel {
			if value == "next" || value == "prev" || value == "first" || value == "last" {
				navigation = true
			}
		}
		if navigation || (pageNumber(u) > 0 && sameCatalog(root, u) && chapterIndex(u.Path) == 0) {
			hasControls = true
			add(a, href, false)
		}
	})
	doc.Find("select").Each(func(_ int, selectNode *goquery.Selection) {
		name := selectNode.AttrOr("name", "")
		lower := strings.ToLower(name)
		inPager := selectNode.Closest(paginationContainers).Length() > 0
		if !inPager && lower != "page" && lower != "trang" {
			return
		}
		hasControls = true
		selectNode.Find("option[value]").Each(func(_ int, option *goquery.Selection) {
			value := strings.TrimSpace(option.AttrOr("value", ""))
			if n, err := strconv.Atoi(value); err == nil && n > 0 {
				target := *current
				query := target.Query()
				key := name
				if key == "" {
					key = "page"
				}
				query.Set(key, strconv.Itoa(n))
				target.RawQuery = query.Encode()
				add(option, target.String(), false)
			} else {
				add(option, value, false)
			}
		})
	})
	if ajax != nil {
		doc.Find(paginationContainers).Find("[onclick]").Each(func(_ int, s *goquery.Selection) {
			if isDisabled(s) {
				return
			}
			match := ajaxCall.FindStringSubmatch(s.AttrOr("onclick", ""))
			if len(match) != 3 || match[1] != ajax.id {
				return
			}
			number, err := strconv.Atoi(match[2])
			if err != nil || number < 1 {
				return
			}
			u, ok := ajax.pageURL(root, number)
			if !ok {
				return
			}
			hasControls = true
			add(s, u.String(), true)
		})
	}
	sort.SliceStable(pages, func(i, j int) bool { return pages[i].number < pages[j].number })
	return pages, hasControls
}

type catalogChapter struct {
	chapter   domain.Chapter
	page      int
	discovery int
	position  int
}

func (c *Crawler) Preview(ctx context.Context, raw string, source Source) (Preview, error) {
	out := Preview{Chapters: []domain.Chapter{}, Warnings: []string{}, Numbering: SourceNumbering}
	base, err := parseURL(raw)
	if err != nil {
		return out, err
	}
	base, err = url.Parse(canonicalURL(base))
	if err != nil {
		return out, err
	}
	if strings.TrimSpace(source.ChapterLinks) == "" {
		source.ChapterLinks = "a[href]"
	}
	firstNumber := pageNumber(base)
	if firstNumber == 0 {
		firstNumber = 1
	}
	pending := []catalogPage{{url: base, number: firstNumber}}
	queued := map[string]bool{base.String(): true}
	seenLinks := map[string]bool{}
	chapters := []catalogChapter{}
	unresolvedPagination := false
	var ajax *ajaxPagination
	for len(pending) > 0 {
		if err = ctx.Err(); err != nil {
			return out, err
		}
		if out.Pages >= maxCatalogPages {
			return out, fmt.Errorf("Mục lục vượt giới hạn %d trang; chưa trả danh sách thiếu", maxCatalogPages)
		}
		page := pending[0]
		pending = pending[1:]
		doc, err := c.fetch(ctx, page.url.String())
		if err != nil {
			return out, fmt.Errorf("Đọc trang mục lục %s: %w", page.url.String(), err)
		}
		out.Pages++
		if out.Pages == 1 {
			out.Novel = domain.Novel{Title: textutil.Normalize(doc.Find(source.Title).First().Text()), Author: textutil.Normalize(doc.Find(source.Author).First().Text()), Description: textutil.Normalize(doc.Find(source.Description).First().Text()), SourceURL: base.String()}
			if out.Novel.Title == "" {
				out.Novel.Title = textutil.Normalize(doc.Find("title").Text())
			}
			ajax = detectAjaxPagination(doc)
			if ajax != nil {
				if alias, ok := ajax.pageURL(base, page.number); ok {
					queued[alias.String()] = true
				}
			}
		}
		if ajax != nil && ajax.container != "" && doc.Find("#"+ajax.container).Length() == 0 {
			doc.Find("body").WrapInnerHtml(`<div id="` + ajax.container + `"></div>`)
		}
		for position, ch := range ParseLinks(doc, page.url, source) {
			if seenLinks[ch.SourceURL] {
				continue
			}
			seenLinks[ch.SourceURL] = true
			chapters = append(chapters, catalogChapter{chapter: ch, page: page.number, discovery: page.order, position: position})
			if len(chapters) > maxCatalogChapters {
				return out, fmt.Errorf("Mục lục vượt %d chương", maxCatalogChapters)
			}
		}
		links, controls := paginationLinks(doc, page.url, base, source, ajax)
		if controls && len(links) == 0 {
			unresolvedPagination = true
		}
		for _, next := range links {
			if queued[next.url.String()] {
				continue
			}
			queued[next.url.String()] = true
			next.order = len(queued)
			if next.number == 0 {
				next.number = page.number + 1
			}
			pending = append(pending, next)
		}
		sort.SliceStable(pending, func(i, j int) bool { return pending[i].number < pending[j].number })
	}
	if len(chapters) == 0 {
		return out, errors.New("Không tìm thấy chương; hãy nhập selector liên kết chương hoặc dùng nhập văn bản")
	}
	if warning := c.robotsWarning(base); warning != "" {
		out.Warnings = append(out.Warnings, warning)
	}
	sort.SliceStable(chapters, func(i, j int) bool {
		a, b := chapters[i], chapters[j]
		if a.page != b.page {
			return a.page < b.page
		}
		if a.discovery != b.discovery {
			return a.discovery < b.discovery
		}
		return a.position < b.position
	})
	indexes := map[int]string{}
	duplicateNumbers := map[int]bool{}
	for _, item := range chapters {
		ch := item.chapter
		if ch.Index == 0 {
			continue
		}
		if _, exists := indexes[ch.Index]; exists {
			duplicateNumbers[ch.Index] = true
		}
		indexes[ch.Index] = ch.SourceURL
	}
	if len(duplicateNumbers) > 0 {
		out.Numbering = SequentialNumbering
		numbers := []int{}
		for number := range duplicateNumbers {
			numbers = append(numbers, number)
		}
		sort.Ints(numbers)
		out.Warnings = append(out.Warnings, fmt.Sprintf("Nguồn dùng trùng số chương %v cho các URL khác nhau. Đã giữ mọi chương theo thứ tự mục lục và dùng số thứ tự 1–%d trong app; tiêu đề và URL nguồn không đổi.", numbers, len(chapters)))
	}
	nextIndex := 1
	unnumbered := false
	for position, item := range chapters {
		ch := item.chapter
		if out.Numbering == SequentialNumbering {
			original := ch.Index
			ch.Index = position + 1
			if original != ch.Index {
				ch.Flags = fmt.Sprintf("source_chapter_number=%d; catalog_position=%d", original, ch.Index)
			}
		} else if ch.Index == 0 {
			unnumbered = true
			for indexes[nextIndex] != "" {
				nextIndex++
			}
			ch.Index = nextIndex
			indexes[ch.Index] = ch.SourceURL
		}
		nextIndex = ch.Index + 1
		if ch.Title == "" {
			ch.Title = fmt.Sprintf("Chương %d", ch.Index)
		}
		out.Chapters = append(out.Chapters, ch)
	}
	sort.Slice(out.Chapters, func(i, j int) bool { return out.Chapters[i].Index < out.Chapters[j].Index })
	if unnumbered {
		out.Warnings = append(out.Warnings, "Một số liên kết không có số chương; đã đánh số liên tục theo thứ tự mục lục sau khi gộp các trang. Hãy kiểm tra thứ tự trước khi thêm.")
	}
	if unresolvedPagination && out.Pages == 1 {
		out.Warnings = append(out.Warnings, "Trang có điều khiển phân trang nhưng không có URL trang có thể đọc. Nguồn có thể tải mục lục bằng JavaScript; hãy cấu hình selector hoặc dùng nhập văn bản.")
	}
	for i, ch := range out.Chapters {
		if i > 0 && ch.Index != out.Chapters[i-1].Index+1 {
			out.Warnings = append(out.Warnings, fmt.Sprintf("Thiếu chương giữa %d và %d", out.Chapters[i-1].Index, ch.Index))
		}
	}
	return out, nil
}
