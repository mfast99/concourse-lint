package security

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// CS001 reports a secret-like key with a fixed value in source, task params,
// set_pipeline vars and var_sources config. Cases: spec/rules/security/CS001/
var CS001 = analysis.Rule{
	ID:       "CS001",
	Severity: analysis.SeverityWarning,
	Title:    "Plaintext secret",
	Run:      runCS001,
}

func runCS001(pass *analysis.Pass) {
	pipeline := pass.Pipeline
	for _, sourceBlock := range allSources(pipeline) {
		checkSecrets(pass, sourceBlock.node)
	}
	for _, varSource := range pipeline.VarSources {
		if varSource != nil {
			checkSecrets(pass, varSource.Config)
		}
	}
	forEachStep(pipeline, func(step *model.Step) {
		switch {
		case step.Task != nil:
			checkSecrets(pass, step.Task.Params)
			if step.Task.Config != nil {
				checkSecrets(pass, step.Task.Config.Params)
			}
		case step.SetPipeline != nil:
			checkSecrets(pass, step.SetPipeline.Vars)
		}
	})
}

// checkSecrets looks for secret-like keys at every level of the block.
func checkSecrets(pass *analysis.Pass, node *model.Node) {
	if node == nil {
		return
	}
	switch node.Kind {
	case model.MappingNode:
		for _, pair := range node.Pairs {
			if pair.Value != nil && pair.Value.Kind == model.ScalarNode {
				if isSecretKey(pair.Key, pass.Settings.ExtraSecretWords) && isFixedValue(pair.Value) {
					pass.Report(pair.KeyPosition,
						fmt.Sprintf("%s has a fixed value in plain text", pair.Key),
						"use a ((var)) from a credential manager")
				}
				continue
			}
			checkSecrets(pass, pair.Value)
		}
	case model.ListNode:
		for _, item := range node.Items {
			checkSecrets(pass, item)
		}
	}
}

// onlyVariable matches a value that is a single ((var)) or ((.:var)).
var onlyVariable = regexp.MustCompile(`^\(\([^()]*\)\)$`)

// publicValues are fixed words that are no secret although they stand in a
// secret-like field.
var publicValues = []string{
	// GitHub token auth over HTTPS: the token is the username, the password
	// is this word.
	"x-oauth-basic",
}

// placeholder matches a value that stands in for a secret: REDACTED, text in
// angle brackets, YOUR-… and REPLACE-… (any case, also with _).
var placeholder = regexp.MustCompile(`(?i)^(redacted|<[^<>]+>|your[-_].+|replace[-_].+)$`)

// isFixedValue reports whether the scalar is a value written into the file:
// not empty, no boolean or number, not only a ((var)), no public word, no
// placeholder and no known token (that is CS002).
func isFixedValue(scalar *model.Node) bool {
	text := strings.TrimSpace(scalar.Text)
	if text == "" || onlyVariable.MatchString(text) {
		return false
	}
	if slices.Contains(publicValues, strings.ToLower(text)) || placeholder.MatchString(text) {
		return false
	}
	if !scalar.Quoted {
		switch model.ResolveYAML11(text).Kind {
		case model.BoolScalar, model.IntScalar, model.FloatScalar, model.NullScalar:
			return false
		}
	}
	_, isToken := knownToken(scalar.Text)
	return !isToken
}

// isSecretKey reports whether the last word of the key (split at _, - and
// case changes) is password, passwd, secret, token or one of the extra words
// from the config, or the last two are private, api, access or secret
// followed by key.
func isSecretKey(key string, extraWords []string) bool {
	words := splitWords(key)
	if len(words) == 0 {
		return false
	}
	last := words[len(words)-1]
	if slices.Contains(extraWords, last) {
		return true
	}
	switch last {
	case "password", "passwd", "secret", "token":
		return true
	case "key":
		if len(words) < 2 {
			return false
		}
		switch words[len(words)-2] {
		case "private", "api", "access", "secret":
			return true
		}
	}
	return false
}

// splitWords splits a key into lowercase words at "_", "-" and case changes:
// "dbPassword" -> db, password; "APIKey" -> api, key.
func splitWords(key string) []string {
	var words []string
	var current []rune
	runes := []rune(key)
	flush := func() {
		if len(current) > 0 {
			words = append(words, strings.ToLower(string(current)))
			current = nil
		}
	}
	for index, character := range runes {
		if character == '_' || character == '-' || character == '.' || unicode.IsSpace(character) {
			flush()
			continue
		}
		if unicode.IsUpper(character) && len(current) > 0 {
			previous := runes[index-1]
			nextIsLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || (unicode.IsUpper(previous) && nextIsLower) {
				flush()
			}
		}
		current = append(current, character)
	}
	flush()
	return words
}
