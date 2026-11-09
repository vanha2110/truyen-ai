package glossary

import (
	"convertfix/internal/domain"
	"golang.org/x/text/unicode/norm"
	"testing"
)

func TestMatcherBoundariesLongestAndNoCascade(t *testing.T) {
	terms := []domain.Term{{Source: "An", Target: "Bình"}, {Source: "Lâm Phong", Target: "Lâm Vũ", Variants: []string{"Lam Phong"}}, {Source: "Lâm", Target: "Rừng"}, {Source: "Bình", Target: "Khác"}}
	m := New(terms)
	raw := "An Anan, Lam Phong và Lâm Phong; Bình."
	expected := "Bình Anan, Lâm Vũ và Lâm Vũ; Khác."
	if got := m.Replace(raw); got != expected {
		t.Fatalf("%q != %q", got, expected)
	}
	if got := m.Replace(norm.NFD.String(raw)); got != expected {
		t.Fatalf("NFD mismatch: %s", got)
	}
	if got := New([]domain.Term{{Source: "Vũ", Target: "Vân"}}).Replace("Vũa Vũ."); got != "Vũa Vân." {
		t.Fatal(got)
	}
}
func TestLockedPriorityAndScopedOverride(t *testing.T) {
	novel := int64(1)
	m := New([]domain.Term{{ID: 1, Source: "Lâm Phong", Target: "global"}, {ID: 2, NovelID: &novel, Source: "Lâm Phong", Target: "local"}})
	if got := m.Replace("Lâm Phong"); got != "local" {
		t.Fatal(got)
	}
	m = New([]domain.Term{{ID: 1, Locked: true, Source: "Lâm Phong", Target: "locked"}, {ID: 2, NovelID: &novel, Priority: 999, Source: "Lâm Phong", Target: "local"}})
	if got := m.Replace("Lâm Phong"); got != "locked" {
		t.Fatal(got)
	}
	if len(m.Relevant("không có tên", 60)) != 0 {
		t.Fatal("unrelated entries selected")
	}
}
