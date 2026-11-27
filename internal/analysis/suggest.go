package analysis

import (
	"fmt"
	"unicode/utf8"
)

// ClosestName returns the candidate closest to word, for a hint like
// "did you mean "source"?". A candidate counts only if it can be reached
// with at most a third as many single-character edits as word is long (at
// least one). Of equally close candidates the alphabetically first wins, so
// the same typo always gets the same hint.
func ClosestName(word string, candidates []string) (closest string, found bool) {
	if word == "" {
		return "", false
	}
	allowedEdits := max(1, utf8.RuneCountInString(word)/3)
	bestEdits := allowedEdits + 1
	for _, candidate := range candidates {
		edits := editDistance(word, candidate)
		if edits > allowedEdits {
			continue
		}
		if edits < bestEdits || (edits == bestEdits && candidate < closest) {
			closest, bestEdits = candidate, edits
		}
	}
	return closest, closest != ""
}

// editDistance is the smallest number of edits that turns one text into the
// other: insert, remove or replace a character, or swap two neighbors
// ("tsak" -> "task" is one edit). This is the optimal string alignment
// variant of the Damerau-Levenshtein distance.
func editDistance(first, second string) int {
	firstRunes, secondRunes := []rune(first), []rune(second)
	// distances[row][column] is the distance between the first row characters
	// of first and the first column characters of second.
	distances := make([][]int, len(firstRunes)+1)
	for row := range distances {
		distances[row] = make([]int, len(secondRunes)+1)
		distances[row][0] = row
	}
	for column := range distances[0] {
		distances[0][column] = column
	}
	for row := 1; row <= len(firstRunes); row++ {
		for column := 1; column <= len(secondRunes); column++ {
			replaceCost := 1
			if firstRunes[row-1] == secondRunes[column-1] {
				replaceCost = 0
			}
			distances[row][column] = min(
				distances[row-1][column]+1,
				distances[row][column-1]+1,
				distances[row-1][column-1]+replaceCost,
			)
			swapped := row > 1 && column > 1 &&
				firstRunes[row-1] == secondRunes[column-2] && firstRunes[row-2] == secondRunes[column-1]
			if swapped {
				distances[row][column] = min(distances[row][column], distances[row-2][column-2]+1)
			}
		}
	}
	return distances[len(firstRunes)][len(secondRunes)]
}

// DidYouMean returns the hint "did you mean "x"?" for the candidate closest
// to word, or "" if none is close enough (see ClosestName).
func DidYouMean(word string, candidates []string) string {
	closest, found := ClosestName(word, candidates)
	if !found {
		return ""
	}
	return fmt.Sprintf("did you mean %q?", closest)
}
