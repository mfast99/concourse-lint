package mcp

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/lint"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// update rewrites the golden files instead of comparing against them:
//
//	go test ./internal/mcp -update
//
// Only use it after a deliberate change, and review the diff.
var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

// compareGolden compares output with testdata/<name>.golden.
func compareGolden(t *testing.T, name string, output []byte) {
	t.Helper()
	goldenPath := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, output, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("%v (write it by hand, or run with -update after a deliberate change)", err)
	}
	if string(output) != string(expected) {
		t.Errorf("output differs from %s\ngot:\n%s\nwant:\n%s", goldenPath, output, expected)
	}
}

// session sends requests after the handshake for a protocol version and
// returns their answers (without the answer to initialize).
func session(t *testing.T, protocolVersion string, lines ...string) []map[string]any {
	t.Helper()
	all := append([]string{initializeRequest(0, protocolVersion), notification("notifications/initialized")}, lines...)
	messages := exchange(t, Options{}, all...)
	if len(messages) != len(lines)+1 {
		t.Fatalf("%d answers, want %d", len(messages)-1, len(lines))
	}
	return messages[1:]
}

// callTool builds a tools/call request.
func callTool(id int, name string, arguments any) string {
	params := map[string]any{"name": name}
	if arguments != nil {
		params["arguments"] = arguments
	}
	return request(id, "tools/call", params)
}

// toolResult is the result of a tools/call, checked for its shape: one
// text block, and from 2025-06-18 on the same JSON as structuredContent.
type toolResult struct {
	text       string
	structured map[string]any
	isError    bool
}

func readToolResult(t *testing.T, message map[string]any, protocolVersion string) toolResult {
	t.Helper()
	result, isMap := message["result"].(map[string]any)
	if !isMap {
		t.Fatalf("no result: %v", message)
	}
	content, _ := result["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content = %v, want one block", result["content"])
	}
	block, _ := content[0].(map[string]any)
	text, _ := block["text"].(string)
	if block["type"] != "text" {
		t.Errorf("block type = %v, want text", block["type"])
	}
	isError, _ := result["isError"].(bool)
	structured, hasStructured := result["structuredContent"].(map[string]any)
	wantStructured := !isError && protocolVersion >= "2025-06-18"
	if hasStructured != wantStructured {
		t.Errorf("protocol %s, isError %v: structuredContent present = %v, want %v", protocolVersion, isError, hasStructured, wantStructured)
	}
	if hasStructured {
		var fromText map[string]any
		if err := json.Unmarshal([]byte(text), &fromText); err != nil {
			t.Errorf("text block is no JSON: %v", err)
		} else if !reflect.DeepEqual(fromText, structured) {
			t.Errorf("text block and structuredContent differ:\n%s\n%v", text, structured)
		}
	}
	return toolResult{text: text, structured: structured, isError: isError}
}

// TestToolsListGolden: names, descriptions and schemas of the tools.
func TestToolsListGolden(t *testing.T) {
	answers := session(t, latestProtocolVersion, request(1, "tools/list", nil))
	output, err := json.MarshalIndent(answers[0]["result"], "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "tools", append(output, '\n'))
}

// TestToolsListByVersion: annotations from 2025-03-26, outputSchema from
// 2025-06-18.
func TestToolsListByVersion(t *testing.T) {
	cases := []struct {
		version                           string
		wantAnnotations, wantOutputSchema bool
	}{
		{"2024-11-05", false, false},
		{"2025-03-26", true, false},
		{"2025-06-18", true, true},
		{"2025-11-25", true, true},
	}
	for _, testCase := range cases {
		answers := session(t, testCase.version, request(1, "tools/list", nil))
		result, _ := answers[0]["result"].(map[string]any)
		tools, _ := result["tools"].([]any)
		if len(tools) == 0 {
			t.Fatalf("%s: no tools", testCase.version)
		}
		for _, entry := range tools {
			tool := entry.(map[string]any)
			_, hasAnnotations := tool["annotations"]
			_, hasOutputSchema := tool["outputSchema"]
			if hasAnnotations != testCase.wantAnnotations || hasOutputSchema != testCase.wantOutputSchema {
				t.Errorf("%s %s: annotations %v, outputSchema %v", testCase.version, tool["name"], hasAnnotations, hasOutputSchema)
			}
			if _, hasInputSchema := tool["inputSchema"]; !hasInputSchema {
				t.Errorf("%s %s: no inputSchema", testCase.version, tool["name"])
			}
		}
	}
}

func TestToolsCallErrors(t *testing.T) {
	answers := session(t, latestProtocolVersion,
		callTool(1, "validate_pipeline", map[string]any{}),
		request(2, "tools/call", "not an object"),
		request(3, "tools/call", map[string]any{"arguments": map[string]any{}}),
	)
	for index, answer := range answers {
		if code := errorCode(t, answer); code != codeInvalidParams {
			t.Errorf("answer %d: code %d, want %d", index, code, codeInvalidParams)
		}
	}
}

func TestListRules(t *testing.T) {
	catalog := spectest.LoadCatalog(t)
	for _, version := range supportedProtocolVersions {
		answers := session(t, version, callTool(1, "list_rules", nil), callTool(2, "list_rules", map[string]any{}))
		for _, answer := range answers {
			result := readToolResult(t, answer, version)
			var document struct {
				Rules []struct {
					ID, Severity, Title, Docs string
					DefaultSeverity           string `json:"default_severity"`
				}
			}
			if err := json.Unmarshal([]byte(result.text), &document); err != nil {
				t.Fatal(err)
			}
			if len(document.Rules) != len(catalog) {
				t.Errorf("%s: %d rules, want %d", version, len(document.Rules), len(catalog))
			}
			for index, rule := range document.Rules {
				if index > 0 && document.Rules[index-1].ID >= rule.ID {
					t.Errorf("rules are not sorted by ID at %s", rule.ID)
				}
				want := string(catalog[rule.ID])
				if rule.Severity != want || rule.DefaultSeverity != want || rule.Title == "" || rule.Docs != analysis.DocsURL(rule.ID) {
					t.Errorf("rule %+v does not fit the catalog (%s)", rule, want)
				}
			}
		}
	}
}

func TestListRulesRejectsArguments(t *testing.T) {
	answers := session(t, latestProtocolVersion, callTool(1, "list_rules", map[string]any{"area": "security"}))
	if result := readToolResult(t, answers[0], latestProtocolVersion); !result.isError || !strings.Contains(result.text, "area") {
		t.Errorf("result = %+v, want a tool error naming the argument", result)
	}
}

// TestExplainRule: every rule of the catalog, the ID in any case.
func TestExplainRule(t *testing.T) {
	catalog := spectest.LoadCatalog(t)
	for id, severity := range catalog {
		answers := session(t, latestProtocolVersion, callTool(1, "explain_rule", map[string]any{"id": strings.ToLower(id)}))
		result := readToolResult(t, answers[0], latestProtocolVersion)
		if result.isError {
			t.Errorf("%s: tool error %s", id, result.text)
			continue
		}
		var explanation struct {
			ID, Severity, Title, Checked, Example, Fix, Suppress, Docs string
			DefaultSeverity                                            string `json:"default_severity"`
		}
		if err := json.Unmarshal([]byte(result.text), &explanation); err != nil {
			t.Fatal(err)
		}
		if explanation.ID != id || explanation.Severity != string(severity) || explanation.DefaultSeverity != string(severity) || explanation.Docs != analysis.DocsURL(id) {
			t.Errorf("%s: %+v", id, explanation)
		}
		if explanation.Title == "" || explanation.Checked == "" || explanation.Example == "" || explanation.Fix == "" || explanation.Suppress == "" {
			t.Errorf("%s: title and every section must be set: %+v", id, explanation)
		}
	}
}

func TestExplainRuleGolden(t *testing.T) {
	answers := session(t, latestProtocolVersion, callTool(1, "explain_rule", map[string]any{"id": "CL002"}))
	var output bytes.Buffer
	if err := json.Indent(&output, []byte(readToolResult(t, answers[0], latestProtocolVersion).text), "", "  "); err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "explain_rule_CL002", append(output.Bytes(), '\n'))
}

// TestRuleToolsWithConfig: list_rules and explain_rule show the severity
// under the config file of the working directory, read at every call.
func TestRuleToolsWithConfig(t *testing.T) {
	inTempProject(t, map[string]string{".concourse-lint.yml": "rules:\n  CS005: error\n  CL018: off\n"})
	answers := session(t, latestProtocolVersion, callTool(1, "list_rules", nil), callTool(2, "explain_rule", map[string]any{"id": "CS005"}))
	var list struct {
		Rules []lint.RuleInfo
	}
	if err := json.Unmarshal([]byte(readToolResult(t, answers[0], latestProtocolVersion).text), &list); err != nil {
		t.Fatal(err)
	}
	severities := map[string]string{}
	for _, rule := range list.Rules {
		severities[rule.ID] = rule.Severity + " " + rule.DefaultSeverity
	}
	if severities["CS005"] != "error warning" || severities["CL018"] != "off warning" || severities["CL002"] != "error error" {
		t.Errorf("list_rules severities = %v", severities)
	}
	var explanation lint.RuleExplanation
	if err := json.Unmarshal([]byte(readToolResult(t, answers[1], latestProtocolVersion).text), &explanation); err != nil {
		t.Fatal(err)
	}
	if explanation.Severity != "error" || explanation.DefaultSeverity != "warning" {
		t.Errorf("explain_rule CS005: severity %q, default %q", explanation.Severity, explanation.DefaultSeverity)
	}
}

// TestRuleToolsBrokenConfig: a broken config file is a tool error with its
// position, as for check_pipeline.
func TestRuleToolsBrokenConfig(t *testing.T) {
	inTempProject(t, map[string]string{".concourse-lint.yml": "rules:\n  CL999: off\n"})
	answers := session(t, latestProtocolVersion, callTool(1, "list_rules", nil), callTool(2, "explain_rule", map[string]any{"id": "CL002"}))
	for index, answer := range answers {
		if result := readToolResult(t, answer, latestProtocolVersion); !result.isError || !strings.Contains(result.text, ".concourse-lint.yml:2:3:") {
			t.Errorf("call %d: result = %+v, want a tool error at the position of CL999", index+1, result)
		}
	}
}

func TestExplainRuleErrors(t *testing.T) {
	cases := []struct {
		arguments any
		// wantText is a part of the error message.
		wantText string
	}{
		{map[string]any{"id": "CL02"}, `did you mean "CL002"?`},
		{map[string]any{"id": "nothing"}, `unknown rule "nothing"`},
		{map[string]any{}, "id"},
		{nil, "id"},
		{map[string]any{"id": 2}, "id"},
		{map[string]any{"id": "CL002", "verbose": true}, "verbose"},
		{"CL002", "object"},
	}
	for _, testCase := range cases {
		answers := session(t, latestProtocolVersion, callTool(1, "explain_rule", testCase.arguments))
		result := readToolResult(t, answers[0], latestProtocolVersion)
		if !result.isError || !strings.Contains(result.text, testCase.wantText) {
			t.Errorf("arguments %v: result %+v, want a tool error containing %q", testCase.arguments, result, testCase.wantText)
		}
	}
}
