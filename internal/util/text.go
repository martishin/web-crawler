package util

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
	"unicode"

	"github.com/kljensen/snowball"
)

// MD5Hex returns hex-encoded md5 of a string.
func MD5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// DetectLang is a tiny heuristic: if the text contains Cyrillic letters => "russian", else "english".
func DetectLang(text string) string {
	for _, r := range text {
		if unicode.In(r, unicode.Cyrillic) {
			return "russian"
		}
	}
	return "english"
}

// ExtractTokens splits a string into word tokens using unicode letters.
// Digits and punctuation are treated as separators.
func ExtractTokens(text string) []string {
	var out []string
	var b strings.Builder

	flush := func() {
		if b.Len() == 0 {
			return
		}
		out = append(out, b.String())
		b.Reset()
	}

	for _, r := range text {
		if unicode.IsLetter(r) {
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		flush()
	}
	flush()
	return out
}

// Stem applies Snowball stemmer for the given language (e.g. "russian").
// If stemming fails, returns empty string.
func Stem(token, language string) string {
	if token == "" {
		return ""
	}
	stemmed, err := snowball.Stem(token, language, true)
	if err != nil {
		return ""
	}
	return stemmed
}

// StemOrIdentity applies Snowball stemmer for the given language (e.g. "russian").
// If stemming fails, returns the original token.
func StemOrIdentity(token, language string) string {
	if token == "" {
		return token
	}
	stemmed, err := snowball.Stem(token, language, true)
	if err != nil || stemmed == "" {
		return token
	}
	return stemmed
}

// TermCounts normalizes + stems all tokens and returns a term -> count map.
// For Russian it uses morph-based lemmatization; for others it uses Snowball stemmer.
func TermCounts(text, language string) map[string]int {
	counts := make(map[string]int)
	for _, tok := range ExtractTokens(text) {
		var term string
		switch language {
		case "russian":
			term = NormalizeRussian(tok)
		default:
			term = StemOrIdentity(tok, language)
		}
		if term == "" {
			continue
		}
		counts[term]++
	}
	return counts
}
