package util

import (
	"strings"
	"sync"

	morphlib "github.com/vench/morph"
)

var (
	morphOnce sync.Once
	morphErr  error
)

// initMorph initializes the morph analyzer once.
func initMorph() error {
	morphOnce.Do(func() {
		morphErr = morphlib.Init()
	})
	return morphErr
}

// NormalizeRussian returns a normalized (lemma) form of a Russian token,
// using morph. If morph is unavailable or returns nothing, it falls back
// to Snowball stemming (StemOrIdentity).
func NormalizeRussian(token string) string {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return ""
	}

	if err := initMorph(); err == nil {
		_, norms, _ := morphlib.Parse(token)
		if len(norms) > 0 && norms[0] != "" {
			return norms[0]
		}
	}

	// Fallback: Snowball stemming.
	return StemOrIdentity(token, "russian")
}
