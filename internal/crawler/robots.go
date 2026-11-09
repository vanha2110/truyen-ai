package crawler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/temoto/robotstxt"
)

type robotsEntry struct {
	rules     *robotstxt.RobotsData
	expiresAt time.Time
	warning   string
}

type CertificateError struct {
	Host, Issuer string
	Cause        error
}

func (e *CertificateError) Error() string {
	issuer := ""
	if e.Issuer != "" {
		issuer = " (CA: " + e.Issuer + ")"
	}
	return fmt.Sprintf("Không xác minh được chứng chỉ HTTPS của %s%s. Kiểm tra chuỗi chứng chỉ của website hoặc CA do proxy/tường lửa mạng sử dụng. Đây là lỗi TLS trước khi đọc robots.txt/nội dung web, không phải quy tắc Disallow. Chi tiết: %v", e.Host, issuer, e.Cause)
}
func (e *CertificateError) Unwrap() error { return e.Cause }
func explainTransportError(u *url.URL, err error) error {
	var verification *tls.CertificateVerificationError
	if !errors.As(err, &verification) {
		return err
	}
	issuer := ""
	if len(verification.UnverifiedCertificates) > 0 {
		cert := verification.UnverifiedCertificates[0]
		issuer = strings.Join(cert.Issuer.Organization, ", ")
		if issuer == "" {
			issuer = cert.Issuer.CommonName
		}
	}
	return &CertificateError{Host: u.Hostname(), Issuer: issuer, Cause: err}
}
func (c *Crawler) checkRobots(ctx context.Context, u *url.URL) error {
	origin := u.Scheme + "://" + u.Host
	c.mu.Lock()
	entry, known := c.robots[origin]
	c.mu.Unlock()
	if !known || time.Now().After(entry.expiresAt) {
		robotsURL := *u
		robotsURL.Path = "/robots.txt"
		robotsURL.RawPath = ""
		robotsURL.RawQuery = ""
		robotsURL.Fragment = ""
		robotsURL.RawFragment = ""
		body, status, err := c.request(ctx, &robotsURL)
		if err != nil {
			return fmt.Errorf("Không đọc được robots.txt: %w", err)
		}
		if status == 429 {
			return errors.New("robots.txt trả HTTP 429: nguồn đang giới hạn yêu cầu. Đợi một lúc rồi thử lại")
		}
		if status >= 500 {
			return fmt.Errorf("robots.txt trả HTTP %d: máy chủ nguồn đang lỗi. Tạm dừng tải và thử lại sau", status)
		}
		rules, err := robotstxt.FromStatusAndBytes(status, body)
		if err != nil {
			return fmt.Errorf("Không phân tích được robots.txt (HTTP %d): %w", status, err)
		}
		entry = robotsEntry{rules: rules, expiresAt: time.Now().Add(24 * time.Hour)}
		// RFC 9309 §2.3.1.3: an HTTP 4xx response is an unavailable robots file, not a Disallow rule.
		if status >= 400 && status < 500 && status != 404 && status != 410 {
			entry.warning = fmt.Sprintf("robots.txt không khả dụng (HTTP %d); nguồn không trả quy tắc crawl. App tiếp tục tải các trang công khai với giới hạn tốc độ.", status)
		}
		c.mu.Lock()
		c.robots[origin] = entry
		c.mu.Unlock()
	}
	if entry.rules != nil && !entry.rules.TestAgent(u.RequestURI(), "ConvertFix") {
		return errors.New("robots.txt của nguồn có quy tắc không cho phép tải đường dẫn này")
	}
	return nil
}
func (c *Crawler) robotsWarning(u *url.URL) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.robots[u.Scheme+"://"+u.Host].warning
}
