package security

import (
	"strings"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// CS003 reports disabled TLS verification in a source (resources, resource
// types, image_resource) or in get_params of a put. Cases:
// spec/rules/security/CS003/
var CS003 = analysis.Rule{
	ID:       "CS003",
	Severity: analysis.SeverityWarning,
	Title:    "TLS verification disabled",
	Run:      runCS003,
}

// tlsOffKeys switch TLS verification off when they are true.
var tlsOffKeys = []string{"skip_ssl_verification", "insecure", "disable_ssl"}

func runCS003(pass *analysis.Pass) {
	for _, sourceBlock := range allSources(pass.Pipeline) {
		checkTLS(pass, sourceBlock.node)
	}
	forEachStep(pass.Pipeline, func(step *model.Step) {
		if step.Put != nil {
			checkTLS(pass, step.Put.GetParams)
		}
	})
}

func checkTLS(pass *analysis.Pass, node *model.Node) {
	if node == nil || node.Kind != model.MappingNode {
		return
	}
	for _, pair := range node.Pairs {
		for _, key := range tlsOffKeys {
			if pair.Key == key && isTruthy(pair.Value) {
				pass.Report(pair.Value.Position, key+" switches TLS verification off", "use a trusted certificate instead (e.g. ca_cert, ca_certs)")
			}
		}
		if pair.Key == "insecure_registries" && pair.Value != nil && pair.Value.Kind == model.ListNode && len(pair.Value.Items) > 0 {
			pass.Report(pair.Value.Position, "insecure_registries switches TLS verification off for these registries", "use a trusted certificate instead")
		}
	}
}

// isTruthy reports whether the value switches a flag on: a YAML 1.1 boolean
// true, or the quoted text "true" (spec/rules/security/CS003/image-resource.yml).
func isTruthy(node *model.Node) bool {
	if node == nil || node.Kind != model.ScalarNode {
		return false
	}
	if node.Quoted {
		return strings.EqualFold(node.Text, "true")
	}
	value, isBoolean := node.Boolean()
	return isBoolean && value
}
