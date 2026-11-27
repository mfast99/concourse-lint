package docs_test

import (
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/docs"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestRulePages: every rule of the catalog has an embedded page with a
// title and the sections "What is checked?", "Example", "Fix" and
// "Suppress", for the hover in the editor and for
// concourse-lint explain.
func TestRulePages(t *testing.T) {
	for rule := range spectest.LoadCatalog(t) {
		page, found := docs.Rule(rule)
		if !found {
			t.Errorf("%s: no embedded page", rule)
			continue
		}
		if page.Title == "" || page.Checked == "" || page.Example == "" || page.Fix == "" || page.Suppress == "" {
			t.Errorf("%s: %+v: title and every section must be set", rule, page)
		}
		for _, section := range []string{page.Checked, page.Example, page.Fix, page.Suppress} {
			if strings.Contains(section, "\n## ") {
				t.Errorf("%s: a section runs into the next one", rule)
			}
		}
	}
	if _, found := docs.Rule("CL999"); found {
		t.Error("CL999 has a page, want none")
	}
}

// TestRulePageSections: the sections are taken without their heading and
// without the blank lines around them.
func TestRulePageSections(t *testing.T) {
	page, _ := docs.Rule("CL002")
	if page.Title != "get/put or a group refers to a missing resource" {
		t.Errorf("title = %q", page.Title)
	}
	if !strings.HasPrefix(page.Checked, "Every `get:` and `put:` step") || strings.HasSuffix(page.Checked, "\n") {
		t.Errorf("checked = %q", page.Checked)
	}
	if page.Fix != "Correct the name or add the resource under `resources:`." {
		t.Errorf("fix = %q", page.Fix)
	}
	if !strings.HasPrefix(page.Example, "```yaml\nresources:") || !strings.HasSuffix(page.Example, "```") {
		t.Errorf("example = %q", page.Example)
	}
	if !strings.HasPrefix(page.Suppress, "```yaml\n# concourse-lint:ignore CL002") || !strings.HasSuffix(page.Suppress, "See [suppression](../../spec/README.md#suppression).") {
		t.Errorf("suppress = %q", page.Suppress)
	}
}
