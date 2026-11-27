// Package config reads the config file .concourse-lint.yml (spec/README.md,
// "Config file"): rules switched off or with another severity, rule options,
// extra resource types and excluded paths.
//
// A nil *Config is the default configuration, so callers never need a check.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gobwas/glob"
	"github.com/mfast99/go-yaml"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// FileNames are the names of the config file, in the order they are looked
// for.
var FileNames = []string{".concourse-lint.yml", ".concourse-lint.yaml"}

// unswitchable rules cannot be switched off.
var unswitchable = []string{"CL000"}

// Config is a loaded config file.
type Config struct {
	// Path of the config file.
	Path string
	// Settings are the rule options.
	Settings analysis.Settings
	// rules maps a rule ID to its setting.
	rules map[string]ruleSetting
	// exclude are the compiled exclude patterns.
	exclude []glob.Glob
}

type ruleSetting struct {
	off bool
	// severity is "" when the rule keeps its own.
	severity analysis.Severity
}

// IsOff reports whether the rule is switched off.
func (config *Config) IsOff(ruleID string) bool {
	return config != nil && config.rules[ruleID].off
}

// Severity returns the configured severity of the rule, or the default.
func (config *Config) Severity(ruleID string, defaultSeverity analysis.Severity) analysis.Severity {
	if config == nil || config.rules[ruleID].severity == "" {
		return defaultSeverity
	}
	return config.rules[ruleID].severity
}

// RuleSettings returns the rule options (the defaults for a nil config).
func (config *Config) RuleSettings() analysis.Settings {
	if config == nil {
		return analysis.Settings{}
	}
	return config.Settings
}

// Excluded reports whether the file is excluded: a pattern matches its path
// relative to the folder of the config file, or one of the folders above it
// (up to that folder).
func (config *Config) Excluded(path string) bool {
	if config == nil || len(config.exclude) == 0 {
		return false
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	relativePath, err := filepath.Rel(filepath.Dir(config.Path), absolutePath)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return false
	}
	relativePath = filepath.ToSlash(relativePath)
	for candidate := relativePath; candidate != "." && candidate != ""; candidate = parentPath(candidate) {
		for _, pattern := range config.exclude {
			if safeMatch(pattern, candidate) {
				return true
			}
		}
	}
	return false
}

// safeMatch matches like pattern.Match, but a pattern on which the glob
// library panics (an unclosed or empty {} alternative at the end, e.g.
// "ci{" on "ci") matches nothing instead of crashing the CLI or the language
// server.
func safeMatch(pattern glob.Glob, candidate string) (matched bool) {
	defer func() {
		if recover() != nil {
			matched = false
		}
	}()
	return pattern.Match(candidate)
}

func parentPath(slashPath string) string {
	index := strings.LastIndex(slashPath, "/")
	if index < 0 {
		return ""
	}
	return slashPath[:index]
}

// Find looks for the config file of a file in folder startDir: in that
// folder and the ones above, up to and including the Git root (the folder
// with .git) or the file system root. It returns "" when there is none.
func Find(startDir string) (string, error) {
	folder, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		var found []string
		for _, name := range FileNames {
			candidate := filepath.Join(folder, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				found = append(found, candidate)
			}
		}
		if len(found) > 1 {
			return "", fmt.Errorf("%s: both %s and %s exist, keep one", folder, FileNames[0], FileNames[1])
		}
		if len(found) == 1 {
			return found[0], nil
		}
		if _, err := os.Stat(filepath.Join(folder, ".git")); err == nil {
			return "", nil
		}
		parent := filepath.Dir(folder)
		if parent == folder {
			return "", nil
		}
		folder = parent
	}
}

// Load reads and checks a config file. knownRuleIDs are all rule IDs of
// the linter.
func Load(path string, knownRuleIDs []string) (*Config, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	config, err := Parse(source, knownRuleIDs)
	var configError *Error
	if errors.As(err, &configError) {
		configError.Path = path
	}
	if err != nil {
		return nil, err
	}
	config.Path = absolutePath
	return config, nil
}

// file is the shape of the config file.
type file struct {
	Rules         map[string]any `yaml:"rules"`
	ResourceTypes []string       `yaml:"resource_types"`
	Exclude       []string       `yaml:"exclude"`
}

// Parse reads the text of a config file. Errors are of type *Error, with
// the position of the offending key or value.
func Parse(source []byte, knownRuleIDs []string) (*Config, error) {
	var content file
	if err := yaml.UnmarshalWithOptions(source, &content, yaml.DisallowUnknownField()); err != nil {
		return nil, yamlError(err)
	}
	config, err := build(content, knownRuleIDs)
	var located *problem
	if errors.As(err, &located) {
		return nil, locate(source, located)
	}
	return config, err
}

// build checks the content of a config file and turns it into a Config.
func build(content file, knownRuleIDs []string) (*Config, error) {
	config := &Config{rules: map[string]ruleSetting{}}
	config.Settings.ExtraResourceTypes = content.ResourceTypes
	for _, ruleID := range sortedKeys(content.Rules) {
		if !slices.Contains(knownRuleIDs, ruleID) {
			message := fmt.Sprintf("unknown rule %q in rules:", ruleID)
			if hint := analysis.DidYouMean(ruleID, knownRuleIDs); hint != "" {
				message += " (" + hint + ")"
			}
			return nil, problemAtKey([]any{"rules", ruleID}, "%s", message)
		}
		setting, err := config.ruleSetting(ruleID, content.Rules[ruleID])
		if err != nil {
			return nil, err
		}
		if setting.off && slices.Contains(unswitchable, ruleID) {
			return nil, problemAt(ruleValuePath(ruleID, content.Rules[ruleID]), "rules: %s cannot be switched off", ruleID)
		}
		config.rules[ruleID] = setting
	}
	for index, pattern := range content.Exclude {
		compiled, err := glob.Compile(strings.TrimSuffix(filepath.ToSlash(pattern), "/"), '/')
		if err != nil {
			return nil, problemAt([]any{"exclude", index}, "exclude: %q is no valid glob", pattern)
		}
		config.exclude = append(config.exclude, compiled)
	}
	return config, nil
}

// ruleOptions are the options each rule accepts, besides severity.
var ruleOptions = map[string][]string{
	"CS001": {"extra_words"},
	"CS004": {"allowed_hosts"},
	"CB002": {"min_interval"},
}

// ruleValuePath is the path to the severity of a rule entry: the value
// itself, or its severity key in a mapping.
func ruleValuePath(ruleID string, value any) []any {
	if _, isMapping := value.(map[string]any); isMapping {
		return []any{"rules", ruleID, "severity"}
	}
	return []any{"rules", ruleID}
}

// ruleSetting reads the value of one entry under rules: a severity, off, or
// a mapping with severity and options.
func (config *Config) ruleSetting(ruleID string, value any) (ruleSetting, error) {
	path := []any{"rules", ruleID}
	prefix := "rules: " + ruleID + ": "
	switch typed := value.(type) {
	case string:
		setting, err := parseSeverity(typed)
		if err != nil {
			return ruleSetting{}, problemAt(path, "%s%v", prefix, err)
		}
		return setting, nil
	case bool:
		if !typed {
			return ruleSetting{off: true}, nil
		}
		return ruleSetting{}, problemAt(path, "%suse a severity (error, warning, info) or off", prefix)
	case map[string]any:
		setting := ruleSetting{}
		for _, key := range sortedKeys(typed) {
			option := typed[key]
			optionPath := []any{"rules", ruleID, key}
			if key == "severity" {
				text, isText := option.(string)
				if !isText {
					return ruleSetting{}, problemAt(optionPath, "%sseverity must be error, warning, info or off", prefix)
				}
				parsed, err := parseSeverity(text)
				if err != nil {
					return ruleSetting{}, problemAt(optionPath, "%s%v", prefix, err)
				}
				setting = parsed
				continue
			}
			if !slices.Contains(ruleOptions[ruleID], key) {
				known := append([]string{"severity"}, ruleOptions[ruleID]...)
				return ruleSetting{}, problemAtKey(optionPath, "%sunknown option %q (known: %s)", prefix, key, strings.Join(known, ", "))
			}
			if err := config.applyOption(key, option); err != nil {
				return ruleSetting{}, problemAt(optionPath, "%s%s: %v", prefix, key, err)
			}
		}
		return setting, nil
	case nil:
		return ruleSetting{}, nil
	}
	return ruleSetting{}, problemAt(path, "%suse a severity (error, warning, info), off, or a mapping with severity and options", prefix)
}

func (config *Config) applyOption(key string, value any) error {
	switch key {
	case "extra_words":
		words, err := stringList(value)
		config.Settings.ExtraSecretWords = lowercase(words)
		return err
	case "allowed_hosts":
		hosts, err := stringList(value)
		config.Settings.AllowedHosts = lowercase(hosts)
		return err
	case "min_interval":
		text, isText := value.(string)
		interval, err := time.ParseDuration(text)
		if !isText || err != nil {
			return fmt.Errorf("must be a Go duration like 30s or 2m, not %v", value)
		}
		config.Settings.MinCheckInterval = interval
	}
	return nil
}

func parseSeverity(text string) (ruleSetting, error) {
	switch text {
	case "off":
		return ruleSetting{off: true}, nil
	case string(analysis.SeverityError), string(analysis.SeverityWarning), string(analysis.SeverityInfo):
		return ruleSetting{severity: analysis.Severity(text)}, nil
	}
	return ruleSetting{}, fmt.Errorf("unknown severity %q (use error, warning, info or off)", text)
}

func stringList(value any) ([]string, error) {
	items, isList := value.([]any)
	if !isList {
		return nil, errors.New("must be a list of strings")
	}
	var texts []string
	for _, item := range items {
		text, isText := item.(string)
		if !isText {
			return nil, errors.New("must be a list of strings")
		}
		texts = append(texts, text)
	}
	return texts, nil
}

func lowercase(texts []string) []string {
	result := make([]string, 0, len(texts))
	for _, text := range texts {
		result = append(result, strings.ToLower(text))
	}
	return result
}

// sortedKeys returns the keys of a map sorted, so errors come in a fixed
// order.
func sortedKeys[V any](entries map[string]V) []string {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
