package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mfast99/concourse-lint/internal/lint"
	"github.com/mfast99/concourse-lint/internal/report"
)

// textName is the file name of text checked without a filename.
const textName = "<text>"

var checkPipelineTool = tool{
	name: "check_pipeline",
	description: "Check a Concourse CI pipeline for errors, security issues and bad practice, offline. " +
		"Pass either text, the pipeline YAML (optionally with filename, which also decides which .concourse-lint.yml applies), " +
		"or paths, files or folders inside the working directory (folders are searched for pipelines; the nearest .concourse-lint.yml applies). " +
		"Returns the same JSON as `concourse-lint check --format json`: findings with file, rule, severity (error, warning, info), " +
		"pos (line and col from 1, end_line and end_col right after the range), message, hint, suggestion " +
		"(the exact replacement text for the range, or empty) and docs (link to the rule page), plus errors for inputs that could not be read.",
	inputSchema: `{
		"type": "object",
		"properties": {
			"text": {"type": "string", "description": "The pipeline YAML to check."},
			"filename": {"type": "string", "description": "Only with text: the file name used in the findings, relative to the working directory; the config file is searched from its folder."},
			"paths": {"type": "array", "items": {"type": "string"}, "minItems": 1, "description": "Pipeline files or folders to check, absolute or relative to the working directory, inside it."}
		},
		"additionalProperties": false
	}`,
	outputSchema: checkOutputSchema,
	call:         checkPipeline,
}

// checkOutputSchema describes the JSON document of the CLI contract.
const checkOutputSchema = `{
	"type": "object",
	"properties": {
		"findings": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"file": {"type": "string"},
					"rule": {"type": "string"},
					"severity": {"type": "string", "enum": ["error", "warning", "info"]},
					"pos": {
						"type": "object",
						"properties": {
							"line": {"type": "integer"},
							"col": {"type": "integer"},
							"end_line": {"type": "integer"},
							"end_col": {"type": "integer"}
						},
						"required": ["line", "col", "end_line", "end_col"]
					},
					"message": {"type": "string"},
					"hint": {"type": "string"},
					"suggestion": {"type": "string"},
					"docs": {"type": "string"}
				},
				"required": ["file", "rule", "severity", "pos", "message", "hint", "suggestion", "docs"]
			}
		},
		"errors": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"file": {"type": "string"},
					"message": {"type": "string"}
				},
				"required": ["file", "message"]
			}
		}
	},
	"required": ["findings", "errors"]
}`

type checkPipelineArguments struct {
	// Pointers, so that a missing value differs from an empty one.
	Text     *string   `json:"text"`
	Filename *string   `json:"filename"`
	Paths    *[]string `json:"paths"`
}

// checkPipeline checks text as one pipeline, or files and folders like the
// CLI (spec/README.md, "MCP server contract").
func checkPipeline(server *server, arguments json.RawMessage) ([]byte, error) {
	var parsed checkPipelineArguments
	if err := decodeArguments(arguments, &parsed); err != nil {
		return nil, err
	}
	switch {
	case parsed.Text != nil && parsed.Paths != nil:
		return nil, errors.New("check_pipeline needs text or paths, exactly one, not both")
	case parsed.Paths != nil:
		if parsed.Filename != nil {
			return nil, errors.New("filename only goes with text; paths are reported as given")
		}
		return server.checkPaths(*parsed.Paths)
	case parsed.Text == nil:
		return nil, errors.New("check_pipeline needs text or paths, exactly one: text is the pipeline YAML, paths are files or folders inside the working directory")
	}
	displayName, configFolder := textName, "."
	if parsed.Filename != nil && *parsed.Filename != "" {
		if err := server.insideWorkingDirectory(*parsed.Filename); err != nil {
			return nil, fmt.Errorf("filename: %w", err)
		}
		displayName, configFolder = *parsed.Filename, filepath.Dir(*parsed.Filename)
	}
	config, err := lint.ConfigFor(filepath.Join(configFolder, "pipeline.yml"))
	if err != nil {
		return nil, err
	}
	findings := lint.CheckWithConfig(displayName, []byte(*parsed.Text), config)
	return writeReport(report.Result{Files: []report.File{{Path: displayName, Findings: findings}}})
}

// checkPaths checks files and folders like the CLI. Every path given must
// lie inside the working directory, or nothing is checked; a file found in
// a folder that lies outside (a symlink) is not read and goes into errors.
func (server *server) checkPaths(paths []string) ([]byte, error) {
	if len(paths) == 0 {
		return nil, errors.New("paths must not be empty")
	}
	for _, path := range paths {
		if path == "" {
			return nil, errors.New("paths: a path must not be empty")
		}
		if path == "-" {
			return nil, errors.New(`paths: "-" (stdin) is not available here; pass the YAML as text`)
		}
		if err := server.insideWorkingDirectory(path); err != nil {
			return nil, fmt.Errorf("paths: %w; nothing was checked", err)
		}
	}
	checked, err := lint.CheckPaths(paths, lint.PathOptions{Allow: server.insideWorkingDirectory})
	if err != nil {
		return nil, err
	}
	result := report.Result{}
	for _, file := range checked.Files {
		result.Files = append(result.Files, report.File{Path: file.Path, Findings: file.Findings})
	}
	for _, inputError := range checked.Errors {
		result.Errors = append(result.Errors, report.InputError{Path: inputError.Path, Message: inputError.Message})
	}
	return writeReport(result)
}

// writeReport writes the result exactly as `check --format json` does.
func writeReport(result report.Result) ([]byte, error) {
	var output bytes.Buffer
	err := report.JSON(&output, result)
	return output.Bytes(), err
}

// insideWorkingDirectory returns an error when path (absolute, or relative
// to the working directory) lies outside the working directory. Symlinks
// are resolved as the operating system follows them, so "link/../x" counts
// where it really points; a path that does not exist is compared as
// written (reading it fails anyway).
func (server *server) insideWorkingDirectory(path string) error {
	absolute := path
	if !filepath.IsAbs(path) {
		// Not filepath.Join: it would remove "link/.." as text, before the
		// symlink is followed.
		absolute = server.workingDirectory + string(filepath.Separator) + path
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		resolved = filepath.Clean(absolute)
	}
	relative, err := filepath.Rel(server.workingDirectory, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%q is outside the working directory %s", path, server.workingDirectory)
	}
	return nil
}
