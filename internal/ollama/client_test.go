package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"convertfix/internal/domain"
)

func TestChatRequiresFinalFrameAndPreservesStream(t *testing.T) {
	cases := []struct {
		name  string
		final bool
	}{{"complete", true}, {"truncated", false}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request Request
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if !request.Stream || request.Options.NumCtx == 0 {
					t.Error("request missing stream/context")
				}
				e := json.NewEncoder(w)
				_ = e.Encode(Response{Message: Message{Content: "Xin "}})
				_ = e.Encode(Response{Message: Message{Content: "chào"}})
				if tc.final {
					_ = e.Encode(Response{Done: true, EvalCount: 10, EvalDuration: 1e9})
				}
			}))
			defer server.Close()
			c, err := New(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			var streamed string
			r, err := c.Chat(context.Background(), NewRequest("model", "system", "user", domain.DefaultConfig(), Info{}), func(s string) { streamed += s })
			if tc.final {
				if err != nil || r.Message.Content != "Xin chào" || streamed != "Xin chào" || r.TokensPerSecond() != 10 {
					t.Fatalf("bad stream: %+v %v", r, err)
				}
			} else if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("truncated stream accepted: %v", err)
			}
		})
	}
}
func TestThinkingOnlyForSupportedModels(t *testing.T) {
	c := domain.DefaultConfig()
	r := NewRequest("model", "s", "u", c, Info{})
	if r.Think != nil {
		t.Fatal("think sent to unsupported model")
	}
	r = NewRequest("model", "s", "u", c, Info{Capabilities: []string{"thinking"}})
	if r.Think == nil || *r.Think {
		t.Fatal("thinking not disabled")
	}
}
