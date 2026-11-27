package main

import (
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/lint"
)

// TestRenderMarkdown: the Markdown of the rule pages as plain terminal
// text (spec/README.md, "Rules and explain").
func TestRenderMarkdown(t *testing.T) {
	cases := []struct {
		name     string
		markdown string
		want     string
	}{
		{
			name:     "inline code keeps its backticks, bold loses its stars",
			markdown: "Every **`get:`** step needs a resource.",
			want:     "Every `get:` step needs a resource.",
		},
		{
			name:     "link",
			markdown: "See [suppression](https://example.com/README.md#suppression).",
			want:     "See suppression (https://example.com/README.md#suppression).",
		},
		{
			name:     "lines of a paragraph are joined, paragraphs stay apart",
			markdown: "First line\nsecond line.\n\nNext paragraph.",
			want:     "First line second line.\n\nNext paragraph.",
		},
		{
			name: "paragraph wrapped at 80 columns",
			markdown: "A glob that ends in an unclosed or empty `{}` alternative (`build{`, `build{}`) is also reported: " +
				"the glob library Concourse uses panics on it.",
			want: "A glob that ends in an unclosed or empty `{}` alternative (`build{`, `build{}`)\n" +
				"is also reported: the glob library Concourse uses panics on it.",
		},
		{
			name:     "a code span is never broken",
			markdown: strings.Repeat("word ", 14) + "`fly set-pipeline` end",
			want:     strings.TrimSpace(strings.Repeat("word ", 14)) + "\n`fly set-pipeline` end",
		},
		{
			name:     "a word longer than a line stays whole",
			markdown: "See https://example.com/" + strings.Repeat("x", 80) + " here.",
			want:     "See\nhttps://example.com/" + strings.Repeat("x", 80) + "\nhere.",
		},
		{
			name:     "code block indented, never wrapped, blank lines kept empty",
			markdown: "```yaml\njobs:\n\n- name: " + strings.Repeat("x", 80) + "\n```",
			want:     "    jobs:\n\n    - name: " + strings.Repeat("x", 80),
		},
		{
			name:     "code block between paragraphs",
			markdown: "Before:\n\n```yaml\na: 1\n```\n\nAfter.",
			want:     "Before:\n\n    a: 1\n\nAfter.",
		},
		{
			name:     "list items with a hanging indent",
			markdown: "- one\n- two " + strings.Repeat("word ", 16) + "\n  more\n\nAfter.",
			want:     "- one\n- two " + strings.TrimSpace(strings.Repeat("word ", 15)) + "\n  word more\n\nAfter.",
		},
		{
			name:     "heading underlined with dashes",
			markdown: "## Fix\n\nCorrect it.",
			want:     "Fix\n---\n\nCorrect it.",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := renderMarkdown(testCase.markdown, textStyle{}); got != testCase.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, testCase.want)
			}
		})
	}
}

// TestRenderRulePages: every section of every rule page fits in 80 columns
// (code and long words aside), and colors only add escape sequences.
func TestRenderRulePages(t *testing.T) {
	for _, id := range lint.RuleIDs() {
		explanation, err := lint.ExplainRule(id, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, section := range []string{explanation.Checked, explanation.Example, explanation.Fix, explanation.Suppress} {
			plain := renderMarkdown(section, textStyle{})
			colored := renderMarkdown(section, textStyle{colors: true})
			if stripped := escapeSequence.ReplaceAllString(colored, ""); stripped != plain {
				t.Errorf("%s: with colors the text differs:\n%s\nwant:\n%s", id, stripped, plain)
			}
			for _, line := range strings.Split(plain, "\n") {
				if len(line) > lineWidth && !strings.HasPrefix(line, codeIndent) && strings.Contains(line, " ") {
					t.Errorf("%s: line longer than %d columns: %q", id, lineWidth, line)
				}
			}
		}
	}
}

// TestRulePagesUseTheSubset guards the renderer: every section of every
// rule page uses only the Markdown it knows. A new construct needs the
// renderer first.
func TestRulePagesUseTheSubset(t *testing.T) {
	for _, id := range lint.RuleIDs() {
		explanation, _ := lint.ExplainRule(id, nil)
		for _, section := range []string{explanation.Checked, explanation.Example, explanation.Fix, explanation.Suppress} {
			if err := checkMarkdownSubset(section); err != nil {
				t.Errorf("%s: %v", id, err)
			}
		}
	}
}

// TestCheckMarkdownSubset: what the guard rejects.
func TestCheckMarkdownSubset(t *testing.T) {
	accepted := []string{
		"Text with `code`, **bold** and a [link](https://example.com).",
		"A glob (`*`, `deploy-*`) and `\\t` in code.",
		"- item\n  continued\n- item",
		"## Heading\n\nText.",
		"```yaml\n<anything> | **here** > _x_\n```",
	}
	for _, markdown := range accepted {
		if err := checkMarkdownSubset(markdown); err != nil {
			t.Errorf("%q: %v, want accepted", markdown, err)
		}
	}
	rejected := []string{
		"| a | b |",
		"> quote",
		"### deep heading",
		"1. numbered",
		"Some *emphasis* here.",
		"Some _emphasis_ here.",
		"An <b>HTML</b> tag.",
		"An escaped \\* star.",
		"An unclosed `code span.",
		"![image](picture.png)",
		"---",
		"    indented code",
		"```yaml\nnever closed",
	}
	for _, markdown := range rejected {
		if err := checkMarkdownSubset(markdown); err == nil {
			t.Errorf("%q: accepted, want an error", markdown)
		}
	}
}
