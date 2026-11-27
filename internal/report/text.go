package report

import (
	"fmt"
	"io"
)

// Text writes one line per finding: file:line:col: severity RULE message,
// followed by the hint in parentheses if there is one. Inputs that could not
// be read are not written here; the front end reports them on stderr.
func Text(writer io.Writer, result Result) error {
	for _, file := range result.Files {
		for _, finding := range file.Findings {
			_, err := fmt.Fprintf(writer, "%s:%d:%d: %s %s %s\n",
				file.Path, finding.Position.Line, finding.Position.Column,
				finding.Severity, finding.Rule, messageWithHint(finding))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
