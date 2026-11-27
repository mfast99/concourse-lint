package lsp

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/config"
	"github.com/mfast99/concourse-lint/internal/lint"
	"github.com/mfast99/concourse-lint/internal/model"
)

// languageIDPipeline is the language ID of the VS Code extension for a
// file the user marked as a pipeline: it is checked even when its content
// does not look like one.
const languageIDPipeline = "concourse-pipeline"

const diagnosticSource = "concourse-lint"

// check checks an open document and publishes its diagnostics, unless the
// document changed or was closed in the meantime.
func (server *server) check(uri string) {
	server.mutex.Lock()
	open, isOpen := server.documents[uri]
	if !isOpen {
		server.mutex.Unlock()
		return
	}
	snapshot := *open
	server.mutex.Unlock()

	diagnostics := server.diagnose(snapshot)
	// Keep the model of a readable version for completion.
	server.namesModel(snapshot)

	server.mutex.Lock()
	defer server.mutex.Unlock()
	current, stillOpen := server.documents[uri]
	if !stillOpen || current.version != snapshot.version || current.text != snapshot.text {
		return
	}
	version := snapshot.version
	server.publish(uri, &version, diagnostics)
}

// diagnose returns the diagnostics of a document: the errors of a config
// file, or the findings of a pipeline.
func (server *server) diagnose(open document) []Diagnostic {
	if isConfigFile(open.path) {
		return configFileDiagnostics(open.text)
	}
	if open.languageID != languageIDPipeline && !lint.LooksLikePipeline([]byte(open.text)) {
		return nil
	}
	findings, err := server.findings(open)
	if err != nil {
		server.publishConfigError(err)
		return []Diagnostic{brokenConfigDiagnostic(err)}
	}
	index := newLineIndex(open.text)
	var diagnostics []Diagnostic
	for _, finding := range findings {
		diagnostics = append(diagnostics, toDiagnostic(finding, index))
	}
	return diagnostics
}

// findings checks a pipeline document with its config file. An excluded
// document has none; a broken config file is the error.
func (server *server) findings(open document) ([]analysis.Finding, error) {
	var configuration *config.Config
	if open.path != "" {
		loaded, err := server.configs.forFile(open.path)
		if err != nil {
			return nil, err
		}
		if loaded.Excluded(open.path) {
			return nil, nil
		}
		configuration = loaded
	}
	name := open.path
	if name == "" {
		name = open.uri
	}
	return lint.CheckWithConfig(name, []byte(open.text), configuration), nil
}

// toDiagnostic converts a finding.
func toDiagnostic(finding analysis.Finding, index lineIndex) Diagnostic {
	message := finding.Message
	if finding.Hint != "" {
		message += " (" + finding.Hint + ")"
	}
	return Diagnostic{
		Range:           index.toRange(finding.Position),
		Severity:        toSeverity(finding.Severity),
		Code:            finding.Rule,
		CodeDescription: &CodeDescription{Href: analysis.DocsURL(finding.Rule)},
		Source:          diagnosticSource,
		Message:         message,
	}
}

func toSeverity(severity analysis.Severity) int {
	switch severity {
	case analysis.SeverityError:
		return severityError
	case analysis.SeverityWarning:
		return severityWarning
	}
	return severityInformation
}

// brokenConfigDiagnostic is the one info a pipeline shows while its config
// file is broken: like the CLI, nothing is checked then.
func brokenConfigDiagnostic(err error) Diagnostic {
	return Diagnostic{
		Severity: severityInformation,
		Source:   diagnosticSource,
		Message:  "not checked: the config file is broken: " + err.Error(),
	}
}

func isConfigFile(path string) bool {
	return path != "" && slices.Contains(config.FileNames, filepath.Base(path))
}

// configFileDiagnostics checks the text of an open config file, also when
// it is not saved yet.
func configFileDiagnostics(text string) []Diagnostic {
	_, err := config.Parse([]byte(text), lint.RuleIDs())
	if err == nil {
		return nil
	}
	return []Diagnostic{configErrorDiagnostic(err, newLineIndex(text))}
}

// configErrorDiagnostic places a config error at its key or value, or at the
// start of the file when it has no position.
func configErrorDiagnostic(err error, index lineIndex) Diagnostic {
	diagnostic := Diagnostic{Severity: severityError, Source: diagnosticSource, Message: err.Error()}
	var configError *config.Error
	if errors.As(err, &configError) {
		diagnostic.Message = configError.Message
		if configError.Line > 0 {
			line := ""
			if configError.Line <= len(index.lines) {
				line = index.lines[configError.Line-1]
			}
			diagnostic.Range = index.toRange(model.Position{
				Line: configError.Line, Column: configError.Column,
				EndLine: configError.Line, EndColumn: wordEnd(line, configError.Column),
			})
		}
	}
	return diagnostic
}

// wordEnd returns the column right after the key or value that starts at
// column: up to the closing quote, or up to a space, ":", ",", "]", "}" or
// "#".
func wordEnd(line string, column int) int {
	characters := []rune(line)
	start := column - 1
	if start < 0 || start >= len(characters) {
		return column
	}
	if quote := characters[start]; quote == '"' || quote == '\'' {
		for index := start + 1; index < len(characters); index++ {
			if characters[index] == quote {
				return index + 2
			}
		}
		return len(characters) + 1
	}
	end := start
	for end < len(characters) && !unicode.IsSpace(characters[end]) && !strings.ContainsRune(":,]}#", characters[end]) {
		end++
	}
	return max(end, start+1) + 1
}
