package lint

import (
	"bytes"
	"regexp"
)

var (
	// topLevelJobs matches a top-level jobs: key.
	topLevelJobs = regexp.MustCompile(`(?m)^jobs[ \t]*:`)
	// nextTopLevelKey matches a line that starts the next top-level key:
	// not indented, no sequence entry, no comment.
	nextTopLevelKey = regexp.MustCompile(`(?m)^[^\s#-]`)
	// planKey matches a plan: key in block or flow style. A step named
	// terraform-plan: is no plan: key.
	planKey = regexp.MustCompile(`(^|[\s{,])plan[ \t]*:`)
)

// LooksLikePipeline reports whether a YAML file is probably a pipeline: it
// has a top-level jobs: key with a plan: key in it. Front ends use it to skip
// task files, var files and other YAML with a jobs: key (BOSH release and
// deployment manifests, CI files of other tools) when they check a whole
// folder.
func LooksLikePipeline(source []byte) bool {
	jobsKey := topLevelJobs.FindIndex(source)
	if jobsKey == nil {
		return false
	}
	return planKey.Match(jobsBlock(source, jobsKey[1]))
}

// jobsBlock returns the value of the top-level jobs: key, from the end of
// the key to the next top-level key.
func jobsBlock(source []byte, valueStart int) []byte {
	block := source[valueStart:]
	lineEnd := bytes.IndexByte(block, '\n')
	if lineEnd < 0 {
		return block
	}
	next := nextTopLevelKey.FindIndex(block[lineEnd+1:])
	if next == nil {
		return block
	}
	return block[:lineEnd+1+next[0]]
}
