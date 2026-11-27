package security

import (
	"reflect"
	"regexp"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// CS002 reports a value or comment that contains a token in a known format
// (AWS, GitHub, GitLab, Slack, Stripe live, PEM/OpenSSH private key). It
// replaces CS001 at the same place. Cases: spec/rules/security/CS002/
var CS002 = analysis.Rule{
	ID:       "CS002",
	Severity: analysis.SeverityError,
	Title:    "Known token format found",
	Run:      runCS002,
}

// tokenFormat is a token format with a unique prefix.
type tokenFormat struct {
	name    string
	pattern *regexp.Regexp
}

var tokenFormats = []tokenFormat{
	{"AWS access key ID", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"GitHub personal access token", regexp.MustCompile(`\bghp_[A-Za-z0-9]{36}\b`)},
	{"GitLab personal access token", regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20}`)},
	{"Slack webhook URL", regexp.MustCompile(`https://hooks\.slack\.com/services/T[A-Za-z0-9]+/B[A-Za-z0-9]+/[A-Za-z0-9]+`)},
	{"Slack bot token", regexp.MustCompile(`\bxoxb-[0-9]+-[0-9]+-[A-Za-z0-9]+`)},
	{"Stripe live secret key", regexp.MustCompile(`\bsk_live_[A-Za-z0-9]{24,}`)},
	{"private key", privateKeyHeader},
}

var (
	// privateKeyHeader matches the first line of a PEM or OpenSSH private key.
	privateKeyHeader = regexp.MustCompile(`-----BEGIN ([A-Z0-9]+ )*PRIVATE KEY-----`)
	// privateKeyBody matches a private key up to its END line; the group is
	// the text between the two lines.
	privateKeyBody = regexp.MustCompile(`-----BEGIN (?:[A-Z0-9]+ )*PRIVATE KEY-----([\s\S]*?)-----END `)
	// base64Character matches one character of the key's base64 text.
	base64Character = regexp.MustCompile(`[A-Za-z0-9+/=]`)
	// keyVariable matches a ((var)) in the key's text; it does not count.
	keyVariable = regexp.MustCompile(`\(\([^()]*\)\)`)
)

// minimumKeyCharacters is the least base64 text between BEGIN and END that
// counts as a key. Real keys have hundreds of characters; a frame around a
// placeholder (###) or a ((var)) has fewer.
const minimumKeyCharacters = 20

// knownToken returns the name of the token format the text contains.
func knownToken(text string) (formatName string, found bool) {
	for _, format := range tokenFormats {
		if format.pattern.MatchString(text) && !isPrivateKeyFrame(format.pattern, text) {
			return format.name, true
		}
	}
	return "", false
}

// isPrivateKeyFrame reports whether the text holds only the frame of a
// private key: BEGIN and END lines with too little between them. A header
// without END (a comment line, a single value) is no frame.
func isPrivateKeyFrame(pattern *regexp.Regexp, text string) bool {
	if pattern != privateKeyHeader {
		return false
	}
	match := privateKeyBody.FindStringSubmatch(text)
	if match == nil {
		return false
	}
	keyText := keyVariable.ReplaceAllString(match[1], "")
	return len(base64Character.FindAllString(keyText, -1)) < minimumKeyCharacters
}

func runCS002(pass *analysis.Pass) {
	report := func(position model.Position, text string) {
		if formatName, found := knownToken(text); found {
			pass.Report(position, formatName+" in plain text", "remove it, revoke it, and use a ((var)) from a credential manager")
		}
	}
	forEachText(reflect.ValueOf(pass.Pipeline), report)
}

var (
	stringValueType = reflect.TypeOf(model.Value[string]{})
	nodePointerType = reflect.TypeOf((*model.Node)(nil))
	commentType     = reflect.TypeOf(model.Comment{})
)

// forEachText calls visit for every string value, every scalar of a
// free-form block and every comment in the model.
func forEachText(value reflect.Value, visit func(position model.Position, text string)) {
	switch value.Type() {
	case stringValueType:
		if text := value.Interface().(model.Value[string]); text.Set {
			visit(text.Position, text.Value)
		}
		return
	case nodePointerType:
		forEachScalar(value.Interface().(*model.Node), func(scalar *model.Node) {
			visit(scalar.Position, scalar.Text)
		})
		return
	case commentType:
		comment := value.Interface().(model.Comment)
		visit(comment.Position, comment.Text)
		return
	}
	switch value.Kind() {
	case reflect.Pointer:
		if !value.IsNil() {
			forEachText(value.Elem(), visit)
		}
	case reflect.Struct:
		for index := range value.NumField() {
			if value.Type().Field(index).IsExported() {
				forEachText(value.Field(index), visit)
			}
		}
	case reflect.Slice:
		for index := range value.Len() {
			forEachText(value.Index(index), visit)
		}
	}
}
