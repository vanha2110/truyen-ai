package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"convertfix/internal/crawler"
	"convertfix/internal/domain"
	"convertfix/internal/ollama"
	"convertfix/internal/store"
)

func TestPipelineRetryFallbackAndContinue(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/show" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"capabilities": []string{"completion"}, "model_info": map[string]int{"test.context_length": 8192}})
			return
		}
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		var request ollama.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request.Options.NumCtx != 8192 {
			t.Error("missing explicit num_ctx")
		}
		n := calls.Add(1)
		user := request.Messages[len(request.Messages)-1].Content
		marker := "VĂN BẢN CONVERT (dữ liệu):\n"
		at := strings.LastIndex(user, marker)
		if at < 0 {
			t.Error("no input marker")
			return
		}
		output := user[at+len(marker):]
		if n <= 2 {
			output = "Sai hoàn toàn"
		}
		_ = json.NewEncoder(w).Encode(ollama.Response{Done: true, Message: ollama.Message{Role: "assistant", Content: output}, EvalCount: 100, EvalDuration: 1e9})
	}))
	defer server.Close()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := domain.DefaultConfig()
	c.Editor = "test"
	c.OllamaURL = server.URL
	c.ChunkWords = 50
	c.Retries = 1
	if err = s.PutJSON("config", c); err != nil {
		t.Fatal(err)
	}
	var sample strings.Builder
	for i := 1; i <= 25; i++ {
		fmt.Fprintf(&sample, "Lâm Phong bước qua bậc đá thứ %d và nhìn quanh. ", i)
	}
	raw := sample.String()
	novel, err := s.AddNovel(domain.Novel{Title: "Fixture"}, []domain.Chapter{{Index: 1, Title: "Chapter", RawText: raw}, {Index: 2, Title: "Chapter 2", RawText: "Lâm Phong nhìn lên núi. Hắn bước qua cánh cổng vào sân."}})
	if err != nil {
		t.Fatal(err)
	}
	job, err := s.AddJob(novel, 1, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetJob(job, domain.Running, "", false); err != nil {
		t.Fatal(err)
	}
	q := New(s, crawler.New(), nil)
	if err = q.run(context.Background(), domain.Job{ID: job, NovelID: novel, From: 1, To: 2}); err != nil {
		t.Fatal(err)
	}
	chapters, _ := s.Chapters(novel)
	first, _ := s.Chapter(chapters[0].ID)
	second, _ := s.Chapter(chapters[1].ID)
	if first.Status != domain.NeedsReview || second.Status != domain.Done {
		t.Fatalf("queue did not continue: %s %s", first.Status, second.Status)
	}
	chunks, _ := s.Chunks(first.ID)
	if chunks[0].Attempts != 2 || chunks[0].EditedText != chunks[0].RawText || chunks[0].Status != domain.NeedsReview {
		t.Fatal(chunks[0])
	}
	for _, chunk := range chunks[1:] {
		if chunk.Status != domain.Done {
			t.Fatal(chunk)
		}
	}
	if strings.Join(strings.Fields(first.RawText), " ") != strings.Join(strings.Fields(first.EditedText), " ") {
		t.Fatal("fallback dropped/repeated text")
	}
	jobs, _ := s.Jobs()
	if jobs[0].Status != domain.Done || jobs[0].Completed != jobs[0].Total {
		t.Fatal(jobs[0])
	}
}
func TestBackoffIncreasesAcrossRunningTransitions(t *testing.T) {
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id, err := s.AddNovel(domain.Novel{Title: "Test"}, []domain.Chapter{{Index: 1, Title: "One", RawText: "content"}})
	if err != nil {
		t.Fatal(err)
	}
	job, err := s.AddJob(id, 1, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err = s.SetJob(job, domain.Running, "", false); err != nil {
			t.Fatal(err)
		}
		if err = s.SetJob(job, domain.Paused, "offline", true); err != nil {
			t.Fatal(err)
		}
	}
	var failures int
	if err = s.DB.QueryRow(`SELECT failures FROM jobs WHERE id=?`, job).Scan(&failures); err != nil || failures != 3 {
		t.Fatalf("backoff reset: %d %v", failures, err)
	}
}
