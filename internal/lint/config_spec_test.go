package lint

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestConfigSpec runs the scenarios in spec/config/: every pipeline is
// checked with the config the search finds for it, and the expected
// severities are the configured ones.
func TestConfigSpec(t *testing.T) {
	catalog := spectest.LoadCatalog(t)
	for _, scenario := range spectest.ConfigScenarios(t) {
		t.Run(filepath.Base(scenario), func(t *testing.T) {
			pipelines, configs := spectest.ConfigScenarioFiles(t, scenario)
			for _, configPath := range configs {
				source, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatal(err)
				}
				_, loadErr := LoadConfig(configPath)
				wantError := spectest.Marker(source, "want-config-error")
				if wantError && loadErr == nil {
					t.Errorf("%s: loading must fail", spectest.Name(configPath))
				}
				if !wantError && loadErr != nil {
					t.Errorf("%s: %v", spectest.Name(configPath), loadErr)
				}
			}
			for _, pipelinePath := range pipelines {
				source, err := os.ReadFile(pipelinePath)
				if err != nil {
					t.Fatal(err)
				}
				config, err := ConfigFor(pipelinePath)
				if err != nil {
					t.Fatalf("%s: %v", spectest.Name(pipelinePath), err)
				}
				excluded := config.Excluded(pipelinePath)
				if wantExcluded := spectest.Marker(source, "want-excluded"); excluded != wantExcluded {
					t.Errorf("%s: excluded = %v, want %v", spectest.Name(pipelinePath), excluded, wantExcluded)
				}
				if excluded {
					continue
				}
				configured := maps.Clone(catalog)
				for rule, severity := range catalog {
					configured[rule] = config.Severity(rule, severity)
				}
				findings := CheckWithConfig(pipelinePath, source, config)
				for _, problem := range spectest.Compare(spectest.ParseExpectations(source), configured, findings) {
					t.Errorf("%s: %s", spectest.Name(pipelinePath), problem)
				}
			}
		})
	}
}
