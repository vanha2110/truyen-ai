package store

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"convertfix/internal/domain"
	"convertfix/internal/textutil"
)

func (s *Store) Terms(novel int64) ([]domain.Term, error) {
	rows, err := s.DB.Query(`SELECT g.id,g.novel_id,g.source_term,g.target_term,g.type,g.note,g.priority,g.locked,g.origin,COALESCE((SELECT json_group_array(variant) FROM glossary_variants WHERE glossary_id=g.id),'[]') FROM glossary g WHERE g.novel_id IS NULL OR g.novel_id=? ORDER BY g.locked DESC,g.priority DESC,g.novel_id DESC,g.id`, novel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Term{}
	for rows.Next() {
		var t domain.Term
		var variants string
		if err = rows.Scan(&t.ID, &t.NovelID, &t.Source, &t.Target, &t.Kind, &t.Note, &t.Priority, &t.Locked, &t.Origin, &variants); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(variants), &t.Variants); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func saveTerm(tx *sql.Tx, t domain.Term, confirmLocked bool) error {
	t.Source = textutil.Normalize(t.Source)
	t.Target = textutil.Normalize(t.Target)
	if t.Source == "" || t.Target == "" || strings.ContainsAny(t.Source+t.Target, "\n\r") {
		return errors.New("Thuật ngữ phải có nguồn và đích trên một dòng")
	}
	if t.NovelID != nil && *t.NovelID < 1 {
		return errors.New("Phạm vi truyện không hợp lệ")
	}
	var id int64
	var locked bool
	var err error
	if t.ID > 0 {
		err = tx.QueryRow(`SELECT id,locked FROM glossary WHERE id=?`, t.ID).Scan(&id, &locked)
	} else {
		err = tx.QueryRow(`SELECT id,locked FROM glossary WHERE COALESCE(novel_id,0)=COALESCE(?,0) AND source_term=?`, t.NovelID, t.Source).Scan(&id, &locked)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if locked && !confirmLocked {
		return fmt.Errorf("Thuật ngữ %q đã khóa; cần mở khóa trước khi thay đổi", t.Source)
	}
	if t.Origin == "" {
		t.Origin = "manual"
	}
	if t.Kind == "" {
		t.Kind = "name"
	}
	if id > 0 {
		_, err = tx.Exec(`UPDATE glossary SET novel_id=?,source_term=?,target_term=?,type=?,note=?,priority=?,locked=?,origin=? WHERE id=?`, t.NovelID, t.Source, t.Target, t.Kind, t.Note, t.Priority, t.Locked, t.Origin, id)
	} else {
		var result sql.Result
		result, err = tx.Exec(`INSERT INTO glossary(novel_id,source_term,target_term,type,note,priority,locked,origin) VALUES(?,?,?,?,?,?,?,?)`, t.NovelID, t.Source, t.Target, t.Kind, t.Note, t.Priority, t.Locked, t.Origin)
		if err == nil {
			id, err = result.LastInsertId()
		}
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM glossary_variants WHERE glossary_id=?`, id); err != nil {
		return err
	}
	for _, v := range t.Variants {
		v = textutil.Normalize(v)
		if v == "" {
			continue
		}
		if _, err = tx.Exec(`INSERT OR IGNORE INTO glossary_variants(glossary_id,variant) VALUES(?,?)`, id, v); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) SaveTerm(t domain.Term, confirmLocked bool) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = saveTerm(tx, t, confirmLocked); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) DeleteTerm(id int64) error {
	r, err := s.DB.Exec(`DELETE FROM glossary WHERE id=? AND locked=0`, id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("Thuật ngữ không tồn tại hoặc đang khóa")
	}
	return nil
}
func (s *Store) ImportTerms(data, format string, novel *int64) error {
	terms := []domain.Term{}
	switch format {
	case "json":
		d := json.NewDecoder(strings.NewReader(data))
		d.DisallowUnknownFields()
		if err := d.Decode(&terms); err != nil {
			return err
		}
		if d.Decode(new(interface{})) != io.EOF {
			return errors.New("JSON chứa dữ liệu thừa")
		}
	case "csv":
		r := csv.NewReader(strings.NewReader(strings.TrimPrefix(data, "\ufeff")))
		header, err := r.Read()
		if err != nil {
			return err
		}
		if len(header) != 4 || strings.Join(header, ",") != "source,target,kind,variants" {
			return errors.New("CSV cần header source,target,kind,variants")
		}
		for {
			row, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			terms = append(terms, domain.Term{Source: row[0], Target: row[1], Kind: row[2], Variants: strings.Split(row[3], "|")})
		}
	default:
		return errors.New("Định dạng phải là json hoặc csv")
	}
	if len(terms) > 10000 {
		return errors.New("Tối đa 10000 thuật ngữ mỗi lần nhập")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, t := range terms {
		t.ID = 0
		t.NovelID = novel
		t.Origin = "import"
		if err = saveTerm(tx, t, false); err != nil {
			return fmt.Errorf("Dòng %s: %w", strconv.Itoa(i+1), err)
		}
	}
	return tx.Commit()
}
func (s *Store) ExportTerms(novel int64, format string) (string, error) {
	terms, err := s.Terms(novel)
	if err != nil {
		return "", err
	}
	if format == "json" {
		b, err := json.MarshalIndent(terms, "", "  ")
		return string(b), err
	}
	if format != "csv" {
		return "", errors.New("Định dạng không hợp lệ")
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	if err = w.Write([]string{"source", "target", "kind", "variants"}); err != nil {
		return "", err
	}
	for _, t := range terms {
		if err = w.Write([]string{t.Source, t.Target, t.Kind, strings.Join(t.Variants, "|")}); err != nil {
			return "", err
		}
	}
	w.Flush()
	return b.String(), w.Error()
}
