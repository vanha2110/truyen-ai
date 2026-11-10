package service

import (
	"context"
	"testing"

	"convertfix/internal/domain"
	"convertfix/internal/store"
)

func TestReloadFailurePreservesExistingChapter(t *testing.T) {
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	novel, err := s.AddNovel(domain.Novel{Title: "Test"}, []domain.Chapter{{Index: 1, Title: "Chương 1", SourceURL: "file:///unavailable", RawText: "Bản gốc đang lưu"}})
	if err != nil {
		t.Fatal(err)
	}
	chapters, err := s.Chapters(novel)
	if err != nil || len(chapters) != 1 {
		t.Fatalf("chapters: %v %v", chapters, err)
	}
	id := chapters[0].ID
	if err = s.SaveManual(id, "Bản sửa tay đang lưu"); err != nil {
		t.Fatal(err)
	}
	before, err := s.Chapter(id)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(context.Background(), s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.ReloadChapter(id, true); err == nil {
		t.Fatal("invalid source succeeded")
	}
	after, err := s.Chapter(id)
	if err != nil || after != before {
		t.Fatalf("failed download changed stored chapter: %+v %v", after, err)
	}
}
