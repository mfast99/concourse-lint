package parse_test

import (
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/parse"
)

// TestOldVariableSyntaxHint: {{var}} makes the YAML invalid; the CL000 gets a
// hint to use ((var)) (found in pivotal-cf/pcf-pipelines).
func TestOldVariableSyntaxHint(t *testing.T) {
	_, findings := parse.Parse([]byte("resources:\n- name: repo\n  type: git\n  source:\n    private_key: {{git_private_key}}\n"))
	if len(findings) != 1 || findings[0].Rule != "CL000" {
		t.Fatalf("findings = %+v, want one CL000", findings)
	}
	if !strings.Contains(findings[0].Hint, "((name))") {
		t.Errorf("hint = %q, want a hint to ((name))", findings[0].Hint)
	}
}
