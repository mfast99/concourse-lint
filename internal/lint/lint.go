// Package lint is the entry point to the core: parse, run all rules, apply
// suppression comments, sort. CLI, LSP and MCP call Check; the editor
// features get the model from Parse and ask navigate.
package lint

import (
	"bytes"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/config"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/parse"
	"github.com/mfast99/concourse-lint/internal/rules"
)

// Check analyzes a pipeline with the default configuration and returns all
// findings, sorted by line and column.
func Check(filename string, source []byte) []analysis.Finding {
	return CheckWithConfig(filename, source, nil)
}

// CheckWithConfig analyzes a pipeline with a config file (nil: the
// defaults): rules that are off do not run, configured severities replace
// the rules' own, and the rule options reach the rules.
func CheckWithConfig(filename string, source []byte, config *config.Config) []analysis.Finding {
	source = normalizeNewlines(source)
	pipeline, findings := parse.Parse(source)
	if template, isTemplate := activeTemplate(pipeline, source, config); isTemplate {
		return applyConfig([]analysis.Finding{template}, config)
	}
	for _, rule := range rules.All() {
		if config.IsOff(rule.ID) {
			continue
		}
		pass := analysis.NewPass(rule, pipeline, &findings)
		pass.Settings = config.RuleSettings()
		rule.Run(pass)
	}
	findings = readSuppressions(pipeline.Comments, source).apply(findings)
	exactSuggestions(findings, source)
	findings = applyConfig(findings, config)
	sort.SliceStable(findings, func(firstIndex, secondIndex int) bool {
		first, second := findings[firstIndex].Position, findings[secondIndex].Position
		return first.Line < second.Line || (first.Line == second.Line && first.Column < second.Column)
	})
	return removeDuplicates(findings)
}

// activeTemplate returns the CL022 finding of a template (ytt, spruce, ERB,
// Jinja) or the CL023 finding of a pipeline fragment. Such a file gets only
// this finding: it is no pipeline before it is rendered or merged, and the
// other rules would report false positives. When the rule is off or
// suppressed, the file is checked like a pipeline (spec/README.md,
// "Templates" and "Fragments").
func activeTemplate(pipeline *model.Pipeline, source []byte, config *config.Config) (analysis.Finding, bool) {
	for _, detect := range []func([]byte) (analysis.Finding, bool){parse.Template, parse.Fragment} {
		finding, found := detect(source)
		if !found || config.IsOff(finding.Rule) {
			continue
		}
		if readSuppressions(pipeline.Comments, source).suppresses(finding) {
			continue
		}
		return finding, true
	}
	return analysis.Finding{}, false
}

// isActiveTemplate reports whether a file is a template or a fragment that
// gets only CL022 or CL023. CheckPaths skips such files in a folder like
// task files.
func isActiveTemplate(source []byte, config *config.Config) bool {
	source = normalizeNewlines(source)
	_, isTemplate := parse.Template(source)
	_, isFragment := parse.Fragment(source)
	if !isTemplate && !isFragment {
		return false
	}
	pipeline, _ := parse.Parse(source)
	_, isActive := activeTemplate(pipeline, source, config)
	return isActive
}

// exactSuggestions turns the suggestion of every finding into the exact
// text for its range: a quoted typo keeps its quotes ("rep" becomes "repo",
// not repo). The rules only know the word; the quotes are in the source.
func exactSuggestions(findings []analysis.Finding, source []byte) {
	lines := strings.Split(string(source), "\n")
	for index := range findings {
		finding := &findings[index]
		written, found := textInRange(lines, finding.Position)
		if finding.Suggestion == "" || !found || len(written) < 2 {
			continue
		}
		quote := written[0]
		if (quote == '"' || quote == '\'') && written[len(written)-1] == quote {
			finding.Suggestion = string(quote) + finding.Suggestion + string(quote)
		}
	}
}

// textInRange returns the text of a range on one line; columns count
// characters, from 1.
func textInRange(lines []string, position model.Position) (string, bool) {
	if position.Line < 1 || position.Line > len(lines) || position.EndLine != position.Line {
		return "", false
	}
	characters := []rune(lines[position.Line-1])
	start, end := position.Column-1, position.EndColumn-1
	if start < 0 || end > len(characters) || start >= end {
		return "", false
	}
	return string(characters[start:end]), true
}

// Parse reads the pipeline like Check does, for the editor features
// (packages navigate and complete). Even with errors it returns as much as
// possible; readable is false after a syntax error (CL000), when the
// pipeline is empty.
func Parse(source []byte) (pipeline *model.Pipeline, readable bool) {
	pipeline, findings := parse.Parse(normalizeNewlines(source))
	readable = !slices.ContainsFunc(findings, func(finding analysis.Finding) bool { return finding.Rule == "CL000" })
	return pipeline, readable
}

// applyConfig drops the findings of rules that are off (also the ones the
// parser and the suppression comments report) and sets the configured
// severities.
func applyConfig(findings []analysis.Finding, config *config.Config) []analysis.Finding {
	if config == nil {
		return findings
	}
	var kept []analysis.Finding
	for _, finding := range findings {
		if config.IsOff(finding.Rule) {
			continue
		}
		finding.Severity = config.Severity(finding.Rule, finding.Severity)
		kept = append(kept, finding)
	}
	return kept
}

// LoadConfig reads and checks a config file.
func LoadConfig(path string) (*config.Config, error) {
	return config.Load(path, RuleIDs())
}

// ConfigFor finds and loads the config file for the file at path (see
// spec/README.md, "Config file"). It returns nil without an error when there
// is none.
func ConfigFor(path string) (*config.Config, error) {
	configPath, err := config.Find(filepath.Dir(path))
	if err != nil || configPath == "" {
		return nil, err
	}
	return LoadConfig(configPath)
}

// removeDuplicates keeps only the first finding per rule and position. The
// decoder reads an anchor again at every alias, so a
// problem inside an anchor is found once per use, always at the same place.
// The message is not compared: it may name the job of each use.
func removeDuplicates(findings []analysis.Finding) []analysis.Finding {
	type ruleAtPosition struct {
		rule     string
		position model.Position
	}
	seen := map[ruleAtPosition]bool{}
	var unique []analysis.Finding
	for _, finding := range findings {
		identity := ruleAtPosition{finding.Rule, finding.Position}
		if seen[identity] {
			continue
		}
		seen[identity] = true
		unique = append(unique, finding)
	}
	return unique
}

// normalizeNewlines turns CRLF and a lone CR into LF, so that the parser, the
// rules and suppression comments all count lines the same way. YAML and
// editors (LSP) treat all three as one line break; goccy miscounts lines
// with CRLF. Columns do not change because a CR only ever ends a line.
func normalizeNewlines(source []byte) []byte {
	source = bytes.ReplaceAll(source, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(source, []byte("\r"), []byte("\n"))
}
