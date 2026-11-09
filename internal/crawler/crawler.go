package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"convertfix/internal/domain"
	"convertfix/internal/textutil"
	"github.com/PuerkitoBio/goquery"
)

const agent = "ConvertFix/0.1 (+local personal reader)"

type Source struct {
	Title        string `json:"title"`
	Author       string `json:"author"`
	Description  string `json:"description"`
	ChapterLinks string `json:"chapterLinks"`
	Content      string `json:"content"`
	Next         string `json:"next"`
	Remove       string `json:"remove"`
}

func DefaultSource() Source {
	return Source{Title: "h1", Author: "[itemprop=author]", Description: "[itemprop=description]", ChapterLinks: "a[href]", Content: "", Next: "", Remove: "script,style,nav,footer,header,aside,iframe,form,button,.ads,.advertisement"}
}

type Preview struct {
	Novel     domain.Novel     `json:"novel"`
	Chapters  []domain.Chapter `json:"chapters"`
	Warnings  []string         `json:"warnings"`
	Pages     int              `json:"pages"`
	Numbering CatalogNumbering `json:"numbering"`
}
type CatalogNumbering string

const (
	SourceNumbering     CatalogNumbering = "source"
	SequentialNumbering CatalogNumbering = "catalog"
)

type Crawler struct {
	http     *http.Client
	mu       sync.Mutex
	last     time.Time
	robots   map[string]robotsEntry
	interval time.Duration
}

func New() *Crawler {
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	transport := &http.Transport{MaxConnsPerHost: 2, ResponseHeaderTimeout: 30 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, errors.New("Không tìm thấy địa chỉ nguồn")
		}
		for _, ip := range ips {
			if ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() || ip.IP.IsLinkLocalMulticast() || ip.IP.IsUnspecified() || ip.IP.IsMulticast() {
				return nil, errors.New("Crawler chỉ truy cập website công khai")
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}}
	return &Crawler{http: &http.Client{Transport: transport, Timeout: 45 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("Quá nhiều redirect")
		}
		if (req.URL.Scheme != "http" && req.URL.Scheme != "https") || req.URL.User != nil {
			return errors.New("Redirect không hợp lệ")
		}
		if len(via) == 0 {
			return nil
		}
		if via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
			return errors.New("Không chuyển kết nối HTTPS xuống HTTP")
		}
		if req.URL.Host != via[0].URL.Host && via[0].URL.Path != "/robots.txt" {
			return errors.New("Website chuyển sang domain khác; hãy nhập URL đích trực tiếp")
		}
		return nil
	}}, robots: map[string]robotsEntry{}, interval: 1500 * time.Millisecond}
}
func parseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return nil, errors.New("Nguồn phải là URL http(s) công khai")
	}
	return u, nil
}
func (c *Crawler) request(ctx context.Context, u *url.URL) ([]byte, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delay := time.Until(c.last.Add(c.interval))
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-timer.C:
		}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", agent)
	req.Header.Set("Accept", "text/html,text/plain;q=0.9")
	c.last = time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, explainTransportError(u, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if len(body) > 8<<20 {
		return nil, resp.StatusCode, errors.New("Trang vượt giới hạn 8 MB")
	}
	return body, resp.StatusCode, nil
}
func (c *Crawler) fetch(ctx context.Context, raw string) (*goquery.Document, error) {
	u, err := parseURL(raw)
	if err != nil {
		return nil, err
	}
	if err := c.checkRobots(ctx, u); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		b, status, err := c.request(ctx, u)
		var certificateErr *CertificateError
		if errors.As(err, &certificateErr) {
			return nil, err
		}
		if err == nil && status >= 200 && status < 300 {
			if strings.HasPrefix(strings.TrimSpace(string(b)), "{") {
				var envelope struct {
					Data string `json:"data"`
				}
				if err = json.Unmarshal(b, &envelope); err != nil {
					return nil, fmt.Errorf("Phản hồi mục lục JSON không hợp lệ: %w", err)
				}
				if strings.TrimSpace(envelope.Data) == "" {
					return nil, errors.New("Phản hồi mục lục JSON không có nội dung data")
				}
				b = []byte(envelope.Data)
			}
			return goquery.NewDocumentFromReader(strings.NewReader(string(b)))
		}
		if err == nil && (status == 403 || status == 429) {
			return nil, fmt.Errorf("Website trả HTTP %d; dừng tải để tránh gây thêm yêu cầu", status)
		}
		if err == nil && status < 500 {
			return nil, fmt.Errorf("Website trả HTTP %d", status)
		}
		if attempt == 2 {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("Website trả HTTP %d", status)
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, errors.New("Không tải được trang")
}

func Extract(doc *goquery.Document, source Source) string {
	doc.Find(source.Remove).Remove()
	var content *goquery.Selection
	if source.Content != "" {
		content = doc.Find(source.Content).First()
	} else {
		largest := 0
		doc.Find("article,main,[itemprop=articleBody],.chapter-content,.chapter-c,.truyen,.reading-content,#content,#chapter-c").Each(func(_ int, s *goquery.Selection) {
			n := textutil.Words(s.Text())
			if n > largest {
				largest = n
				content = s
			}
		})
	}
	if content == nil || content.Length() == 0 {
		return ""
	}
	content.Find("br").ReplaceWithHtml("\n\n")
	content.Find("p,div,section").Each(func(_ int, s *goquery.Selection) { s.AppendHtml("\n\n") })
	lines := []string{}
	for _, p := range textutil.Paragraphs(content.Text()) {
		p = strings.Join(strings.Fields(p), " ")
		low := strings.ToLower(p)
		if strings.HasPrefix(low, "nguồn:") || strings.HasPrefix(low, "quảng cáo") || strings.Contains(low, "https://") || strings.Contains(low, "http://") {
			continue
		}
		lines = append(lines, p)
	}
	return strings.Join(lines, "\n\n")
}
func (c *Crawler) Chapter(ctx context.Context, raw string, source Source) (string, error) {
	doc, err := c.fetch(ctx, raw)
	if err != nil {
		return "", err
	}
	text := Extract(doc, source)
	if textutil.Words(text) < 30 {
		return "", errors.New("Nội dung quá ngắn hoặc không tìm thấy; kiểm tra selector, trang có thể cần JavaScript")
	}
	return text, nil
}
