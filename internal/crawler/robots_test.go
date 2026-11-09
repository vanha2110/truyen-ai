package crawler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"convertfix/internal/ollama"
)

func TestRobotsStatusPolicy(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		allowed bool
		warning bool
	}{
		{"allow", 200, "User-agent: *\nAllow: /", true, false},
		{"disallow", 200, "User-agent: *\nDisallow: /", false, false},
		{"404 unavailable", 404, "not found", true, false},
		{"410 gone", 410, "gone", true, false},
		{"401 unavailable", 401, "Unauthorized", true, true},
		{"403 unavailable", 403, "Forbidden", true, true},
		{"429 rate limit", 429, "Too many requests", false, false},
		{"503 unreachable", 503, "Unavailable", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var pages atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/robots.txt" {
					w.WriteHeader(tc.status)
					fmt.Fprint(w, tc.body)
					return
				}
				pages.Add(1)
				fmt.Fprint(w, `<h1>Fixture</h1><a href="/story/chuong-1">Chương 1</a>`)
			}))
			defer server.Close()
			c := New()
			c.http = server.Client()
			c.interval = 0
			p, err := c.Preview(context.Background(), server.URL+"/story", DefaultSource())
			if tc.allowed {
				if err != nil || len(p.Chapters) != 1 {
					t.Fatalf("unexpected block: %v", err)
				}
				if (len(p.Warnings) > 0) != tc.warning {
					t.Fatal("wrong warning", p.Warnings)
				}
			} else {
				if err == nil || pages.Load() != 0 {
					t.Fatalf("source fetched despite blocking policy: %v count=%d", err, pages.Load())
				}
			}
		})
	}
}
func TestUntrustedTLSIsReportedAndNotRetried(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); fmt.Fprint(w, "User-agent: *") }))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()
	c := New()
	c.http = &http.Client{Timeout: 5 * time.Second}
	c.interval = 0
	_, err := c.Preview(context.Background(), server.URL+"/story", DefaultSource())
	var certificate *CertificateError
	var verification *tls.CertificateVerificationError
	if !errors.As(err, &certificate) || !errors.As(err, &verification) {
		t.Fatalf("missing typed TLS error: %v", err)
	}
	if !strings.Contains(err.Error(), "lỗi TLS") || strings.Contains(err.Error(), "quy tắc không cho phép") {
		t.Fatalf("misleading robots error: %v", err)
	}
	if requests.Load() != 0 {
		t.Fatal("unverified request reached server")
	}
	if ollama.Transient(err) {
		t.Fatal("certificate failure would auto-retry forever")
	}
	if len(c.robots) != 0 {
		t.Fatal("TLS failure cached as allow")
	}
}
func TestRobotsCacheExpires(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, "User-agent: *\nAllow: /")
	}))
	defer server.Close()
	c := New()
	c.http = server.Client()
	c.interval = 0
	u, _ := url.Parse(server.URL + "/story")
	for i := 0; i < 2; i++ {
		if err := c.checkRobots(context.Background(), u); err != nil {
			t.Fatal(err)
		}
	}
	if requests.Load() != 1 {
		t.Fatal("cache missed")
	}
	entry := c.robots[server.URL]
	entry.expiresAt = time.Now().Add(-time.Second)
	c.robots[server.URL] = entry
	if err := c.checkRobots(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 {
		t.Fatal("stale robots cache reused")
	}
}
func TestRobotsRedirectsKeepNetworkGuards(t *testing.T) {
	check := New().http.CheckRedirect
	request := func(raw string) *http.Request {
		r, err := http.NewRequest("GET", raw, nil)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if err := check(request("https://cdn.example.test/rules"), []*http.Request{request("https://example.test/robots.txt")}); err != nil {
		t.Fatal(err)
	}
	if err := check(request("https://other.example.test/story"), []*http.Request{request("https://example.test/story")}); err == nil {
		t.Fatal("catalog origin guard lost")
	}
	if err := check(request("http://example.test/robots.txt"), []*http.Request{request("https://example.test/robots.txt")}); err == nil {
		t.Fatal("HTTPS silently downgraded")
	}
}
