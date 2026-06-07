package manifest

import (
	"encoding/gob"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pinax/internal/crawler"
)

// IndexVersion is bumped whenever the on-disk gob layout changes. Older
// indexes that don't match are treated as missing and rebuilt on demand.
const IndexVersion = 1

// BM25 parameters. Standard literature values; revisit if recall is poor.
const (
	bm25K1 = 1.2
	bm25B  = 0.75

	// pathDepthPenalty trims score by ~5% per URL depth level so leaf pages
	// outrank hub pages with the same content.
	pathDepthPenalty = 0.05

	// titleAllTokensBoost multiplies score when every query token appears in
	// the doc title (proxy for highly relevant hits).
	titleAllTokensBoost = 1.3
)

// Index is a tiny BM25 inverted index over a manifest's pages.
type Index struct {
	Version  int
	DocCount int
	AvgLen   float64
	DocLens  []int // per-DocID token count
	DocDepth []int // per-DocID URL path depth (boost input)
	Postings map[string][]Posting
}

// Posting records one term occurrence in one doc.
type Posting struct {
	DocID   int
	TF      int
	InTitle bool
}

// Hit is one search result.
type Hit struct {
	DocID int
	Score float64
}

// englishStopwords is intentionally tiny — only the very common function
// words that hurt ranking. Domain words ("api", "auth", "config") stay in.
var englishStopwords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {},
	"be": {}, "but": {}, "by": {}, "for": {}, "from": {}, "if": {},
	"in": {}, "is": {}, "it": {}, "of": {}, "on": {}, "or": {},
	"that": {}, "the": {}, "to": {}, "was": {}, "were": {}, "will": {},
	"with": {},
}

// tokenize lowercases s, splits on any non-alphanumeric rune, drops
// stopwords and 1-character tokens.
func tokenize(s string) []string {
	if s == "" {
		return nil
	}
	out := make([]string, 0, 8)
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		t := cur.String()
		cur.Reset()
		if len(t) < 2 {
			return
		}
		if _, drop := englishStopwords[t]; drop {
			return
		}
		out = append(out, t)
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			cur.WriteRune(r + ('a' - 'A'))
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			cur.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return out
}

// urlPath extracts the path component of a URL, falling back to the raw
// string if parsing fails.
func urlPath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Path == "" {
		return raw
	}
	return u.Path
}

// pathDepth counts path segments (e.g. /a/b/c → 3, / → 0).
func pathDepth(p string) int {
	p = strings.Trim(p, "/")
	if p == "" {
		return 0
	}
	return strings.Count(p, "/") + 1
}

// BuildIndex constructs an index over pages. Empty input produces an empty
// (but valid) index.
func BuildIndex(pages []crawler.Page) *Index {
	idx := &Index{
		Version:  IndexVersion,
		DocCount: len(pages),
		DocLens:  make([]int, len(pages)),
		DocDepth: make([]int, len(pages)),
		Postings: map[string][]Posting{},
	}

	type tfEntry struct {
		tf      int
		inTitle bool
	}
	docTFs := make([]map[string]*tfEntry, len(pages))
	totalLen := 0

	for i, p := range pages {
		path := urlPath(p.URL)
		titleTokens := tokenize(p.Title)
		bodyTokens := append(tokenize(path), tokenize(p.Section)...)
		all := append(append([]string{}, titleTokens...), bodyTokens...)

		idx.DocLens[i] = len(all)
		idx.DocDepth[i] = pathDepth(path)
		totalLen += len(all)

		titleSet := make(map[string]bool, len(titleTokens))
		for _, t := range titleTokens {
			titleSet[t] = true
		}

		tfMap := map[string]*tfEntry{}
		for _, tok := range all {
			if e, ok := tfMap[tok]; ok {
				e.tf++
			} else {
				tfMap[tok] = &tfEntry{tf: 1, inTitle: titleSet[tok]}
			}
		}
		docTFs[i] = tfMap
	}

	if len(pages) > 0 {
		idx.AvgLen = float64(totalLen) / float64(len(pages))
	}

	for docID, tfMap := range docTFs {
		for tok, e := range tfMap {
			idx.Postings[tok] = append(idx.Postings[tok], Posting{
				DocID:   docID,
				TF:      e.tf,
				InTitle: e.inTitle,
			})
		}
	}

	return idx
}

// Search ranks docs against query and returns the top `limit` hits.
// Returns nil when the query has no usable tokens.
func (idx *Index) Search(query string, limit int) []Hit {
	if idx == nil || idx.DocCount == 0 {
		return nil
	}
	tokens := tokenize(query)
	if len(tokens) == 0 {
		return nil
	}

	scores := make(map[int]float64, 32)
	titleHits := make(map[int]int, 32)

	for _, t := range tokens {
		postings := idx.Postings[t]
		df := len(postings)
		if df == 0 {
			continue
		}
		// BM25+ idf: log((N - df + 0.5)/(df + 0.5) + 1) — never negative.
		idf := math.Log(float64(idx.DocCount-df)+0.5)/(float64(df)+0.5) + 1
		// Guard against the tiny case where N==df==1 makes the inner log term
		// pathological.
		if idf < 0 {
			idf = 0
		}
		for _, p := range postings {
			tf := float64(p.TF)
			dl := float64(idx.DocLens[p.DocID])
			num := tf * (bm25K1 + 1)
			denom := tf + bm25K1*(1-bm25B+bm25B*dl/idx.AvgLen)
			scores[p.DocID] += idf * num / denom
			if p.InTitle {
				titleHits[p.DocID]++
			}
		}
	}

	if len(scores) == 0 {
		return nil
	}

	hits := make([]Hit, 0, len(scores))
	for docID, base := range scores {
		s := base / (1 + pathDepthPenalty*float64(idx.DocDepth[docID]))
		if titleHits[docID] == len(tokens) {
			s *= titleAllTokensBoost
		}
		hits = append(hits, Hit{DocID: docID, Score: s})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].DocID < hits[j].DocID
	})

	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// IndexPath returns the on-disk path for a manifest's index file.
func IndexPath(name string) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".bm25"), nil
}

// SaveIndex writes idx atomically to disk for the named manifest.
func SaveIndex(name string, idx *Index) error {
	p, err := IndexPath(name)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".index-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := gob.NewEncoder(tmp).Encode(idx); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, p)
}

// LoadIndex reads the index for name. Returns ErrIndexMissing if the file
// doesn't exist or has an incompatible schema version — callers should treat
// that as "rebuild me".
func LoadIndex(name string) (*Index, error) {
	p, err := IndexPath(name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrIndexMissing
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var idx Index
	if err := gob.NewDecoder(f).Decode(&idx); err != nil {
		return nil, fmt.Errorf("decode index %q: %w", name, err)
	}
	if idx.Version != IndexVersion {
		return nil, ErrIndexMissing
	}
	return &idx, nil
}

// DeleteIndex removes the index file if present. Missing files are not an error.
func DeleteIndex(name string) error {
	p, err := IndexPath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// ErrIndexMissing signals the index file is absent or has a stale schema.
var ErrIndexMissing = errors.New("index missing or stale")
