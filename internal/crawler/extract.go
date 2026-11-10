package crawler

import (
	"regexp"
	"strings"
	"unicode"

	"convertfix/internal/textutil"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

const chapterBody = "[itemprop=articleBody],.chapter-content,.chapter-c,.truyen,.reading-content,#chapter-c,#chapter-content"
const chapterChrome = "script,style,noscript,nav,footer,header,aside,iframe,form,button,select,.ads,.advertisement,.breadcrumb,.breadcrumbs,[aria-label=breadcrumb],.chapter-title,.current-book,.current-chapter,.chapter_control,.chapter-control,.chapter-nav,.chapter-navigation,.pagination,.control-last,.chap-info,.book-relate,#comment,#comments,[role=navigation],a[rel~=next],a[rel~=prev]"

var chapterHeading = regexp.MustCompile(`(?i)^(?:chương|chuong|chapter)\s*\d+`)

func Extract(doc *goquery.Document, source Source) string {
	// Work on a copy so repeated extraction and catalog parsing see the original DOM.
	root := doc.Selection.Clone()
	root.Find(chapterChrome).Remove()
	if source.Remove != "" {
		root.Find(source.Remove).Remove()
	}
	// This source injects an unlabelled promotion inside the chapter body.
	root.Find("div").Each(func(_ int, s *goquery.Selection) {
		promotion := strings.TrimSpace(s.ChildrenFiltered("p").Text())
		action := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s.Text()), promotion))
		if s.Children().Length() == 1 && s.ChildrenFiltered("p").Length() == 1 &&
			strings.HasPrefix(promotion, "Truyện chữ tặng bạn gói xem phim ") && action == "Nhận quà ngay!" {
			s.Remove()
		}
	})
	root.Find("a").Each(func(_ int, s *goquery.Selection) {
		if navigationLabel(s.Text()) {
			s.Remove()
		}
	})
	root.Find("h1,h2").Each(func(_ int, s *goquery.Selection) {
		if chapterHeading.MatchString(textutil.Normalize(s.Text())) {
			s.Remove()
		}
	})
	if source.Content != "" {
		return bodyText(root.Find(source.Content).First())
	}
	// A named chapter body wins over its larger page wrapper, even when the
	// wrapper contains more words from titles, navigation and recommendations.
	for _, selector := range []string{chapterBody, "article,main,#content"} {
		best, largest := "", 0
		root.Find(selector).Each(func(_ int, s *goquery.Selection) {
			if s.Find(selector).Length() > 0 {
				return
			}
			text := bodyText(s)
			if words := textutil.Words(text); words > largest {
				best, largest = text, words
			}
		})
		if best != "" {
			return best
		}
	}
	return ""
}

func navigationLabel(text string) bool {
	label := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsSpace(r) {
			return r
		}
		return -1
	}, strings.ToLower(textutil.Normalize(text)))
	label = strings.Join(strings.Fields(label), " ")
	switch label {
	case "chương trước", "chương sau", "chương tiếp", "chương tiếp theo", "chương kế", "chương kế tiếp", "chương đầu", "chương cuối", "mục lục", "danh sách chương", "trang trước", "trang sau", "trang tiếp", "previous chapter", "next chapter":
		return true
	}
	return false
}

func bodyText(s *goquery.Selection) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			// HTML whitespace inside a text node is not a paragraph boundary.
			b.WriteString(strings.Map(func(r rune) rune {
				if unicode.IsSpace(r) {
					return ' '
				}
				return r
			}, n.Data))
			return
		}
		block := false
		if n.Type == html.ElementNode {
			switch n.Data {
			case "br", "p", "div", "section", "article", "main", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "li":
				block = true
				b.WriteString("\n\n")
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if block {
			b.WriteString("\n\n")
		}
	}
	for _, node := range s.Nodes {
		walk(node)
	}
	paragraphs := textutil.Paragraphs(b.String())
	for i, p := range paragraphs {
		paragraphs[i] = strings.Join(strings.Fields(p), " ")
	}
	return strings.Join(paragraphs, "\n\n")
}
