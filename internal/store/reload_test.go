package store

import (
	"testing"

	"convertfix/internal/domain"
)

func TestReloadSourcePreservesMetadataAndProtectsWork(t *testing.T) {
	s, _, id, job := fixture(t)
	if _, err := s.DB.Exec(`UPDATE chapters SET source_url='https://example.test/chapter/37',edited_text='Bản đã biên tập',flags='source_chapter_number=37; catalog_position=1; old_warning' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	before, err := s.Chapter(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []domain.Status{domain.Running, domain.Paused, domain.Queued, domain.Interrupted} {
		if err = s.SetJob(job, status, "", false); err != nil {
			t.Fatal(err)
		}
		if err = s.ReplaceSourceText(before, "Nội dung sạch.", false); err == nil {
			t.Fatalf("allowed active job %s", status)
		}
	}
	if err = s.SetJob(job, domain.Cancelled, "", false); err != nil {
		t.Fatal(err)
	}
	if err = s.ReplaceSourceText(before, "", false); err == nil {
		t.Fatal("accepted empty content")
	}
	if err = s.SaveManual(id, "Bản sửa tay mới"); err != nil {
		t.Fatal(err)
	}
	if err = s.ReplaceSourceText(before, "Nội dung sạch.", true); err == nil {
		t.Fatal("overwrote a concurrent manual edit")
	}
	manual, err := s.Chapter(id)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ReplaceSourceText(manual, "Nội dung sạch.", false); err == nil {
		t.Fatal("overwrote manual edit without confirmation")
	}
	chunks, err := s.Chunks(id)
	if err != nil || len(chunks) != 4 {
		t.Fatalf("failed reload changed chunks: %v %v", chunks, err)
	}
	if err = s.ReplaceSourceText(manual, "Nội dung sạch.", true); err != nil {
		t.Fatal(err)
	}
	after, err := s.Chapter(id)
	if err != nil {
		t.Fatal(err)
	}
	if after.RawText != "Nội dung sạch." || after.EditedText != "" || after.ManualEdit || after.Status != domain.Pending || after.Flags != "source_chapter_number=37; catalog_position=1" || after.SourceURL != before.SourceURL || after.Title != before.Title {
		t.Fatalf("invalid reloaded chapter: %+v", after)
	}
	chunks, err = s.Chunks(id)
	if err != nil || len(chunks) != 0 {
		t.Fatalf("old chunks retained: %v %v", chunks, err)
	}
	if err = s.UndoEdit(id); err != nil {
		t.Fatal(err)
	}
	restored, err := s.Chapter(id)
	if err != nil || restored.EditedText != "Bản sửa tay mới" {
		t.Fatalf("old edit not recoverable: %+v %v", restored, err)
	}
}
