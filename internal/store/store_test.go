package store

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"convertfix/internal/domain"
)

func fixture(t *testing.T) (*Store, int64, int64, int64) {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	novel, err := s.AddNovel(domain.Novel{Title: "Truyện thử"}, []domain.Chapter{{Index: 37, Title: "Chương 37", RawText: "một\n\nhai\n\nba\n\nbốn"}})
	if err != nil {
		t.Fatal(err)
	}
	chapters, err := s.Chapters(novel)
	if err != nil {
		t.Fatal(err)
	}
	ch := chapters[0].ID
	if err = s.EnsureChunks(ch, []string{"một", "hai", "ba", "bốn"}); err != nil {
		t.Fatal(err)
	}
	job, err := s.AddJob(novel, 37, 37, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetJob(job, domain.Running, "", false); err != nil {
		t.Fatal(err)
	}
	return s, novel, ch, job
}
func TestRecoverPartialChapterAndPoisonChunk(t *testing.T) {
	s, _, ch, job := fixture(t)
	chunks, _ := s.Chunks(ch)
	for _, c := range chunks[:2] {
		if err := s.ClaimChunk(job, c.ID); err != nil {
			t.Fatal(err)
		}
		c.Status = domain.Done
		c.EditedText = c.RawText + " đã sửa"
		if err := s.CompleteChunk(job, c, "test", domain.DefaultConfig()); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ClaimChunk(job, chunks[2].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Recover(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	result, _ := s.Chunks(ch)
	if result[0].Status != domain.Done || result[1].Status != domain.Done || result[2].Status != domain.Pending || result[2].CrashCount != 1 {
		t.Fatalf("recovery incorrect: %+v", result)
	}
	jobs, _ := s.Jobs()
	if jobs[0].Status != domain.Interrupted {
		t.Fatal(jobs)
	}
	if err := s.SetJob(job, domain.Running, "", false); err != nil {
		t.Fatal(err)
	}
	if err := s.ClaimChunk(job, result[2].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Recover(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	result, _ = s.Chunks(ch)
	if result[2].Status != domain.NeedsReview || result[2].EditedText != "ba" || result[2].CrashCount != 2 {
		t.Fatal(result[2])
	}
	if err := s.SetJob(job, domain.Running, "", false); err != nil {
		t.Fatal(err)
	}
	last := result[3]
	if err := s.ClaimChunk(job, last.ID); err != nil {
		t.Fatal(err)
	}
	last.Status = domain.Done
	last.EditedText = "bốn đã sửa"
	if err := s.CompleteChunk(job, last, "test", domain.DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	chapter, _ := s.Chapter(ch)
	if chapter.Status != domain.NeedsReview || chapter.EditedText != "một đã sửa\n\nhai đã sửa\n\nba\n\nbốn đã sửa" {
		t.Fatalf("assembly lost/repeated content: %+v", chapter)
	}
	if chapter.RawText != "một\n\nhai\n\nba\n\nbốn" {
		t.Fatal("raw changed")
	}
}
func TestPauseBlocksLateCommitAndSurvivesRestart(t *testing.T) {
	s, _, ch, job := fixture(t)
	chunks, _ := s.Chunks(ch)
	c := chunks[0]
	if err := s.ClaimChunk(job, c.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetJob(job, domain.Paused, "", false); err != nil {
		t.Fatal(err)
	}
	c.Status = domain.Done
	c.EditedText = "late"
	if err := s.CompleteChunk(job, c, "test", domain.DefaultConfig()); err == nil {
		t.Fatal("late commit accepted")
	}
	if err := s.Recover(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	jobs, _ := s.Jobs()
	parts, _ := s.Chunks(ch)
	if jobs[0].Status != domain.Paused || parts[0].CrashCount != 0 || parts[0].Status != domain.Pending {
		t.Fatal("pause/recovery state wrong")
	}
}
func TestAtomicGlossaryImportGlobalUniquenessAndLocks(t *testing.T) {
	s, _, _, _ := fixture(t)
	term := domain.Term{Source: "Lâm Phong", Target: "Lâm Vũ", Locked: true}
	if err := s.SaveTerm(term, false); err != nil {
		t.Fatal(err)
	}
	if err := s.ImportTerms(`[{"source":"Mới","target":"Tên mới"},{"source":"Lâm Phong","target":"Sai"}]`, "json", nil); err == nil {
		t.Fatal("locked import allowed")
	}
	terms, _ := s.Terms(0)
	if len(terms) != 1 || terms[0].Target != "Lâm Vũ" {
		t.Fatal("import not atomic")
	}
	term.Locked = false
	term.Target = "Đổi"
	if err := s.SaveTerm(term, true); err != nil {
		t.Fatal(err)
	}
	terms, _ = s.Terms(0)
	if len(terms) != 1 || terms[0].Target != "Đổi" {
		t.Fatal("duplicate global entries")
	}
}
func TestManualProtectionOverlapBackupAndLock(t *testing.T) {
	s, novel, ch, job := fixture(t)
	if _, err := Open(s.Dir); err == nil {
		t.Fatal("second instance allowed")
	}
	if _, err := s.AddJob(novel, 37, 37, 1); err == nil {
		t.Fatal("overlap allowed")
	}
	if err := s.SaveManual(ch, "Sửa tay"); err == nil {
		t.Fatal("manual edit during active job allowed")
	}
	if err := s.SetJob(job, domain.Cancelled, "", false); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveManual(ch, "Sửa tay"); err != nil {
		t.Fatal(err)
	}
	if err := s.RetryChapter(ch, false, false); err == nil {
		t.Fatal("manual edit overwritten without confirmation")
	}
	path, err := s.Backup()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogNumberingWarningSurvivesImport(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	flags := "source_chapter_number=107; catalog_position=105"
	id, err := s.AddNovel(domain.Novel{Title: "Mục lục"}, []domain.Chapter{{Index: 105, Title: "Chương 107: Tên gốc", SourceURL: "https://example.test/chuong-107", Flags: flags}})
	if err != nil {
		t.Fatal(err)
	}
	chapters, err := s.Chapters(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 1 || chapters[0].Flags != flags || chapters[0].Title != "Chương 107: Tên gốc" {
		t.Fatal(chapters)
	}
}
func TestAbruptProcessExitLeavesRecoverableChunk(t *testing.T) {
	if os.Getenv("CONVERTFIX_CRASH_HELPER") == "1" {
		dir := os.Getenv("CONVERTFIX_TEST_DIR")
		s, err := Open(dir)
		if err != nil {
			os.Exit(11)
		}
		id, err := s.AddNovel(domain.Novel{Title: "Crash"}, []domain.Chapter{{Index: 1, Title: "One", RawText: "first second"}})
		if err != nil {
			os.Exit(12)
		}
		chs, _ := s.Chapters(id)
		if err = s.EnsureChunks(chs[0].ID, []string{"first", "second"}); err != nil {
			os.Exit(13)
		}
		job, err := s.AddJob(id, 1, 1, 0)
		if err != nil {
			os.Exit(14)
		}
		if err = s.SetJob(job, domain.Running, "", false); err != nil {
			os.Exit(15)
		}
		chunks, _ := s.Chunks(chs[0].ID)
		if err = s.ClaimChunk(job, chunks[0].ID); err != nil {
			os.Exit(16)
		}
		first := chunks[0]
		first.Status = domain.Done
		first.EditedText = "committed"
		if err = s.CompleteChunk(job, first, "test", domain.DefaultConfig()); err != nil {
			os.Exit(17)
		}
		if err = s.ClaimChunk(job, chunks[1].ID); err != nil {
			os.Exit(18)
		}
		tx, err := s.DB.Begin()
		if err != nil {
			os.Exit(19)
		}
		if _, err = tx.Exec(`UPDATE chunks SET edited_text='UNCOMMITTED',status='done' WHERE id=?`, chunks[1].ID); err != nil {
			os.Exit(20)
		}
		if err = os.WriteFile(filepath.Join(dir, "ready"), []byte("ready"), 0600); err != nil {
			os.Exit(21)
		}
		time.Sleep(30 * time.Second)
		os.Exit(22)
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestAbruptProcessExitLeavesRecoverableChunk$")
	cmd.Env = append(os.Environ(), "CONVERTFIX_CRASH_HELPER=1", "CONVERTFIX_TEST_DIR="+dir)
	var log strings.Builder
	cmd.Stdout = &log
	cmd.Stderr = &log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "ready")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child timeout")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Recover(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	parts, err := s.Chunks(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 || parts[0].EditedText != "committed" || parts[1].Status != domain.Pending || parts[1].EditedText != "" || parts[1].CrashCount != 1 {
		t.Fatalf("durability failure: %+v", parts)
	}
	var integrity string
	if err = s.DB.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity: %s %v", integrity, err)
	}
}
