package report

import (
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// GitHubEnvironment is what the GitHub writers need from a GitHub Actions
// run. The front end reads it from the environment variables; empty fields
// mean "not in a GitHub Actions run" or "unknown".
type GitHubEnvironment struct {
	// GITHUB_WORKSPACE: the repository root. Paths inside it are written
	// relative to it.
	Workspace string
	// The current directory, against which relative paths are resolved.
	WorkingDirectory string
	// GITHUB_SERVER_URL, e.g. "https://github.com".
	ServerURL string
	// GITHUB_REPOSITORY, e.g. "org/repo".
	Repository string
	// GITHUB_SHA: the checked commit, for links in the summary.
	Commit string
}

// githubTitle starts the title of every annotation, so that annotations of
// concourse-lint stand out next to those of other tools in the same run.
const githubTitle = "concourse-lint"

// summaryRowLimit keeps the step summary far below GitHub's limit of 1 MiB
// per step (a row is about 200 bytes).
const summaryRowLimit = 500

// githubLevels maps a severity to the workflow command of its annotation.
var githubLevels = map[analysis.Severity]string{
	analysis.SeverityError:   "error",
	analysis.SeverityWarning: "warning",
	analysis.SeverityInfo:    "notice",
}

// GitHub's escaping of workflow commands: the message must not contain a
// line break (it would end the command), property values also no ":" and
// "," (they separate the properties from each other and from the message).
var (
	messageEscaper  = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	propertyEscaper = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

// markdownEscaper makes Markdown show a text as written. "\r\n" comes before
// "\r" because the replacer tries the pairs in this order.
var markdownEscaper = strings.NewReplacer(
	"\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "|", "\\|",
	"&", "&amp;", "<", "&lt;", ">", "&gt;",
	"\r\n", " ", "\n", " ", "\r", " ",
)

// GitHub writes one workflow command per finding and per input that could
// not be read, which GitHub Actions shows as annotations
// (spec/README.md, "GitHub format").
func GitHub(writer io.Writer, result Result, environment GitHubEnvironment) error {
	for _, file := range result.Files {
		path, _ := environment.repositoryPath(file.Path)
		for _, finding := range file.Findings {
			if _, err := io.WriteString(writer, annotation(path, finding)+"\n"); err != nil {
				return err
			}
		}
	}
	for _, inputError := range result.Errors {
		path, _ := environment.repositoryPath(inputError.Path)
		_, err := fmt.Fprintf(writer, "::error file=%s,title=%s::%s\n",
			propertyEscaper.Replace(path), githubTitle, messageEscaper.Replace(inputError.Message))
		if err != nil {
			return err
		}
	}
	return nil
}

// annotation is the workflow command for one finding in the file at path.
func annotation(path string, finding analysis.Finding) string {
	level := githubLevels[finding.Severity]
	title := propertyEscaper.Replace(githubTitle + " " + finding.Rule)
	start := finding.Position
	message := messageWithHint(finding)
	// Without a file GitHub cannot place the annotation, so the position
	// goes into the message.
	if path == StdinPath {
		message = fmt.Sprintf("%s:%d:%d: %s", path, start.Line, start.Column, message)
		return fmt.Sprintf("::%s title=%s::%s", level, title, messageEscaper.Replace(message))
	}
	end := endOf(finding.Position)
	properties := fmt.Sprintf("file=%s,line=%d", propertyEscaper.Replace(path), start.Line)
	// GitHub accepts columns only when the annotation stays within one line.
	if end.Line == start.Line {
		properties += fmt.Sprintf(",col=%d,endLine=%d,endColumn=%d", start.Column, end.Line, end.Column)
	} else {
		properties += fmt.Sprintf(",endLine=%d", end.Line)
	}
	return fmt.Sprintf("::%s %s,title=%s::%s", level, properties, title, messageEscaper.Replace(message))
}

// repositoryPath returns path relative to the workspace, with / as
// separator, and true. It returns path as given and false for stdin, a path
// outside the workspace, or when there is no workspace.
func (environment GitHubEnvironment) repositoryPath(path string) (string, bool) {
	if environment.Workspace == "" || path == StdinPath {
		return path, false
	}
	absolutePath := path
	if !filepath.IsAbs(path) {
		absolutePath = filepath.Join(environment.WorkingDirectory, path)
	}
	relativePath, err := filepath.Rel(environment.Workspace, absolutePath)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return path, false
	}
	return filepath.ToSlash(relativePath), true
}

// GitHubSummary writes the Markdown step summary of all findings
// (spec/README.md, "GitHub format"). It is built completely before it is
// written, so that it goes to the file in one write.
func GitHubSummary(writer io.Writer, result Result, environment GitHubEnvironment) error {
	var summary strings.Builder
	summary.WriteString("### concourse-lint\n\n")
	summary.WriteString(countLine(result) + "\n")

	rowCount := 0
	for _, file := range result.Files {
		path, inRepository := environment.repositoryPath(file.Path)
		for _, finding := range file.Findings {
			if rowCount == 0 {
				summary.WriteString("\n| Location | Severity | Rule | Message |\n|---|---|---|---|\n")
			}
			rowCount++
			if rowCount > summaryRowLimit {
				continue
			}
			fmt.Fprintf(&summary, "| %s | %s | [%s](%s) | %s |\n",
				environment.location(path, inRepository, finding.Position), finding.Severity,
				finding.Rule, analysis.DocsURL(finding.Rule), markdownEscaper.Replace(messageWithHint(finding)))
		}
	}
	if rowCount > summaryRowLimit {
		fmt.Fprintf(&summary, "\n… and %s (use --format json to see all).\n", counted(rowCount-summaryRowLimit, "more finding"))
	}

	if len(result.Errors) > 0 {
		summary.WriteString("\n")
	}
	for _, inputError := range result.Errors {
		path, _ := environment.repositoryPath(inputError.Path)
		fmt.Fprintf(&summary, "- Could not read %s: %s\n", codeSpan(path), markdownEscaper.Replace(inputError.Message))
	}

	_, err := io.WriteString(writer, summary.String())
	return err
}

// countLine is e.g. "1 error, 2 warnings in 3 files." or "No findings in 1
// file.".
func countLine(result Result) string {
	counts := map[analysis.Severity]int{}
	for _, file := range result.Files {
		for _, finding := range file.Findings {
			counts[finding.Severity]++
		}
	}
	files := counted(len(result.Files), "file")
	var parts []string
	if counts[analysis.SeverityError] > 0 {
		parts = append(parts, counted(counts[analysis.SeverityError], "error"))
	}
	if counts[analysis.SeverityWarning] > 0 {
		parts = append(parts, counted(counts[analysis.SeverityWarning], "warning"))
	}
	// "info" has no plural.
	if counts[analysis.SeverityInfo] > 0 {
		parts = append(parts, fmt.Sprintf("%d info", counts[analysis.SeverityInfo]))
	}
	if len(parts) == 0 {
		return "No findings in " + files + "."
	}
	return strings.Join(parts, ", ") + " in " + files + "."
}

// counted is the number followed by the noun, with a plural "s" unless the
// number is 1.
func counted(number int, noun string) string {
	if number == 1 {
		return fmt.Sprintf("%d %s", number, noun)
	}
	return fmt.Sprintf("%d %ss", number, noun)
}

// location is the Location cell of a summary row: file:line:col as code,
// linked to the line in the repository when the path is in it and the run
// says which repository and commit were checked.
func (environment GitHubEnvironment) location(path string, inRepository bool, position model.Position) string {
	text := codeSpan(fmt.Sprintf("%s:%d:%d", path, position.Line, position.Column))
	if !inRepository || environment.ServerURL == "" || environment.Repository == "" || environment.Commit == "" {
		return text
	}
	segments := strings.Split(path, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	link := fmt.Sprintf("%s/%s/blob/%s/%s#L%d",
		strings.TrimSuffix(environment.ServerURL, "/"), environment.Repository, environment.Commit,
		strings.Join(segments, "/"), position.Line)
	return "[" + text + "](" + link + ")"
}

// codeSpan writes text as Markdown code. Inside a table a "|" would still
// end the cell, so it is escaped; GitHub removes the backslash.
func codeSpan(text string) string {
	return "`" + strings.ReplaceAll(text, "|", "\\|") + "`"
}
