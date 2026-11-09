package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"convertfix/internal/domain"
	"convertfix/internal/textutil"
	"github.com/gofrs/flock"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	DB   *sql.DB
	Dir  string
	lock *flock.Flock
}

func DefaultDir() string {
	if dir := os.Getenv("CONVERTFIX_DATA"); dir != "" {
		return dir
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "ConvertFix-data"
	}
	return filepath.Join(dir, "ConvertFix")
}
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(dir, "instance.lock"))
	ok, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("ConvertFix đang chạy với thư mục dữ liệu này")
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "convertfix.db"))
	if err != nil {
		_ = lock.Unlock()
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db, Dir: dir, lock: lock}
	for _, pragma := range []string{"PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000"} {
		if _, err = db.Exec(pragma); err != nil {
			_ = s.Close()
			return nil, err
		}
	}
	if err = s.migrate(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error {
	err := s.DB.Close()
	if s.lock != nil {
		return errors.Join(err, s.lock.Unlock())
	}
	return err
}
func (s *Store) migrate() error {
	if _, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_version(version TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, file := range files {
		var count int
		if err = s.DB.QueryRow(`SELECT count(*) FROM schema_version WHERE version=?`, file.Name()).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		var versions int
		if err = s.DB.QueryRow(`SELECT count(*) FROM schema_version`).Scan(&versions); err != nil {
			return err
		}
		if versions > 0 {
			if _, err = s.Backup(); err != nil {
				return fmt.Errorf("backup before migration: %w", err)
			}
		}
		data, err := migrations.ReadFile("migrations/" + file.Name())
		if err != nil {
			return err
		}
		tx, err := s.DB.Begin()
		if err != nil {
			return err
		}
		if _, err = tx.Exec(string(data)); err == nil {
			_, err = tx.Exec(`INSERT INTO schema_version(version) VALUES(?)`, file.Name())
		}
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) Backup() (string, error) {
	dir := filepath.Join(s.Dir, "backups")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	name := filepath.Join(dir, time.Now().UTC().Format("20060102-150405.000000000")+".db")
	if _, err := s.DB.Exec(`VACUUM INTO ?`, name); err != nil {
		return "", err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.db"))
	if err != nil {
		return name, err
	}
	sort.Strings(files)
	for len(files) > 5 {
		if err := os.Remove(files[0]); err != nil {
			return name, err
		}
		files = files[1:]
	}
	return name, nil
}
func (s *Store) GetJSON(key string, dst interface{}) error {
	var value string
	err := s.DB.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(value), dst)
}
func (s *Store) PutJSON(key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, string(data))
	return err
}
func (s *Store) Config() (domain.Config, error) {
	c := domain.DefaultConfig()
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		if !strings.Contains(host, "://") {
			host = "http://" + host
		}
		c.OllamaURL = host
	}
	err := s.GetJSON("config", &c)
	return c, err
}
func (s *Store) ConfigFor(novel int64) (domain.Config, error) {
	c, err := s.Config()
	if err != nil {
		return c, err
	}
	var value string
	err = s.DB.QueryRow(`SELECT value FROM novel_overrides WHERE novel_id=?`, novel).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal([]byte(value), &c)
	return c, err
}
func (s *Store) Novels() ([]domain.Novel, error) {
	rows, err := s.DB.Query(`SELECT n.id,n.title,n.author,n.description,n.source_url,n.last_read_chapter,n.last_read_offset,count(c.id),COALESCE(sum(c.status='done'),0),COALESCE(sum(c.status='needs_review'),0) FROM novels n LEFT JOIN chapters c ON c.novel_id=n.id GROUP BY n.id ORDER BY n.updated_at DESC,n.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Novel{}
	for rows.Next() {
		var n domain.Novel
		if err = rows.Scan(&n.ID, &n.Title, &n.Author, &n.Description, &n.SourceURL, &n.LastReadChapter, &n.LastReadOffset, &n.ChapterCount, &n.DoneCount, &n.ReviewCount); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
func (s *Store) AddNovel(n domain.Novel, chapters []domain.Chapter) (int64, error) {
	n.Title = textutil.Normalize(n.Title)
	if n.Title == "" || len(chapters) == 0 {
		return 0, errors.New("Cần tên truyện và ít nhất một chương")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`INSERT INTO novels(title,author,description,source_url) VALUES(?,?,?,?)`, n.Title, n.Author, n.Description, n.SourceURL)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, ch := range chapters {
		raw := textutil.Normalize(ch.RawText)
		hash := ""
		if raw != "" {
			hash = fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
		}
		if _, err = tx.Exec(`INSERT INTO chapters(novel_id,idx,title,source_url,raw_text,raw_hash,flags) VALUES(?,?,?,?,?,?,?)`, id, ch.Index, ch.Title, ch.SourceURL, raw, hash, ch.Flags); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

const chapterColumns = `id,novel_id,idx,title,source_url,raw_text,edited_text,summary,status,model_used,flags,manual_edit`

type scanner interface{ Scan(...interface{}) error }

func scanChapter(row scanner) (domain.Chapter, error) {
	var c domain.Chapter
	err := row.Scan(&c.ID, &c.NovelID, &c.Index, &c.Title, &c.SourceURL, &c.RawText, &c.EditedText, &c.Summary, &c.Status, &c.ModelUsed, &c.Flags, &c.ManualEdit)
	return c, err
}
func (s *Store) Chapter(id int64) (domain.Chapter, error) {
	return scanChapter(s.DB.QueryRow(`SELECT `+chapterColumns+` FROM chapters WHERE id=?`, id))
}
func (s *Store) Chapters(novel int64) ([]domain.Chapter, error) {
	rows, err := s.DB.Query(`SELECT id,novel_id,idx,title,source_url,'','',summary,status,model_used,flags,manual_edit FROM chapters WHERE novel_id=? ORDER BY idx`, novel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Chapter{}
	for rows.Next() {
		c, err := scanChapter(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) SaveRaw(id int64, raw string) error {
	raw = textutil.Normalize(raw)
	if raw == "" {
		return errors.New("Nội dung chương trống")
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
	_, err := s.DB.Exec(`UPDATE chapters SET raw_text=?,raw_hash=?,flags=CASE WHEN EXISTS(SELECT 1 FROM chapters c WHERE c.novel_id=chapters.novel_id AND c.id<>chapters.id AND c.raw_hash=?) THEN 'duplicate_content' ELSE flags END,updated_at=CURRENT_TIMESTAMP WHERE id=? AND raw_text=''`, raw, hash, hash, id)
	return err
}
func (s *Store) EnsureChunks(chapter int64, parts []string) error {
	if len(parts) == 0 {
		return errors.New("Chương không có nội dung")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRow(`SELECT count(*) FROM chunks WHERE chapter_id=?`, chapter).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	for i, p := range parts {
		if _, err = tx.Exec(`INSERT INTO chunks(chapter_id,idx,raw_text) VALUES(?,?,?)`, chapter, i, p); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`UPDATE chapters SET status='queued' WHERE id=? AND manual_edit=0`, chapter)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Chunks(chapter int64) ([]domain.Chunk, error) {
	rows, err := s.DB.Query(`SELECT id,chapter_id,idx,raw_text,edited_text,status,attempts,crash_count,flags FROM chunks WHERE chapter_id=? ORDER BY idx`, chapter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Chunk{}
	for rows.Next() {
		var c domain.Chunk
		if err = rows.Scan(&c.ID, &c.ChapterID, &c.Index, &c.RawText, &c.EditedText, &c.Status, &c.Attempts, &c.CrashCount, &c.Flags); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) SaveManual(chapter int64, value string) error {
	if textutil.Normalize(value) == "" {
		return errors.New("Bản sửa không được trống")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var active int
	if err = tx.QueryRow(`SELECT count(*) FROM jobs j JOIN chapters c ON c.novel_id=j.novel_id WHERE c.id=? AND c.idx BETWEEN j.from_idx AND j.to_idx AND j.status IN('running','queued','interrupted')`, chapter).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return errors.New("Tạm dừng job trước khi sửa chương")
	}
	if _, err = tx.Exec(`INSERT INTO edit_history(chapter_id,previous_text) SELECT id,edited_text FROM chapters WHERE id=?`, chapter); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE chapters SET edited_text=?,manual_edit=1,updated_at=CURRENT_TIMESTAMP WHERE id=?`, textutil.Normalize(value), chapter); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) UndoEdit(chapter int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	var value string
	if err = tx.QueryRow(`SELECT id,previous_text FROM edit_history WHERE chapter_id=? ORDER BY id DESC LIMIT 1`, chapter).Scan(&id, &value); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE chapters SET edited_text=?,manual_edit=1 WHERE id=?`, value, chapter); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM edit_history WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) SavePosition(novel, chapter int64, offset int) error {
	if offset < 0 {
		return errors.New("Vị trí đọc không hợp lệ")
	}
	_, err := s.DB.Exec(`UPDATE novels SET last_read_chapter=?,last_read_offset=? WHERE id=? AND EXISTS(SELECT 1 FROM chapters WHERE id=? AND novel_id=?)`, chapter, offset, novel, chapter, novel)
	return err
}
func (s *Store) Style(novel int64) (domain.Style, error) {
	style := domain.Style{Pronouns: "Giữ nhất quán xưng hô theo nguyên tác.", Tone: "Tự nhiên, rõ nghĩa; giữ tên riêng, công pháp và cảnh giới Hán Việt.", FewShot: []domain.Pair{}}
	var value string
	err := s.DB.QueryRow(`SELECT value FROM style_profiles WHERE novel_id=?`, novel).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return style, nil
	}
	if err != nil {
		return style, err
	}
	err = json.Unmarshal([]byte(value), &style)
	return style, err
}
func (s *Store) SaveStyle(novel int64, style domain.Style) error {
	data, err := json.Marshal(style)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`INSERT INTO style_profiles(novel_id,value) VALUES(?,?) ON CONFLICT(novel_id) DO UPDATE SET value=excluded.value`, novel, string(data))
	return err
}
func (s *Store) Recover(ctx context.Context, auto bool) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	queries := []string{
		`UPDATE chunks SET status='pending',crash_count=crash_count+1,started_at=NULL WHERE status='processing'`,
		`UPDATE chunks SET status='needs_review',edited_text=raw_text,flags='repeated_crash' WHERE status='pending' AND crash_count>=2`,
		`UPDATE chapters SET status='queued' WHERE status='processing'`,
		`UPDATE jobs SET status='interrupted' WHERE status='running'`,
	}
	if !auto {
		queries = append(queries, `UPDATE jobs SET status='paused',auto_retry=0 WHERE status IN('queued','interrupted') OR auto_retry=1`)
	}
	for _, q := range queries {
		if _, err = tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return tx.Commit()
}
