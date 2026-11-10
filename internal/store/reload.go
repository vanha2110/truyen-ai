package store

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"convertfix/internal/domain"
	"convertfix/internal/textutil"
)

// ReplaceSourceText commits only if the chapter still matches the version fetched by the caller.
func (s *Store) ReplaceSourceText(expected domain.Chapter, raw string, confirmManual bool) error {
	raw = textutil.Normalize(raw)
	if raw == "" || expected.SourceURL == "" {
		return errors.New("Cần URL nguồn và nội dung tải lại không trống")
	}
	if expected.ManualEdit && !confirmManual {
		return errors.New("Chương đã sửa tay; cần xác nhận ghi đè")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var active int
	if err = tx.QueryRow(`SELECT count(*) FROM jobs j WHERE j.novel_id=? AND ? BETWEEN j.from_idx AND j.to_idx AND j.status IN('running','queued','interrupted','paused')`, expected.NovelID, expected.Index).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return errors.New("Hủy hoặc hoàn tất job của chương trước khi tải lại nội dung gốc")
	}
	metadata := []string{}
	for _, flag := range strings.FieldsFunc(expected.Flags, func(r rune) bool { return r == ';' || r == ',' }) {
		flag = strings.TrimSpace(flag)
		if strings.HasPrefix(flag, "source_chapter_number=") || strings.HasPrefix(flag, "catalog_position=") {
			metadata = append(metadata, flag)
		}
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
	result, err := tx.Exec(`UPDATE chapters SET raw_text=?,raw_hash=?,edited_text='',summary='',status='pending',manual_edit=0,model_used='',preset_json='',flags=?,updated_at=CURRENT_TIMESTAMP WHERE id=? AND source_url=? AND raw_text=? AND edited_text=? AND manual_edit=? AND status=? AND flags=?`, raw, hash, strings.Join(metadata, "; "), expected.ID, expected.SourceURL, expected.RawText, expected.EditedText, expected.ManualEdit, expected.Status, expected.Flags)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("Chương đã thay đổi trong lúc tải; hãy kiểm tra lại rồi thử lại")
	}
	if expected.EditedText != "" {
		if _, err = tx.Exec(`INSERT INTO edit_history(chapter_id,previous_text) VALUES(?,?)`, expected.ID, expected.EditedText); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`DELETE FROM chunks WHERE chapter_id=?`, expected.ID); err != nil {
		return err
	}
	return tx.Commit()
}
