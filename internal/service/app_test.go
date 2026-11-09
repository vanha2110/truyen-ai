package service

import "testing"

func TestImportPreservesNumberingAndPreface(t *testing.T) {
	parts := ImportChapters("Lời mở đầu\n\nChương 37: Sơn môn\n\nVăn bản một.\n\nChương 38: Người mới\n\nVăn bản hai.")
	if len(parts) != 2 || parts[0].Index != 37 || parts[1].Index != 38 || parts[0].RawText != "Lời mở đầu\n\nVăn bản một." {
		t.Fatalf("bad import: %+v", parts)
	}
}
