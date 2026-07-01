// Package bulletin implements an aggregating feed pipeline: it polls a set of
// high-frequency source feeds, extracts full text, deduplicates near-identical
// stories via SimHash, and periodically publishes an LLM-summarised digest as a
// single Atom entry. It runs outside the provider registry as its own code path.
package bulletin

import (
	"hash/fnv"
	"math/bits"
	"strings"
	"unicode"
)

// simhashBits is the fingerprint width. 64 bits gives a good balance between
// collision resistance and cheap popcount-based Hamming distance.
const simhashBits = 64

// stopwords are dropped before hashing so that common filler doesn't dominate
// the fingerprint. Intentionally small — SimHash only needs "minor stopword
// cleanups" to work well.
var stopwords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {},
	"but": {}, "by": {}, "for": {}, "from": {}, "has": {}, "have": {}, "he": {},
	"in": {}, "is": {}, "it": {}, "its": {}, "of": {}, "on": {}, "or": {},
	"that": {}, "the": {}, "this": {}, "to": {}, "was": {}, "were": {}, "will": {},
	"with": {}, "would": {}, "you": {}, "your": {},
}

// tokenize lowercases text and splits on any non-letter/-digit boundary,
// dropping stopwords and single-character tokens. Unicode letters (e.g. Finnish
// å/ä/ö and other non-ASCII scripts) are kept so non-English text still yields a
// meaningful fingerprint.
func tokenize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	tokens := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) < 2 {
			continue
		}
		if _, skip := stopwords[f]; skip {
			continue
		}
		tokens = append(tokens, f)
	}
	return tokens
}

// SimHash computes a 64-bit SimHash fingerprint of text. Similar text yields
// fingerprints with small Hamming distance. Returns 0 for empty/all-stopword
// input (callers treat 0 as "no fingerprint").
func SimHash(text string) uint64 {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return 0
	}

	var vector [simhashBits]int
	for _, tok := range tokens {
		h := fnv.New64a()
		_, _ = h.Write([]byte(tok))
		hash := h.Sum64()
		for i := range simhashBits {
			if hash&(1<<uint(i)) != 0 {
				vector[i]++
			} else {
				vector[i]--
			}
		}
	}

	var fingerprint uint64
	for i := range simhashBits {
		if vector[i] > 0 {
			fingerprint |= 1 << uint(i)
		}
	}
	return fingerprint
}

// Hamming returns the number of differing bits between two fingerprints.
func Hamming(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}
