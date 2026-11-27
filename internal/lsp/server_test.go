package lsp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestSpec opens every file of spec/ in the server, as a pipeline, and
// compares the diagnostics with the "# want" lines: the editor shows
// exactly what the CLI reports.
func TestSpec(t *testing.T) {
	catalog := spectest.LoadCatalog(t)
	client := startServer(t, Options{})
	client.initialize()
	for _, file := range spectest.Files(t) {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		uri := pathToURI(file)
		client.open(uri, languageIDPipeline, string(source))
		published, found := client.sync()[uri]
		if !found {
			t.Errorf("%s: no diagnostics published", spectest.Name(file))
			continue
		}
		got := toFindings(published.Diagnostics, string(source))
		for _, problem := range spectest.Compare(spectest.ParseExpectations(source), catalog, got) {
			t.Errorf("%s: %s", spectest.Name(file), problem)
		}
		client.notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": uri}})
	}
	if err := client.shutdown(); err != nil {
		t.Errorf("Serve = %v, want nil after shutdown and exit", err)
	}
}

// toFindings is the way back from diagnostics to findings, for spectest.
func toFindings(diagnostics []Diagnostic, text string) []analysis.Finding {
	index := newLineIndex(text)
	severities := map[int]analysis.Severity{
		severityError: analysis.SeverityError, severityWarning: analysis.SeverityWarning, severityInformation: analysis.SeverityInfo,
	}
	var findings []analysis.Finding
	for _, diagnostic := range diagnostics {
		line, column := index.fromPosition(diagnostic.Range.Start)
		findings = append(findings, analysis.Finding{
			Rule: diagnostic.Code, Severity: severities[diagnostic.Severity],
			Position: model.Position{Line: line, Column: column}, Message: diagnostic.Message,
		})
	}
	return findings
}

func TestInitialize(t *testing.T) {
	client := startServer(t, Options{Version: "v9.9.9"})
	response := client.initialize()
	result := response["result"].(map[string]any)
	capabilities := result["capabilities"].(map[string]any)
	if capabilities["positionEncoding"] != "utf-16" {
		t.Errorf("positionEncoding = %v", capabilities["positionEncoding"])
	}
	sync := capabilities["textDocumentSync"].(map[string]any)
	if sync["openClose"] != true || sync["change"] != float64(1) {
		t.Errorf("textDocumentSync = %v, want open/close and full text", sync)
	}
	if info := result["serverInfo"].(map[string]any); info["name"] != "concourse-lint" || info["version"] != "v9.9.9" {
		t.Errorf("serverInfo = %v", info)
	}
	if again := client.initialize(); again["error"] == nil {
		t.Error("a second initialize must fail")
	}
	if err := client.shutdown(); err != nil {
		t.Errorf("Serve = %v", err)
	}
}

// TestLifecycle: requests before initialize and after shutdown fail, an
// unknown method fails, and exit without shutdown is an error.
func TestLifecycle(t *testing.T) {
	client := startServer(t, Options{})
	if response := client.request("textDocument/hover", map[string]any{}, nil); errorCode(response) != codeServerNotInitialized {
		t.Errorf("before initialize: %v", response)
	}
	client.initialize()
	if response := client.request("textDocument/typeDefinition", map[string]any{}, nil); errorCode(response) != codeMethodNotFound {
		t.Errorf("unknown method: %v", response)
	}
	if response := client.request("shutdown", nil, nil); response["error"] != nil || !hasKey(response, "result") {
		t.Errorf("shutdown: %v, want result null", response)
	}
	if response := client.request("textDocument/hover", map[string]any{}, nil); errorCode(response) != codeInvalidRequest {
		t.Errorf("after shutdown: %v", response)
	}

	withoutShutdown := startServer(t, Options{})
	withoutShutdown.initialize()
	withoutShutdown.notify("exit", nil)
	if err := <-withoutShutdown.done; !errors.Is(err, ErrExitWithoutShutdown) {
		t.Errorf("exit without shutdown: %v", err)
	}
}

// TestWatcherRegistration: a client that offers dynamic registration of
// file watchers gets one for the config files after initialized; its
// response is not answered. Without the capability nothing is registered.
func TestWatcherRegistration(t *testing.T) {
	client := startServer(t, Options{})
	client.request("initialize", map[string]any{"capabilities": map[string]any{
		"workspace": map[string]any{"didChangeWatchedFiles": map[string]any{"dynamicRegistration": true}},
	}}, nil)
	client.notify("initialized", map[string]any{})
	registration := client.next()
	if registration["method"] != "client/registerCapability" || !hasKey(registration, "id") {
		t.Fatalf("after initialized: %v, want a client/registerCapability request", registration)
	}
	encoded, _ := json.Marshal(registration["params"])
	want := `{"registrations":[{"id":"concourse-lint/config-files","method":"workspace/didChangeWatchedFiles",` +
		`"registerOptions":{"watchers":[{"globPattern":"**/.concourse-lint.yml"},{"globPattern":"**/.concourse-lint.yaml"}]}}]}`
	if string(encoded) != want {
		t.Errorf("params = %s\nwant      %s", encoded, want)
	}
	client.send(map[string]any{"id": registration["id"], "result": nil})
	client.request("concourse-lint/sync", nil, func(message map[string]any) {
		t.Errorf("the server answered the client's response: %v", message)
	})
	if err := client.shutdown(); err != nil {
		t.Errorf("Serve = %v", err)
	}

	without := startServer(t, Options{})
	without.initialize()
	without.notify("initialized", map[string]any{})
	without.request("concourse-lint/sync", nil, func(message map[string]any) {
		t.Errorf("without dynamic registration: %v, want nothing", message)
	})
	if err := without.shutdown(); err != nil {
		t.Errorf("Serve = %v", err)
	}
}

func errorCode(response map[string]any) int {
	responseError, isError := response["error"].(map[string]any)
	if !isError {
		return 0
	}
	return int(responseError["code"].(float64))
}

func hasKey(object map[string]any, key string) bool {
	_, found := object[key]
	return found
}

// TestDetection: YAML that does not look like a pipeline gets no findings,
// unless the user marked it as one; a closed document is cleared.
func TestDetection(t *testing.T) {
	client := startServer(t, Options{})
	client.initialize()
	taskFile := "platform: linux\nrun: {path: echo}\nunknown_field: 1\n"
	client.open("untitled:task", "yaml", taskFile)
	if diagnostics := client.sync()["untitled:task"].Diagnostics; len(diagnostics) != 0 {
		t.Errorf("task file: %v, want none", diagnostics)
	}
	client.open("untitled:marked", languageIDPipeline, taskFile)
	if diagnostics := client.sync()["untitled:marked"].Diagnostics; len(diagnostics) == 0 {
		t.Error("a file marked as pipeline must be checked")
	}
	client.notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": "untitled:marked"}})
	if published, found := client.sync()["untitled:marked"]; !found || len(published.Diagnostics) != 0 {
		t.Errorf("after close: %+v, want an empty list", published)
	}
}

// TestDiagnosticFields: range in UTF-16 with the real end, rule, link,
// hint in the message.
func TestDiagnosticFields(t *testing.T) {
	client := startServer(t, Options{})
	client.initialize()
	text := "resources:\n- {name: repo, type: git, source: {uri: \"https://example.com/😀.git\"}}\n" +
		"jobs:\n- name: build\n  plan:\n  - {params: {x: 😀}, get: rpo}\n"
	client.open("untitled:fields", "yaml", text)
	diagnostics := client.sync()["untitled:fields"].Diagnostics
	// CL006 (repo is never used) and CL002.
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics = %+v, want CL006 and CL002", diagnostics)
	}
	got := diagnostics[1]
	// "rpo" is character 27 to 29 (from 1); 😀 before it counts twice in UTF-16.
	wantRange := Range{Start: Position{Line: 5, Character: 27}, End: Position{Line: 5, Character: 30}}
	if got.Code != "CL002" || got.Severity != severityError || got.Range != wantRange || got.Source != "concourse-lint" {
		t.Errorf("diagnostic = %+v, want CL002 error at %+v", got, wantRange)
	}
	if got.CodeDescription == nil || got.CodeDescription.Href != analysis.DocsURL("CL002") {
		t.Errorf("codeDescription = %+v", got.CodeDescription)
	}
	if !strings.HasSuffix(got.Message, `(did you mean "repo"?)`) {
		t.Errorf("message = %q, want the hint at the end", got.Message)
	}
}

// TestDebounce: with a delay, fast changes are checked once, for the last
// text, with its version.
func TestDebounce(t *testing.T) {
	client := startServer(t, Options{Delay: 50 * time.Millisecond})
	client.initialize()
	uri := "untitled:typing"
	base := "resources:\n- {name: repo, type: git, source: {uri: \"https://example.com/r.git\"}}\njobs:\n- name: j\n  plan:\n  - get: "
	client.open(uri, "yaml", base+"r")
	for version, typed := range []string{"re", "rep", "repo"} {
		client.change(uri, version+2, base+typed)
	}
	var published []PublishDiagnosticsParams
	deadline := time.After(2 * time.Second)
	for len(published) == 0 {
		select {
		case message := <-client.messages:
			if message["method"] == "textDocument/publishDiagnostics" {
				published = append(published, decodePublished(t, message))
			}
		case <-deadline:
			t.Fatal("no diagnostics within 2 s")
		}
	}
	// Anything else would arrive within the delay.
	time.Sleep(150 * time.Millisecond)
	for len(client.messages) > 0 {
		if message := <-client.messages; message["method"] == "textDocument/publishDiagnostics" {
			published = append(published, decodePublished(t, message))
		}
	}
	if len(published) != 1 {
		t.Fatalf("published %d times, want once: %+v", len(published), published)
	}
	if published[0].Version == nil || *published[0].Version != 4 || len(published[0].Diagnostics) != 0 {
		t.Errorf("published %+v, want version 4 without findings", published[0])
	}
}

func decodePublished(t *testing.T, message map[string]any) PublishDiagnosticsParams {
	t.Helper()
	params := message["params"].(map[string]any)
	result := PublishDiagnosticsParams{URI: params["uri"].(string)}
	if version, found := params["version"].(float64); found {
		number := int(version)
		result.Version = &number
	}
	diagnostics, _ := params["diagnostics"].([]any)
	for range diagnostics {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{})
	}
	return result
}

// TestConfig: the config next to the file applies; a broken config shows
// in the config file and as one info in the pipeline, and fixing it on disk
// (didChangeWatchedFiles) checks the pipeline again.
func TestConfig(t *testing.T) {
	folder := t.TempDir()
	if err := os.Mkdir(filepath.Join(folder, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(folder, ".concourse-lint.yml")
	writeFile(t, configPath, "rules:\n  CS008: error\n")
	pipelinePath := filepath.Join(folder, "pipeline.yml")
	pipeline := "resources:\n- {name: repo, type: git, source: {uri: \"https://example.com/r.git\"}}\n" +
		"jobs:\n- name: j\n  public: true\n  plan:\n  - get: repo\n"
	writeFile(t, pipelinePath, pipeline)

	client := startServer(t, Options{})
	client.initialize()
	pipelineURI := pathToURI(pipelinePath)
	client.open(pipelineURI, "yaml", pipeline)
	diagnostics := client.sync()[pipelineURI].Diagnostics
	if len(diagnostics) != 1 || diagnostics[0].Code != "CS008" || diagnostics[0].Severity != severityError {
		t.Fatalf("diagnostics = %+v, want CS008 as error (configured)", diagnostics)
	}

	writeFile(t, configPath, "rules:\n  CS008: fatal\n")
	client.notify("workspace/didChangeWatchedFiles", map[string]any{"changes": []any{map[string]any{"uri": pathToURI(configPath), "type": 2}}})
	published := client.sync()
	pipelineDiagnostics := published[pipelineURI].Diagnostics
	if len(pipelineDiagnostics) != 1 || pipelineDiagnostics[0].Severity != severityInformation || !strings.Contains(pipelineDiagnostics[0].Message, "config file is broken") {
		t.Errorf("pipeline with a broken config: %+v, want one info", pipelineDiagnostics)
	}
	configDiagnostics := published[pathToURI(configPath)].Diagnostics
	wantRange := Range{Start: Position{Line: 1, Character: 9}, End: Position{Line: 1, Character: 14}}
	if len(configDiagnostics) != 1 || configDiagnostics[0].Range != wantRange || configDiagnostics[0].Severity != severityError {
		t.Errorf("config file: %+v, want one error at %+v", configDiagnostics, wantRange)
	}

	writeFile(t, configPath, "rules:\n  CS008: off\n")
	client.notify("workspace/didChangeWatchedFiles", map[string]any{"changes": []any{map[string]any{"uri": pathToURI(configPath), "type": 2}}})
	published = client.sync()
	if diagnostics := published[pipelineURI].Diagnostics; len(diagnostics) != 0 {
		t.Errorf("CS008 off: %+v, want none", diagnostics)
	}
	if cleared, found := published[pathToURI(configPath)]; !found || len(cleared.Diagnostics) != 0 {
		t.Errorf("fixed config: %+v, want its error cleared", cleared)
	}
}

// TestConfigFileInEditor: an open config file is checked with its text in
// the editor, also when it is not saved.
func TestConfigFileInEditor(t *testing.T) {
	client := startServer(t, Options{})
	client.initialize()
	uri := pathToURI(filepath.Join(t.TempDir(), ".concourse-lint.yml"))
	client.open(uri, "yaml", "rules:\n  CS060: off\n")
	diagnostics := client.sync()[uri].Diagnostics
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, `unknown rule "CS060"`) || diagnostics[0].Range.Start != (Position{Line: 1, Character: 2}) {
		t.Errorf("diagnostics = %+v, want the unknown rule at 2:3", diagnostics)
	}
	client.change(uri, 2, "rules:\n  CS006: off\n")
	if diagnostics := client.sync()[uri].Diagnostics; len(diagnostics) != 0 {
		t.Errorf("after the fix: %+v, want none", diagnostics)
	}
}

// TestExcluded: a file the config excludes shows nothing.
func TestExcluded(t *testing.T) {
	folder := t.TempDir()
	if err := os.Mkdir(filepath.Join(folder, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(folder, ".concourse-lint.yml"), "exclude: [\"generated/**\"]\n")
	path := filepath.Join(folder, "generated", "pipeline.yml")
	client := startServer(t, Options{})
	client.initialize()
	client.open(pathToURI(path), "yaml", "jobs:\n- name: j\n  plan:\n  - get: missing\nresources: []\n")
	if diagnostics := client.sync()[pathToURI(path)].Diagnostics; len(diagnostics) != 0 {
		t.Errorf("excluded file: %+v, want none", diagnostics)
	}
}

// TestMalformedMessage: a body that is no JSON is answered with a parse
// error, and the server goes on.
func TestMalformedMessage(t *testing.T) {
	client := startServer(t, Options{})
	if _, err := client.toServer.Write([]byte("Content-Length: 5\r\n\r\n{nope")); err != nil {
		t.Fatal(err)
	}
	if response := client.next(); errorCode(response) != codeParseError {
		t.Errorf("response = %v, want a parse error", response)
	}
	client.initialize()
	if err := client.shutdown(); err != nil {
		t.Errorf("Serve = %v", err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
