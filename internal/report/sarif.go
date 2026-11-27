package report

import (
	"io"
	"maps"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// The types below are the part of SARIF 2.1.0 that concourse-lint writes.
// Field names and nesting follow the SARIF specification.

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool        sarifTool         `json:"tool"`
	Invocations []sarifInvocation `json:"invocations"`
	ColumnKind  string            `json:"columnKind"`
	Results     []sarifResult     `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version,omitempty"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID      string `json:"id"`
	HelpURI string `json:"helpUri"`
}

type sarifInvocation struct {
	ExecutionSuccessful        bool                `json:"executionSuccessful"`
	ToolExecutionNotifications []sarifNotification `json:"toolExecutionNotifications,omitempty"`
}

type sarifNotification struct {
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifResult struct {
	RuleID string `json:"ruleId"`
	// No omitempty: index 0 is a valid index.
	RuleIndex int             `json:"ruleIndex"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
	// The suggestion of a finding as a standard SARIF fix.
	Fixes []sarifFix `json:"fixes,omitempty"`
}

type sarifFix struct {
	Description     sarifMessage          `json:"description"`
	ArtifactChanges []sarifArtifactChange `json:"artifactChanges"`
}

type sarifArtifactChange struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Replacements     []sarifReplacement    `json:"replacements"`
}

type sarifReplacement struct {
	DeletedRegion   sarifRegion  `json:"deletedRegion"`
	InsertedContent sarifMessage `json:"insertedContent"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	// A pointer, so that "no region" (unknown position) can be left out.
	Region *sarifRegion `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn,omitempty"`
	// End right after the last character; left out without an end.
	EndLine   int `json:"endLine,omitempty"`
	EndColumn int `json:"endColumn,omitempty"`
}

const (
	sarifSchema        = "https://json.schemastore.org/sarif-2.1.0.json"
	sarifVersion       = "2.1.0"
	toolName           = "concourse-lint"
	toolInformationURI = "https://github.com/mfast99/concourse-lint"
	// The parser (goccy) counts columns in characters, not in bytes or
	// UTF-16 units. SARIF's default would be UTF-16.
	sarifColumnKind = "unicodeCodePoints"
	// "<" and ">" are not allowed in a URI, so stdin is written as "stdin".
	sarifStdinURI = "stdin"
)

// SARIF writes the result as a SARIF 2.1.0 log for GitHub Code Scanning:
// one run, one result per finding, and inputs that could not be read as
// tool execution notifications.
//
// The driver lists the rules that fired, each with a link to its page in
// docs/rules/.
func SARIF(writer io.Writer, result Result) error {
	ruleIDs := firedRuleIDs(result)
	run := sarifRun{
		Tool: sarifTool{Driver: sarifDriver{
			Name:           toolName,
			Version:        result.ToolVersion,
			InformationURI: toolInformationURI,
			Rules:          []sarifRule{},
		}},
		Invocations: []sarifInvocation{{ExecutionSuccessful: len(result.Errors) == 0}},
		ColumnKind:  sarifColumnKind,
		Results:     []sarifResult{},
	}
	for _, ruleID := range ruleIDs {
		run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, sarifRule{ID: ruleID, HelpURI: analysis.DocsURL(ruleID)})
	}
	for _, file := range result.Files {
		for _, finding := range file.Findings {
			run.Results = append(run.Results, sarifResult{
				RuleID:    finding.Rule,
				RuleIndex: slices.Index(ruleIDs, finding.Rule),
				Level:     sarifLevel(finding.Severity),
				Message:   sarifMessage{Text: messageWithHint(finding)},
				Locations: []sarifLocation{sarifLocationAt(file.Path, finding.Position)},
				Fixes:     sarifFixes(file.Path, finding),
			})
		}
	}
	invocation := &run.Invocations[0]
	for _, inputError := range result.Errors {
		invocation.ToolExecutionNotifications = append(invocation.ToolExecutionNotifications, sarifNotification{
			Level:     "error",
			Message:   sarifMessage{Text: inputError.Message},
			Locations: []sarifLocation{sarifLocationAt(inputError.Path, model.Position{})},
		})
	}
	return writeJSON(writer, sarifLog{Schema: sarifSchema, Version: sarifVersion, Runs: []sarifRun{run}})
}

// sarifFixes returns the suggestion as a fix that replaces the finding's
// region; none without a suggestion or without a complete region.
func sarifFixes(path string, finding analysis.Finding) []sarifFix {
	region := sarifLocationAt(path, finding.Position).PhysicalLocation.Region
	if finding.Suggestion == "" || region == nil || region.StartColumn == 0 || region.EndLine == 0 {
		return nil
	}
	return []sarifFix{{
		Description: sarifMessage{Text: finding.SuggestionTitle()},
		ArtifactChanges: []sarifArtifactChange{{
			ArtifactLocation: sarifArtifactLocation{URI: sarifURI(path)},
			Replacements:     []sarifReplacement{{DeletedRegion: *region, InsertedContent: sarifMessage{Text: finding.Suggestion}}},
		}},
	}}
}

// firedRuleIDs returns the IDs of all rules with a finding, sorted, each once.
func firedRuleIDs(result Result) []string {
	fired := map[string]bool{}
	for _, file := range result.Files {
		for _, finding := range file.Findings {
			fired[finding.Rule] = true
		}
	}
	return slices.Sorted(maps.Keys(fired))
}

// sarifLevel maps our severity to a SARIF level. SARIF has no "info"; "note"
// is its lowest level that is shown.
func sarifLevel(severity analysis.Severity) string {
	switch severity {
	case analysis.SeverityError:
		return "error"
	case analysis.SeverityInfo:
		return "note"
	default:
		return "warning"
	}
}

// sarifLocationAt is the location of a file and, if known, a position in it.
// SARIF requires line and column to be at least 1, so unknown parts are left
// out instead of being written as 0.
func sarifLocationAt(path string, position model.Position) sarifLocation {
	physicalLocation := sarifPhysicalLocation{ArtifactLocation: sarifArtifactLocation{URI: sarifURI(path)}}
	if position.Line >= 1 {
		physicalLocation.Region = &sarifRegion{StartLine: position.Line}
		if position.Column >= 1 {
			physicalLocation.Region.StartColumn = position.Column
		}
		if position.EndLine >= 1 && position.EndColumn >= 1 {
			physicalLocation.Region.EndLine, physicalLocation.Region.EndColumn = position.EndLine, position.EndColumn
		}
	}
	return sarifLocation{PhysicalLocation: physicalLocation}
}

// sarifURI turns a path from the command line into a SARIF URI: relative
// paths stay relative (GitHub resolves them against the repository), with
// "/" as separator and special characters percent-encoded; absolute paths
// become file:// URIs.
func sarifURI(path string) string {
	if path == StdinPath {
		return sarifStdinURI
	}
	slashPath := filepath.ToSlash(filepath.Clean(path))
	if filepath.IsAbs(path) {
		// A Windows path like C:/ci/pipeline.yml needs a leading slash to
		// become file:///C:/ci/pipeline.yml.
		if !strings.HasPrefix(slashPath, "/") {
			slashPath = "/" + slashPath
		}
		return (&url.URL{Scheme: "file", Path: slashPath}).String()
	}
	return (&url.URL{Path: slashPath}).String()
}
