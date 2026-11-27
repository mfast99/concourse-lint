// Package navigate answers the questions of the editor features: what is
// at a position of a pipeline, where it is defined and where it is used. It
// also holds the resolving of references that
// the rules use, so that the linter and the editor always agree on what
// refers to what.
//
// It works on the model only; the language server gets the model through
// lint and translates the answers to LSP.
package navigate

import (
	"cmp"
	"slices"

	"github.com/mfast99/concourse-lint/internal/model"
)

// Kind is the kind of a name that can be defined and referred to.
type Kind string

const (
	KindResource     Kind = "resource"
	KindJob          Kind = "job"
	KindResourceType Kind = "resource_type"
)

// Reference is a definition of a name or a use of it.
type Reference struct {
	Kind Kind
	Name string
	// The position of the name (of the whole glob for a glob).
	Position     model.Position
	IsDefinition bool
	// IsGlob marks a group entry that is a glob matching the job: no
	// literal mention of the name, so a rename must not rewrite it.
	IsGlob bool
}

// Index returns every definition and use of a resource, job or resource
// type in the order of the file. Names with ((var)) are left out, because
// what they refer to is unknown.
func Index(pipeline *model.Pipeline) []Reference {
	builder := &indexBuilder{seen: map[Reference]bool{}}
	for _, resourceType := range pipeline.ResourceTypes {
		if resourceType == nil {
			continue
		}
		builder.define(KindResourceType, resourceType.Name)
		builder.use(KindResourceType, resourceType.Type)
	}
	for _, resource := range pipeline.Resources {
		if resource == nil {
			continue
		}
		builder.define(KindResource, resource.Name)
		builder.use(KindResourceType, resource.Type)
	}
	for _, job := range pipeline.Jobs {
		if job == nil {
			continue
		}
		builder.define(KindJob, job.Name)
		for step := range job.AllSteps() {
			builder.stepUses(step)
		}
	}
	for _, group := range pipeline.Groups {
		if group == nil {
			continue
		}
		for _, resource := range group.Resources {
			builder.use(KindResource, resource)
		}
	}
	builder.groupJobUses(pipeline)
	slices.SortStableFunc(builder.references, func(first, second Reference) int {
		return comparePositions(first.Position, second.Position)
	})
	return builder.references
}

// Definition returns where the name at the cursor is defined: one position,
// several for a duplicate name or a glob, none for an unknown name, a
// built-in resource type or a place without a name.
func Definition(pipeline *model.Pipeline, cursor model.Position) []model.Position {
	index := Index(pipeline)
	symbols := symbolsAt(index, cursor)
	var positions []model.Position
	for _, reference := range index {
		if reference.IsDefinition && symbols[symbolOf(reference)] {
			positions = appendOnce(positions, reference.Position)
		}
	}
	return positions
}

// References returns where the name at the cursor is used, in the order of
// the file, and with includeDeclaration also where it is defined.
func References(pipeline *model.Pipeline, cursor model.Position, includeDeclaration bool) []model.Position {
	index := Index(pipeline)
	symbols := symbolsAt(index, cursor)
	var positions []model.Position
	for _, reference := range index {
		if symbols[symbolOf(reference)] && (includeDeclaration || !reference.IsDefinition) {
			positions = appendOnce(positions, reference.Position)
		}
	}
	return positions
}

// appendOnce adds a position that is not in the list yet: a glob refers to
// every job it matches, but it is one place in the file.
func appendOnce(positions []model.Position, position model.Position) []model.Position {
	if slices.Contains(positions, position) {
		return positions
	}
	return append(positions, position)
}

// symbol is a name of one kind.
type symbol struct {
	kind Kind
	name string
}

func symbolOf(reference Reference) symbol {
	return symbol{kind: reference.Kind, name: reference.Name}
}

// symbolsAt returns the names at the cursor: usually one, several for a
// glob that matches several jobs.
func symbolsAt(index []Reference, cursor model.Position) map[symbol]bool {
	symbols := map[symbol]bool{}
	for _, reference := range index {
		if contains(reference.Position, cursor) {
			symbols[symbolOf(reference)] = true
		}
	}
	return symbols
}

// contains reports whether the cursor is on the name: from its first
// character to right after its last one, where the cursor stands after
// typing it.
func contains(position model.Position, cursor model.Position) bool {
	if cursor.Line != position.Line || cursor.Column < position.Column {
		return false
	}
	if position.EndLine != position.Line {
		return cursor.Column == position.Column
	}
	return cursor.Column <= position.EndColumn
}

func comparePositions(first, second model.Position) int {
	return cmp.Or(cmp.Compare(first.Line, second.Line), cmp.Compare(first.Column, second.Column))
}

// indexBuilder collects references. An alias decodes an anchored value
// again, with the positions of the anchor, so the same reference can come
// twice; seen drops the second one.
type indexBuilder struct {
	references []Reference
	seen       map[Reference]bool
}

func (builder *indexBuilder) add(reference Reference) {
	if builder.seen[reference] {
		return
	}
	builder.seen[reference] = true
	builder.references = append(builder.references, reference)
}

func (builder *indexBuilder) define(kind Kind, name model.Value[string]) {
	if Resolvable(name) {
		builder.add(Reference{Kind: kind, Name: name.Value, Position: name.Position, IsDefinition: true})
	}
}

func (builder *indexBuilder) use(kind Kind, name model.Value[string]) {
	if Resolvable(name) {
		builder.add(Reference{Kind: kind, Name: name.Value, Position: name.Position})
	}
}

// stepUses adds what a step refers to: the resource of a get or put, the
// jobs in passed: and the resource type of an image_resource.
func (builder *indexBuilder) stepUses(step *model.Step) {
	if resource, isResourceStep := StepResource(step); isResourceStep {
		builder.use(KindResource, resource)
	}
	if step.Get != nil {
		for _, job := range step.Get.Passed {
			builder.use(KindJob, job)
		}
	}
	if step.Task != nil && step.Task.Config != nil && step.Task.Config.ImageResource != nil {
		builder.use(KindResourceType, step.Task.Config.ImageResource.Type)
	}
}

// groupJobUses adds the jobs of the groups: an exact name refers to that
// job, a glob to every job it matches.
func (builder *indexBuilder) groupJobUses(pipeline *model.Pipeline) {
	jobs := JobNames(pipeline)
	for _, pattern := range GroupPatterns(pipeline) {
		if !IsGlob(pattern.Entry.Value) {
			builder.use(KindJob, pattern.Entry)
			continue
		}
		for _, job := range jobs {
			if pattern.Matches(job) {
				builder.add(Reference{Kind: KindJob, Name: job, Position: pattern.Entry.Position, IsGlob: true})
			}
		}
	}
}
