package ollama

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"convertfix/internal/domain"
)

type Client struct {
	base string
	http *http.Client
}
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("Ollama HTTP %d: %s", e.Status, e.Message) }
func Transient(err error) bool {
	var verification *tls.CertificateVerificationError
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &verification) || errors.As(err, &authority) || errors.As(err, &hostname) || errors.As(err, &invalid) {
		return false
	}
	var h *HTTPError
	if errors.As(err, &h) {
		return h.Status == 429 || h.Status >= 500
	}
	var u *url.Error
	return errors.As(err, &u) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF)
}
func New(host string) (*Client, error) {
	u, err := url.Parse(host)
	if err != nil {
		return nil, err
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("URL Ollama phải là http(s)://host:port, không chứa thông tin đăng nhập")
	}
	return &Client{base: strings.TrimRight(host, "/"), http: &http.Client{Timeout: 20 * time.Minute}}, nil
}

type Model struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Details struct {
		Family       string `json:"family"`
		Parameters   string `json:"parameter_size"`
		Quantization string `json:"quantization_level"`
	} `json:"details"`
}
type Info struct {
	Capabilities []string                   `json:"capabilities"`
	ModelInfo    map[string]json.RawMessage `json:"model_info"`
}

func (i Info) Has(capability string) bool {
	for _, c := range i.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}
func (i Info) Context() int {
	max := 0
	for key, value := range i.ModelInfo {
		if strings.HasSuffix(key, ".context_length") {
			var n int
			if json.Unmarshal(value, &n) == nil && n > max {
				max = n
			}
		}
	}
	return max
}

type RunningModel struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	SizeVRAM int64  `json:"size_vram"`
}
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Options struct {
	Temperature   float64 `json:"temperature"`
	TopP          float64 `json:"top_p"`
	RepeatPenalty float64 `json:"repeat_penalty"`
	NumCtx        int     `json:"num_ctx"`
	NumPredict    int     `json:"num_predict"`
}
type Request struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	Stream    bool      `json:"stream"`
	Options   Options   `json:"options"`
	KeepAlive string    `json:"keep_alive"`
	Think     *bool     `json:"think,omitempty"`
	Format    string    `json:"format,omitempty"`
}

func NewRequest(model, system, user string, c domain.Config, info Info) Request {
	r := Request{Model: model, Messages: []Message{{Role: "system", Content: system}, {Role: "user", Content: user}}, Stream: true, Options: Options{c.Temperature, c.TopP, c.RepeatPenalty, c.NumCtx, c.NumPredict}, KeepAlive: c.KeepAlive}
	if c.ThinkOff && info.Has("thinking") {
		off := false
		r.Think = &off
	}
	return r
}

type Response struct {
	Message       Message `json:"message"`
	Done          bool    `json:"done"`
	DoneReason    string  `json:"done_reason"`
	Error         string  `json:"error"`
	EvalCount     int     `json:"eval_count"`
	EvalDuration  int64   `json:"eval_duration"`
	LoadDuration  int64   `json:"load_duration"`
	TotalDuration int64   `json:"total_duration"`
}

func (r Response) TokensPerSecond() float64 {
	if r.EvalDuration <= 0 {
		return 0
	}
	return float64(r.EvalCount) * 1e9 / float64(r.EvalDuration)
}

type PullProgress struct {
	Status    string `json:"status"`
	Digest    string `json:"digest"`
	Total     int64  `json:"total"`
	Completed int64  `json:"completed"`
	Error     string `json:"error"`
}

func (c *Client) request(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, &HTTPError{resp.StatusCode, string(b)}
	}
	return resp, nil
}
func (c *Client) json(ctx context.Context, method, path string, body, dst interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	resp, err := c.request(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(dst)
}
func (c *Client) Version(ctx context.Context) (string, error) {
	var v struct {
		Version string `json:"version"`
	}
	err := c.json(ctx, "GET", "/api/version", nil, &v)
	return v.Version, err
}
func (c *Client) List(ctx context.Context) ([]Model, error) {
	var v struct {
		Models []Model `json:"models"`
	}
	err := c.json(ctx, "GET", "/api/tags", nil, &v)
	if v.Models == nil {
		v.Models = []Model{}
	}
	return v.Models, err
}
func (c *Client) Show(ctx context.Context, name string) (Info, error) {
	var v Info
	err := c.json(ctx, "POST", "/api/show", map[string]string{"model": name}, &v)
	return v, err
}
func (c *Client) Running(ctx context.Context) ([]RunningModel, error) {
	var v struct {
		Models []RunningModel `json:"models"`
	}
	err := c.json(ctx, "GET", "/api/ps", nil, &v)
	if v.Models == nil {
		v.Models = []RunningModel{}
	}
	return v.Models, err
}
func (c *Client) Chat(ctx context.Context, r Request, onToken func(string)) (Response, error) {
	r.Stream = true
	resp, err := c.request(ctx, "POST", "/api/chat", r)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	scan := bufio.NewScanner(resp.Body)
	scan.Buffer(make([]byte, 64<<10), 4<<20)
	var text strings.Builder
	var final Response
	for scan.Scan() {
		var part Response
		if err = json.Unmarshal(scan.Bytes(), &part); err != nil {
			return final, err
		}
		if part.Error != "" {
			return final, errors.New(part.Error)
		}
		if text.Len()+len(part.Message.Content) > 8<<20 {
			return final, errors.New("Kết quả model vượt giới hạn 8 MB")
		}
		text.WriteString(part.Message.Content)
		if onToken != nil && part.Message.Content != "" {
			onToken(part.Message.Content)
		}
		if part.Done {
			final = part
			final.Message.Content = text.String()
			return final, nil
		}
	}
	if err = scan.Err(); err != nil {
		return final, err
	}
	return final, io.ErrUnexpectedEOF
}
func (c *Client) Pull(ctx context.Context, name string, progress func(PullProgress)) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("Cần tên model")
	}
	resp, err := c.request(ctx, "POST", "/api/pull", map[string]interface{}{"model": name, "stream": true})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	scan := bufio.NewScanner(resp.Body)
	scan.Buffer(make([]byte, 64<<10), 1<<20)
	for scan.Scan() {
		var p PullProgress
		if err = json.Unmarshal(scan.Bytes(), &p); err != nil {
			return err
		}
		if p.Error != "" {
			return errors.New(p.Error)
		}
		if progress != nil {
			progress(p)
		}
		if p.Status == "success" {
			return nil
		}
	}
	if err = scan.Err(); err != nil {
		return err
	}
	return io.ErrUnexpectedEOF
}
