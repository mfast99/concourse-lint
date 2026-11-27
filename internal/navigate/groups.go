package navigate

import (
	"github.com/gobwas/glob"

	"github.com/mfast99/concourse-lint/internal/model"
)

// GroupPattern is one entry under jobs: of a group. Concourse treats every
// entry as a glob (github.com/gobwas/glob), so an exact name is a glob that
// matches only itself.
type GroupPattern struct {
	// The name of the group the entry belongs to.
	Group   string
	Entry   model.Value[string]
	matcher glob.Glob
	// Valid is false when the entry is no valid glob (Concourse rejects it)
	// or when it crashes fly.
	Valid bool
	// Crashes is true when the glob library panics on one of the job names,
	// which crashes fly (an unclosed or empty {} alternative at the end).
	Crashes bool
}

// GroupPatterns returns the job entries of all groups.
func GroupPatterns(pipeline *model.Pipeline) []GroupPattern {
	var patterns []GroupPattern
	jobs := JobNames(pipeline)
	for _, group := range pipeline.Groups {
		if group == nil {
			continue
		}
		for _, entry := range group.Jobs {
			if !Resolvable(entry) {
				continue
			}
			matcher, err := glob.Compile(entry.Value)
			crashes := err == nil && crashesOnAny(matcher, jobs)
			patterns = append(patterns, GroupPattern{Group: group.Name.Value, Entry: entry, matcher: matcher, Valid: err == nil && !crashes, Crashes: crashes})
		}
	}
	return patterns
}

// crashesOnAny reports whether the glob library panics when it matches the
// glob against one of the job names. fly matches every group entry against
// every job, so it crashes in exactly that case.
func crashesOnAny(matcher glob.Glob, jobs []string) (crashes bool) {
	defer func() {
		if recover() != nil {
			crashes = true
		}
	}()
	for _, job := range jobs {
		matcher.Match(job)
	}
	return false
}

// Matches reports whether the entry covers the job name.
func (pattern GroupPattern) Matches(jobName string) bool {
	return pattern.Valid && pattern.matcher.Match(jobName)
}

// IsGlob reports whether the entry contains glob characters.
func IsGlob(entry string) bool {
	for _, character := range entry {
		switch character {
		case '*', '?', '[', ']', '{', '}', '\\':
			return true
		}
	}
	return false
}
