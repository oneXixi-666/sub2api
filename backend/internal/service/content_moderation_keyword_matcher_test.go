package service

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContentModerationKeywordMatcherReturnsStrongestLocatedMatch(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		keywords []string
	}{
		{name: "miss", text: "clean prompt", keywords: []string{"blocked", "secret"}},
		{name: "case insensitive", text: "contains SECRET value", keywords: []string{"secret"}},
		{name: "longest wins across text", text: "early appears before much-later", keywords: []string{"early", "much-later"}},
		{name: "overlap uses longest", text: "abc", keywords: []string{"bc", "abc"}},
		{name: "unicode", text: "这里包含敏感词和世界", keywords: []string{"世界", "敏感词"}},
		{name: "duplicates", text: "duplicate", keywords: []string{"duplicate", "DUPLICATE"}},
		{name: "empty entries", text: "blocked", keywords: []string{"", "blocked"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantMatch, wantHit := strongestKeywordMatchForTest(tt.text, tt.keywords)
			gotMatch, gotHit := newContentModerationKeywordMatcher(tt.keywords).Match(tt.text)
			require.Equal(t, wantHit, gotHit)
			require.Equal(t, wantMatch, gotMatch)
		})
	}
}

func TestContentModerationKeywordMatcherRandomizedParity(t *testing.T) {
	rng := rand.New(rand.NewSource(20260714))
	const alphabet = "abcXYZ"
	for iteration := 0; iteration < 1000; iteration++ {
		keywords := make([]string, 1+rng.Intn(30))
		for index := range keywords {
			length := 1 + rng.Intn(8)
			var value strings.Builder
			for range length {
				_ = value.WriteByte(alphabet[rng.Intn(len(alphabet))])
			}
			keywords[index] = value.String()
		}
		var text strings.Builder
		for range 20 + rng.Intn(100) {
			_ = text.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}

		wantMatch, wantHit := strongestKeywordMatchForTest(text.String(), keywords)
		gotMatch, gotHit := newContentModerationKeywordMatcher(keywords).Match(text.String())
		require.Equal(t, wantHit, gotHit, "iteration %d", iteration)
		require.Equal(t, wantMatch, gotMatch, "iteration %d", iteration)
	}
}

func TestContentModerationKeywordMatcherOffsetsReferToOriginalText(t *testing.T) {
	text := "İstanbul then RISK-PHRASE"
	match, hit := newContentModerationKeywordMatcher([]string{"risk-phrase"}).Match(text)
	require.True(t, hit)
	require.Equal(t, "RISK-PHRASE", text[match.Start:match.End])
}

func strongestKeywordMatchForTest(text string, keywords []string) (KeywordMatch, bool) {
	lower := strings.ToLower(text)
	bestIndex := -1
	best := KeywordMatch{}
	for index, keyword := range keywords {
		if keyword == "" {
			continue
		}
		start := strings.Index(lower, strings.ToLower(keyword))
		if start < 0 {
			continue
		}
		candidate := KeywordMatch{Keyword: keyword, Start: start, End: start + len([]byte(strings.ToLower(keyword)))}
		if bestIndex < 0 || len([]byte(strings.ToLower(keyword))) > len([]byte(strings.ToLower(best.Keyword))) ||
			(len([]byte(strings.ToLower(keyword))) == len([]byte(strings.ToLower(best.Keyword))) && index < bestIndex) {
			bestIndex = index
			best = candidate
		}
	}
	return best, bestIndex >= 0
}
