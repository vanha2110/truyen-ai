package textutil

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var blank = regexp.MustCompile(`\n\s*\n+`)

func Normalize(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	s = strings.Map(func(r rune) rune {
		if r == '\u200b' || r == '\ufeff' {
			return -1
		}
		if r == '\u00a0' {
			return ' '
		}
		return r
	}, s)
	return norm.NFC.String(strings.TrimSpace(s))
}

func Paragraphs(s string) []string {
	s = Normalize(s)
	result := []string{}
	for _, p := range blank.Split(s, -1) {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func Words(s string) int  { return len(strings.Fields(s)) }
func Tokens(s string) int { return (len([]rune(s))*2 + 2) / 3 }
func HasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
func IsWord(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || r == '_'
}
