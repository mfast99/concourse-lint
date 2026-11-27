package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/mfast99/concourse-lint/internal/lint"
)

var listRulesTool = tool{
	name:        "list_rules",
	description: "List every concourse-lint rule with its ID, severity in this project (error, warning, info, or off when the config file switches it off), default severity, title and documentation link.",
	inputSchema: `{"type": "object", "properties": {}, "additionalProperties": false}`,
	outputSchema: `{
		"type": "object",
		"properties": {
			"rules": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"id": {"type": "string"},
						"severity": {"type": "string", "enum": ["error", "warning", "info", "off"], "description": "The severity under the config file of the working directory."},
						"default_severity": {"type": "string", "enum": ["error", "warning", "info"]},
						"title": {"type": "string"},
						"docs": {"type": "string"}
					},
					"required": ["id", "severity", "default_severity", "title", "docs"]
				}
			}
		},
		"required": ["rules"]
	}`,
	call: listRules,
}

var explainRuleTool = tool{
	name:        "explain_rule",
	description: "Explain a concourse-lint rule: its title, severity in this project, default severity, what it checks, an example, how to fix a finding and how to suppress it (Markdown from the rule's documentation).",
	inputSchema: `{
		"type": "object",
		"properties": {
			"id": {"type": "string", "description": "The rule ID, e.g. CL002 (case-insensitive)."}
		},
		"required": ["id"],
		"additionalProperties": false
	}`,
	outputSchema: `{
		"type": "object",
		"properties": {
			"id": {"type": "string"},
			"severity": {"type": "string", "enum": ["error", "warning", "info", "off"], "description": "The severity under the config file of the working directory."},
			"default_severity": {"type": "string", "enum": ["error", "warning", "info"]},
			"title": {"type": "string"},
			"checked": {"type": "string", "description": "What the rule checks, as Markdown."},
			"example": {"type": "string", "description": "A pipeline that gets a finding, as Markdown."},
			"fix": {"type": "string", "description": "How to fix a finding, as Markdown."},
			"suppress": {"type": "string", "description": "How to suppress a finding, as Markdown."},
			"docs": {"type": "string"}
		},
		"required": ["id", "severity", "default_severity", "title", "checked", "example", "fix", "suppress", "docs"]
	}`,
	call: explainRule,
}

type ruleList struct {
	Rules []lint.RuleInfo `json:"rules"`
}

// listRules gives the same document as concourse-lint rules --format json
// in the working directory.
func listRules(_ *server, arguments json.RawMessage) ([]byte, error) {
	var noArguments struct{}
	if err := decodeArguments(arguments, &noArguments); err != nil {
		return nil, err
	}
	_, ruleConfig, err := lint.FindConfig(".")
	if err != nil {
		return nil, err
	}
	return encodeJSON(ruleList{Rules: lint.RuleList(ruleConfig)})
}

type explainRuleArguments struct {
	// A pointer, so that a missing ID differs from an empty one.
	ID *string `json:"id"`
}

// explainRule gives the same document as concourse-lint explain --format
// json <id> in the working directory.
func explainRule(_ *server, arguments json.RawMessage) ([]byte, error) {
	var parsed explainRuleArguments
	if err := decodeArguments(arguments, &parsed); err != nil {
		return nil, err
	}
	if parsed.ID == nil {
		return nil, fmt.Errorf("explain_rule needs the argument \"id\", a rule ID such as CL002")
	}
	_, ruleConfig, err := lint.FindConfig(".")
	if err != nil {
		return nil, err
	}
	explanation, err := lint.ExplainRule(*parsed.ID, ruleConfig)
	if err != nil {
		return nil, fmt.Errorf("%w; list_rules lists all rules", err)
	}
	return encodeJSON(explanation)
}
