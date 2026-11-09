package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"convertfix/internal/domain"
)

func (s *Store) AddJob(novel int64, from, to, priority int) (int64, error) {
	if from < 1 || to < from {
		return 0, errors.New("Khoảng chương không hợp lệ")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var count, overlap int
	if err = tx.QueryRow(`SELECT count(*) FROM chapters WHERE novel_id=? AND idx BETWEEN ? AND ?`, novel, from, to).Scan(&count); err != nil {
		return 0, err
	}
	if count != to-from+1 {
		return 0, errors.New("Khoảng chọn có chương thiếu; hãy kiểm tra mục lục")
	}
	if err = tx.QueryRow(`SELECT count(*) FROM jobs WHERE novel_id=? AND from_idx<=? AND to_idx>=? AND status IN('queued','running','paused','interrupted')`, novel, to, from).Scan(&overlap); err != nil {
		return 0, err
	}
	if overlap > 0 {
		return 0, errors.New("Khoảng chương đã có trong một job chưa kết thúc")
	}
	res, err := tx.Exec(`INSERT INTO jobs(novel_id,from_idx,to_idx,priority) VALUES(?,?,?,?)`, novel, from, to, priority)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}
func (s *Store) Jobs() ([]domain.Job, error) {
	rows, err := s.DB.Query(`SELECT j.id,j.novel_id,n.title,j.from_idx,j.to_idx,j.status,j.priority,j.error,j.auto_retry,
 (SELECT count(*) FROM chunks c JOIN chapters ch ON c.chapter_id=ch.id WHERE ch.novel_id=j.novel_id AND ch.idx BETWEEN j.from_idx AND j.to_idx AND c.status IN('done','needs_review')),
 (SELECT count(*) FROM chunks c JOIN chapters ch ON c.chapter_id=ch.id WHERE ch.novel_id=j.novel_id AND ch.idx BETWEEN j.from_idx AND j.to_idx)
 FROM jobs j JOIN novels n ON n.id=j.novel_id ORDER BY j.priority DESC,j.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Job{}
	for rows.Next() {
		var j domain.Job
		if err = rows.Scan(&j.ID, &j.NovelID, &j.Title, &j.From, &j.To, &j.Status, &j.Priority, &j.Error, &j.AutoRetry, &j.Completed, &j.Total); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
func (s *Store) NextJob() (domain.Job, error) {
	var j domain.Job
	err := s.DB.QueryRow(`SELECT id,novel_id,from_idx,to_idx,status,priority FROM jobs WHERE status IN('queued','interrupted') OR (status='paused' AND auto_retry=1 AND retry_after<=?) ORDER BY priority DESC,id LIMIT 1`, time.Now().Unix()).Scan(&j.ID, &j.NovelID, &j.From, &j.To, &j.Status, &j.Priority)
	return j, err
}
func (s *Store) SetJob(id int64, status domain.Status, message string, auto bool) error {
	switch status {
	case domain.Running, domain.Paused, domain.Interrupted, domain.Done, domain.Cancelled, domain.Error, domain.Queued:
	default:
		return errors.New("Trạng thái job không hợp lệ")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old domain.Status
	var failures int
	if err = tx.QueryRow(`SELECT status,failures FROM jobs WHERE id=?`, id).Scan(&old, &failures); err != nil {
		return err
	}
	if old == domain.Done || old == domain.Cancelled {
		return errors.New("Job đã kết thúc")
	}
	delay := int64(0)
	if auto {
		steps := []int64{5, 15, 60, 300}
		i := failures
		if i >= len(steps) {
			i = len(steps) - 1
		}
		delay = time.Now().Unix() + steps[i]
		failures++
	} else if status == domain.Done {
		failures = 0
	}
	_, err = tx.Exec(`UPDATE jobs SET status=?,error=?,auto_retry=?,retry_after=?,failures=?,started_at=CASE WHEN ?='running' THEN COALESCE(started_at,CURRENT_TIMESTAMP) ELSE started_at END,finished_at=CASE WHEN ? IN('done','cancelled','error') THEN CURRENT_TIMESTAMP ELSE NULL END WHERE id=?`, status, message, auto, delay, failures, status, status, id)
	if err != nil {
		return err
	}
	if status != domain.Running {
		if _, err = tx.Exec(`UPDATE chunks SET status='pending',started_at=NULL WHERE status='processing' AND chapter_id IN(SELECT c.id FROM chapters c JOIN jobs j ON c.novel_id=j.novel_id WHERE j.id=? AND c.idx BETWEEN j.from_idx AND j.to_idx)`, id); err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE chapters SET status='queued' WHERE status='processing' AND novel_id=(SELECT novel_id FROM jobs WHERE id=?) AND idx BETWEEN (SELECT from_idx FROM jobs WHERE id=?) AND (SELECT to_idx FROM jobs WHERE id=?)`, id, id, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) ClaimChunk(job, chunk int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.Exec(`UPDATE chunks SET status='processing',started_at=CURRENT_TIMESTAMP WHERE id=? AND status='pending' AND EXISTS(SELECT 1 FROM jobs j JOIN chapters ch ON ch.novel_id=j.novel_id WHERE j.id=? AND j.status='running' AND ch.id=chunks.chapter_id AND ch.idx BETWEEN j.from_idx AND j.to_idx AND ch.manual_edit=0)`, chunk, job)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("Chunk không còn sẵn sàng")
	}
	_, err = tx.Exec(`UPDATE chapters SET status='processing' WHERE id=(SELECT chapter_id FROM chunks WHERE id=?)`, chunk)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Attempt(chunk int64) error {
	_, err := s.DB.Exec(`UPDATE chunks SET attempts=attempts+1 WHERE id=? AND status='processing'`, chunk)
	return err
}
func (s *Store) CompleteChunk(job int64, c domain.Chunk, model string, config domain.Config) error {
	if c.Status != domain.Done && c.Status != domain.NeedsReview {
		return errors.New("Kết quả chunk chưa hợp lệ")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.Exec(`UPDATE chunks SET edited_text=?,status=?,flags=?,started_at=NULL WHERE id=? AND status='processing' AND EXISTS(SELECT 1 FROM jobs WHERE id=? AND status='running')`, c.EditedText, c.Status, c.Flags, c.ID, job)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("Job đã dừng trước khi ghi kết quả")
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE chapters SET model_used=?,preset_json=? WHERE id=?`, model, string(data), c.ChapterID); err != nil {
		return err
	}
	if err = assemble(tx, c.ChapterID); err != nil {
		return err
	}
	return tx.Commit()
}
func assemble(tx *sql.Tx, chapter int64) error {
	rows, err := tx.Query(`SELECT status,raw_text,edited_text,flags FROM chunks WHERE chapter_id=? ORDER BY idx`, chapter)
	if err != nil {
		return err
	}
	parts := []string{}
	flags := []string{}
	finished := true
	review := false
	for rows.Next() {
		var status domain.Status
		var raw, edited, flag string
		if err = rows.Scan(&status, &raw, &edited, &flag); err != nil {
			rows.Close()
			return err
		}
		if status == domain.Pending || status == domain.Processing {
			finished = false
		}
		if status == domain.NeedsReview {
			review = true
		}
		if edited == "" {
			edited = raw
		}
		parts = append(parts, edited)
		if flag != "" {
			flags = append(flags, flag)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !finished || len(parts) == 0 {
		return nil
	}
	status := domain.Done
	if review {
		status = domain.NeedsReview
	}
	_, err = tx.Exec(`UPDATE chapters SET edited_text=?,status=?,flags=trim(flags||' '||?),updated_at=CURRENT_TIMESTAMP WHERE id=? AND manual_edit=0`, strings.Join(parts, "\n\n"), status, strings.Join(flags, "; "), chapter)
	return err
}
func (s *Store) Assemble(chapter int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = assemble(tx, chapter); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) RetryChapter(chapter int64, onlyErrors, confirmManual bool) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var manual bool
	var active int
	if err = tx.QueryRow(`SELECT manual_edit FROM chapters WHERE id=?`, chapter).Scan(&manual); err != nil {
		return err
	}
	if manual && !confirmManual {
		return errors.New("Chương đã sửa tay; cần xác nhận ghi đè")
	}
	if err = tx.QueryRow(`SELECT count(*) FROM jobs j JOIN chapters c ON c.novel_id=j.novel_id WHERE c.id=? AND c.idx BETWEEN j.from_idx AND j.to_idx AND j.status IN('running','queued','interrupted','paused')`, chapter).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return errors.New("Hủy hoặc hoàn tất job hiện tại trước khi dịch lại")
	}
	q := `UPDATE chunks SET status='pending',attempts=0,crash_count=0,flags='',edited_text='' WHERE chapter_id=?`
	if onlyErrors {
		q += ` AND status='needs_review'`
	}
	if _, err = tx.Exec(q, chapter); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE chapters SET status='pending',manual_edit=0,summary='',flags='' WHERE id=?`, chapter); err != nil {
		return err
	}
	return tx.Commit()
}
