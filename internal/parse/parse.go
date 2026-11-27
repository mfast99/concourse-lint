// Package parse reads YAML (with goccy/go-yaml) and builds the model from it.
// Syntax errors are reported as CL000, structural errors as CL001.
package parse

import (
	"bytes"
	"errors"
	"regexp"

	"github.com/mfast99/go-yaml"
	"github.com/mfast99/go-yaml/ast"
	"github.com/mfast99/go-yaml/lexer"
	"github.com/mfast99/go-yaml/parser"
	"github.com/mfast99/go-yaml/token"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// Parse builds a pipeline from the source. Even on errors it returns as much
// of the pipeline as possible so that the remaining rules keep running.
//
// A syntax error is the exception: goccy returns no AST then, so the result
// is exactly one CL000 and an empty pipeline (see spec/README.md).
func Parse(source []byte) (*model.Pipeline, []analysis.Finding) {
	// Concourse reads only the first document; later ones are never parsed
	// (CL024 instead of their syntax errors).
	source, laterDocument := firstDocument(source)
	var documentFindings []analysis.Finding
	if laterDocument != nil {
		documentFindings = append(documentFindings, *laterDocument)
	}
	file, err := parser.ParseBytes(source, 0)
	if err != nil {
		finding := syntaxError(err)
		if oldVariableSyntax.Match(source) {
			finding.Hint = "{{name}} is the old variable syntax that Concourse no longer supports, use ((name))"
		}
		return &model.Pipeline{}, append(documentFindings, finding)
	}
	body := documentBody(file)
	// Like a syntax error, a broken alias or an alias bomb makes Concourse
	// reject the whole file, so there is only that one CL000.
	aliasTargets, aliasProblem := resolveAliases(body)
	if aliasProblem != nil {
		return &model.Pipeline{}, append(documentFindings, *aliasProblem)
	}
	decoder := &pipelineDecoder{lastLine: lastLine(source), aliasTargets: aliasTargets}
	pipeline := decoder.pipeline(body)
	sourceTokens := lexer.Tokenize(string(source))
	pipeline.Comments = comments(sourceTokens)
	findings := append(documentFindings, decoder.findings...)
	return pipeline, append(findings, literalTabs(sourceTokens)...)
}

// documentBody returns the body of the document Concourse reads: the first
// one that is not a directive. goccy returns a directive (%YAML, %TAG) as a
// document of its own. A file with only comments has one document with no
// body. internal/config has the same function for the config file.
func documentBody(file *ast.File) ast.Node {
	for _, document := range file.Docs {
		if _, isDirective := document.Body.(*ast.DirectiveNode); !isDirective {
			return document.Body
		}
	}
	return nil
}

// comments returns all comments of the file. The lexer knows what is a
// comment and what is a "#" inside a string.
func comments(sourceTokens token.Tokens) []model.Comment {
	var found []model.Comment
	for _, sourceToken := range sourceTokens {
		if sourceToken.Type != token.CommentType || sourceToken.Position == nil {
			continue
		}
		found = append(found, model.Comment{
			Position: tokenPosition(sourceToken),
			Text:     sourceToken.Value,
		})
	}
	return found
}

// oldVariableSyntax matches {{name}}, the variable syntax of Concourse before
// version 3, which YAML reads as a flow mapping in a flow mapping.
var oldVariableSyntax = regexp.MustCompile(`\{\{\s*[\w.-]+\s*\}\}`)

// lastLine returns the number of the last line, not counting the empty
// "line" after a final newline.
func lastLine(source []byte) int {
	lineCount := bytes.Count(source, []byte("\n"))
	if len(source) > 0 && source[len(source)-1] != '\n' {
		lineCount++
	}
	return max(lineCount, 1)
}

// syntaxError turns a goccy error into a CL000 finding at the position goccy
// reports. Without a position it falls back to the start of the file.
func syntaxError(err error) analysis.Finding {
	finding := analysis.Finding{
		Rule:     "CL000",
		Severity: analysis.SeverityError,
		Position: model.Position{Line: 1, Column: 1},
		Message:  "YAML syntax error: " + err.Error(),
	}
	var yamlError yaml.Error
	if errors.As(err, &yamlError) {
		finding.Message = "YAML syntax error: " + yamlError.GetMessage()
		if errorToken := yamlError.GetToken(); errorToken != nil && errorToken.Position != nil {
			finding.Position = tokenPosition(errorToken)
		}
	}
	return finding
}

// Rules describe the rules the parser reports itself (no Run: the parser
// finds them while reading). Severity and title as in spec/catalog.txt.
var Rules = []analysis.Rule{
	{ID: "CL000", Severity: analysis.SeverityError, Title: "YAML syntax error"},
	{ID: "CL001", Severity: analysis.SeverityError, Title: "Structural error"},
	{ID: "CL017", Severity: analysis.SeverityWarning, Title: "Unquoted value changes when Concourse reads it"},
	{ID: "CL018", Severity: analysis.SeverityWarning, Title: "Unknown field that Concourse ignores"},
	{ID: "CL019", Severity: analysis.SeverityError, Title: "Value fails when the step runs"},
	{ID: "CL020", Severity: analysis.SeverityWarning, Title: "Unknown keyword that Concourse ignores"},
	{ID: "CL021", Severity: analysis.SeverityInfo, Title: "String contains a literal tab"},
	{ID: TemplateRule, Severity: analysis.SeverityInfo, Title: "Template instead of a pipeline"},
	{ID: FragmentRule, Severity: analysis.SeverityInfo, Title: "Pipeline fragment instead of a pipeline"},
	{ID: LaterDocumentRule, Severity: analysis.SeverityWarning, Title: "Content after the first YAML document"},
}
