package security

import (
	"strings"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// CS005 reports an image in image_resource or resource_types (type
// registry-image or docker-image) without a tag or digest, or with latest.
// Cases: spec/rules/security/CS005/
var CS005 = analysis.Rule{
	ID:       "CS005",
	Severity: analysis.SeverityWarning,
	Title:    "Image without a fixed tag or digest",
	Run:      runCS005,
}

func runCS005(pass *analysis.Pass) {
	for _, resourceType := range pass.Pipeline.ResourceTypes {
		if resourceType != nil {
			checkImage(pass, resourceType.Type.Value, resourceType.Source, nil)
		}
	}
	for _, imageResource := range imageResources(pass.Pipeline) {
		checkImage(pass, imageResource.Type.Value, imageResource.Source, imageResource.Version)
	}
}

func checkImage(pass *analysis.Pass, typeName string, source, version *model.Node) {
	if typeName != "registry-image" && typeName != "docker-image" {
		return
	}
	repository := field(source, "repository")
	if repository == nil || repository.Kind != model.ScalarNode || strings.Contains(repository.Text, "@sha256:") {
		return
	}
	if digest := field(version, "digest"); digest != nil {
		return
	}
	tag := field(source, "tag")
	if tag != nil && tag.Kind == model.ScalarNode && tag.Text != "" {
		if tag.Text == "latest" {
			pass.Report(tag.Position, "image tag latest is not pinned", "use a fixed tag or a digest")
		}
		return
	}
	pass.Report(repository.Position, "image "+repository.Text+" has no tag, so it is latest", "add a fixed tag or a digest")
}
