package glossary

import (
	"sort"
	"strings"

	"convertfix/internal/domain"
	"convertfix/internal/textutil"
)

type pattern struct {
	text []rune
	term int
}
type node struct {
	next    map[rune]int
	fail    int
	outputs []int
}
type Matcher struct {
	nodes    []node
	patterns []pattern
	terms    []domain.Term
}
type hit struct{ start, end, term int }

func New(terms []domain.Term) *Matcher {
	terms = append([]domain.Term(nil), terms...)
	sort.SliceStable(terms, func(i, j int) bool {
		a, b := terms[i], terms[j]
		if a.Locked != b.Locked {
			return a.Locked
		}
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		if (a.NovelID != nil) != (b.NovelID != nil) {
			return a.NovelID != nil
		}
		return a.ID < b.ID
	})
	m := &Matcher{nodes: []node{{next: map[rune]int{}}}, terms: terms}
	seen := map[string]bool{}
	for pass := 0; pass < 2; pass++ {
		for i, t := range terms {
			values := append([]string{t.Source}, t.Variants...)
			if pass == 1 {
				values = []string{t.Target}
			}
			for _, v := range values {
				v = textutil.Normalize(v)
				if v == "" || seen[v] {
					continue
				}
				seen[v] = true
				p := pattern{text: []rune(v), term: i}
				pid := len(m.patterns)
				m.patterns = append(m.patterns, p)
				state := 0
				for _, r := range p.text {
					next, ok := m.nodes[state].next[r]
					if !ok {
						next = len(m.nodes)
						m.nodes = append(m.nodes, node{next: map[rune]int{}})
						m.nodes[state].next[r] = next
					}
					state = next
				}
				m.nodes[state].outputs = append(m.nodes[state].outputs, pid)
			}
		}
	}
	queue := []int{}
	for _, state := range m.nodes[0].next {
		queue = append(queue, state)
	}
	for len(queue) > 0 {
		state := queue[0]
		queue = queue[1:]
		for r, next := range m.nodes[state].next {
			queue = append(queue, next)
			f := m.nodes[state].fail
			for f != 0 {
				if _, ok := m.nodes[f].next[r]; ok {
					break
				}
				f = m.nodes[f].fail
			}
			if target, ok := m.nodes[f].next[r]; ok {
				m.nodes[next].fail = target
			}
			m.nodes[next].outputs = append(m.nodes[next].outputs, m.nodes[m.nodes[next].fail].outputs...)
		}
	}
	return m
}
func (m *Matcher) hits(input []rune) []hit {
	hits := []hit{}
	state := 0
	for i, r := range input {
		for state != 0 {
			if _, ok := m.nodes[state].next[r]; ok {
				break
			}
			state = m.nodes[state].fail
		}
		if next, ok := m.nodes[state].next[r]; ok {
			state = next
		}
		for _, pid := range m.nodes[state].outputs {
			p := m.patterns[pid]
			start := i - len(p.text) + 1
			end := i + 1
			if start > 0 && textutil.IsWord(input[start-1]) && textutil.IsWord(input[start]) {
				continue
			}
			if end < len(input) && textutil.IsWord(input[end-1]) && textutil.IsWord(input[end]) {
				continue
			}
			hits = append(hits, hit{start, end, p.term})
		}
	}
	return hits
}
func (m *Matcher) Relevant(input string, limit int) []domain.Term {
	seen := map[int]bool{}
	for _, h := range m.hits([]rune(textutil.Normalize(input))) {
		seen[h.term] = true
	}
	out := []domain.Term{}
	for i, t := range m.terms {
		if seen[i] {
			out = append(out, t)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out
}
func (m *Matcher) Replace(input string) string {
	runes := []rune(textutil.Normalize(input))
	hits := m.hits(runes)
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.start != b.start {
			return a.start < b.start
		}
		if a.end != b.end {
			return a.end > b.end
		}
		return a.term < b.term
	})
	var out strings.Builder
	cursor := 0
	for _, h := range hits {
		if h.start < cursor {
			continue
		}
		out.WriteString(string(runes[cursor:h.start]))
		out.WriteString(m.terms[h.term].Target)
		cursor = h.end
	}
	out.WriteString(string(runes[cursor:]))
	return out.String()
}
