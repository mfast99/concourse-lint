package analysis_test

import (
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// TestClosestName: the "did you mean" hint suggests a known name only when
// it is close to the typo, and always the same one for the same input.
func TestClosestName(t *testing.T) {
	keys := []string{"name", "type", "source", "check_every", "tags", "serial_groups", "passed", "params"}
	testCases := []struct {
		word       string
		candidates []string
		wantName   string
		wantFound  bool
	}{
		{word: "sorce", candidates: keys, wantName: "source", wantFound: true},
		{word: "serial_group", candidates: keys, wantName: "serial_groups", wantFound: true},
		{word: "passd", candidates: keys, wantName: "passed", wantFound: true},
		{word: "Name", candidates: keys, wantName: "name", wantFound: true},
		{word: "check-every", candidates: keys, wantName: "check_every", wantFound: true},
		{word: "typ", candidates: keys, wantName: "type", wantFound: true},
		{word: "tage", candidates: keys, wantName: "tags", wantFound: true},
		// Two swapped neighbors count as one edit.
		{word: "tsak", candidates: []string{"task", "try"}, wantName: "task", wantFound: true},
		{word: "sourec", candidates: keys, wantName: "source", wantFound: true},
		// Equally close: the alphabetically first wins, so the hint is stable.
		{word: "cat", candidates: []string{"cut", "bat"}, wantName: "bat", wantFound: true},
		{word: "bogus", candidates: keys},
		{word: "x", candidates: keys},
		{word: "", candidates: keys},
		{word: "sorce"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.word, func(t *testing.T) {
			name, found := analysis.ClosestName(testCase.word, testCase.candidates)
			if name != testCase.wantName || found != testCase.wantFound {
				t.Errorf("ClosestName(%q) = %q, %v; want %q, %v", testCase.word, name, found, testCase.wantName, testCase.wantFound)
			}
		})
	}
}

func TestDidYouMean(t *testing.T) {
	tests := []struct {
		word       string
		candidates []string
		want       string
	}{
		{"rpo", []string{"repo", "image"}, `did you mean "repo"?`},
		{"something", []string{"repo"}, ""},
		{"", []string{"repo"}, ""},
	}
	for _, test := range tests {
		if got := analysis.DidYouMean(test.word, test.candidates); got != test.want {
			t.Errorf("DidYouMean(%q) = %q, want %q", test.word, got, test.want)
		}
	}
}
