package processor

import (
	"strings"
	"testing"

	"convertfix/internal/domain"
	"convertfix/internal/glossary"
	"convertfix/internal/textutil"
	"golang.org/x/text/unicode/norm"
)

func TestChunkerPreservesVietnameseAndBounds(t *testing.T) {
	raw := strings.Repeat("Lâm Phong bước tới sơn môn. ", 40) + "\n\n" + strings.Repeat("Nàng im lặng nhìn hắn. ", 25)
	a := Split(raw, 50)
	b := Split(norm.NFD.String(raw), 50)
	if strings.Join(a, "\n\n") != strings.Join(b, "\n\n") {
		t.Fatal("NFC/NFD mismatch")
	}
	if len(a) < 2 {
		t.Fatal("expected multiple chunks")
	}
	total := 0
	for _, part := range a {
		n := textutil.Words(part)
		if n > 50 {
			t.Fatalf("chunk too long: %d", n)
		}
		total += n
	}
	if total != textutil.Words(raw) {
		t.Fatalf("words lost: %d != %d", total, textutil.Words(raw))
	}
	if len(Split("  \n\n", 700)) != 0 {
		t.Fatal("empty input must not produce chunks")
	}
	long := strings.Repeat("từ ", 1000)
	if len(Split(long, 100)) != 10 {
		t.Fatal("long sentence not bounded")
	}
}
func TestValidation(t *testing.T) {
	raw := "Lâm Phong nhìn lên ngọn núi phía xa.\n\nHắn chậm rãi bước qua cánh cổng sơn môn."
	m := glossary.New([]domain.Term{{Source: "Lâm Phong", Target: "Lâm Phong", Variants: []string{"Lam Phong"}}})
	valid := "[1] Lam Phong nhìn lên ngọn núi phía xa.\n\n[2] Hắn chậm rãi bước qua cánh cổng sơn môn."
	cases := []struct{ name, output, flag string }{
		{"valid", valid, ""}, {"nfd", norm.NFD.String(valid), ""}, {"thinking", "<think>analysis</think>\n" + valid, ""},
		{"missing", "[1] Lâm Phong nhìn lên ngọn núi phía xa.", "paragraph_count"},
		{"order", strings.Replace(valid, "[2]", "[3]", 1), "paragraph_order"},
		{"length", "[1] Lam Phong.\n[2] Hắn đi.", "length_ratio"},
		{"han", strings.Replace(valid, "ngọn núi", "山", 1), "han_characters"},
		{"preface", "Dưới đây là kết quả:\n" + valid, "preface"},
		{"missing term", strings.Replace(valid, "Lam Phong", "Thanh niên", 1), "missing_term:Lâm Phong"},
		{"open thinking", "[1] <think> Lam Phong nhìn lên ngọn núi phía xa.\n[2] Hắn chậm rãi bước qua cánh cổng sơn môn.", "model_commentary"},
		{"repeated", "[1] Lam Phong nhìn lên ngọn núi phía xa.\n[2] Lam Phong nhìn lên ngọn núi phía xa.", "repeated_paragraph"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := Validate(raw, tc.output, m)
			if tc.flag == "" {
				if len(v.Flags) > 0 {
					t.Fatalf("unexpected flags: %v", v.Flags)
				}
				if !strings.Contains(v.Text, "Lâm Phong") {
					t.Fatal("glossary not enforced")
				}
			} else {
				found := false
				for _, flag := range v.Flags {
					if flag == tc.flag {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing %s in %v", tc.flag, v.Flags)
				}
			}
		})
	}
}
