package manifest

import (
	"sort"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"

	"pinax/internal/crawler"
)

// FallbackSearch is the substring-AND matcher used when BM25 is unavailable
// or returns no hits. Every query token must appear (case-insensitive) in
// the page's URL+title; ties broken by specificity. When that also returns
// nothing it retries with a fuzzy pass that tolerates typos.
//
// Shared by the CLI `pinax search` command and the MCP `search_pages` tool.
func FallbackSearch(pages []crawler.Page, query string, limit int) []Hit {
	tokens := strings.Fields(strings.ToLower(query))
	if len(tokens) == 0 {
		return nil
	}

	type scored struct {
		idx   int
		score int
	}
	var matches []scored
	for i, p := range pages {
		haystack := NormalizeHaystack(p.URL + " " + p.Title)
		if s, ok := scoreTokens(tokens, haystack); ok {
			matches = append(matches, scored{i, s})
		}
	}
	if len(matches) == 0 {
		for i, p := range pages {
			haystack := NormalizeHaystack(p.URL + " " + p.Title)
			if s, ok := fuzzyScoreTokens(tokens, haystack); ok {
				matches = append(matches, scored{i, s})
			}
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score < matches[j].score })
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	out := make([]Hit, len(matches))
	for i, m := range matches {
		out[i] = Hit{DocID: m.idx, Score: float64(-m.score)}
	}
	return out
}

func scoreTokens(tokens []string, haystack string) (int, bool) {
	score := 0
	for _, tok := range tokens {
		idx := strings.Index(haystack, tok)
		if idx < 0 {
			return 0, false
		}
		score += len(haystack) - idx
	}
	score += len(haystack)
	return score, true
}

func fuzzyScoreTokens(tokens []string, haystack string) (int, bool) {
	total := 0
	for _, tok := range tokens {
		if !fuzzy.MatchFold(tok, haystack) {
			return 0, false
		}
		total += fuzzy.RankMatchFold(tok, haystack)
	}
	return total, true
}

// NormalizeHaystack lowercases and replaces non-alphanumeric runes with
// spaces so per-token substring matching works across path separators,
// dashes and underscores.
func NormalizeHaystack(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
			b[i] = c + ('a' - 'A')
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			b[i] = c
		default:
			b[i] = ' '
		}
	}
	return string(b)
}
