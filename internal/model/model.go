// Package model is our own representation of a Concourse pipeline. Every value
// a rule can point to carries its position in the file.
//
// The model covers every field Concourse knows (atc structs of Concourse
// 8.3.4). Fields whose value can take several shapes
// (e.g. version: latest, every or a map) are kept as a free-form Node.
package model

// Position is a position in the source file, 1-based like in editors, with
// columns counted in characters (Unicode code points). It also says where
// the text it stands for ends.
type Position struct {
	Line   int `json:"line"`
	Column int `json:"col"`
	// EndLine and EndColumn are right after the last character of the text
	// (the key, value, comment or token). Zero: the position has no length,
	// e.g. "no jobs" at the start of the file.
	EndLine   int `json:"end_line,omitempty"`
	EndColumn int `json:"end_col,omitempty"`
}

// Start returns the position without its end.
func (position Position) Start() Position {
	return Position{Line: position.Line, Column: position.Column}
}

// Value is a scalar from the file together with its position.
type Value[T any] struct {
	Value T
	// The value, e.g. "repo" in "name: repo".
	Position Position
	// The key in front of it, e.g. "name". Zero for list items.
	KeyPosition Position
	// False: missing, or wrong type (then already reported as CL001).
	Set bool
}

// Pipeline is the parsed pipeline (atc.Config).
type Pipeline struct {
	Groups     []*Group
	VarSources []*VarSource
	Resources  []*Resource
	// ResourceTypes are the custom resource types of this pipeline.
	ResourceTypes []*ResourceType
	// Prototypes have the same fields as resource types (atc.Prototype) and
	// are used by run steps.
	Prototypes []*ResourceType
	Jobs       []*Job
	Display    *Display
	UserData   *Node
	// Comments are all comments of the file, in file order (for suppression
	// comments and CS002).
	Comments []Comment
}

// Comment is a comment in the file.
type Comment struct {
	// Position of the "#".
	Position Position
	// The text after the "#".
	Text string
}

// Group is an entry under groups:.
type Group struct {
	// First line of the element.
	Position  Position
	Name      Value[string]
	Jobs      []Value[string]
	Resources []Value[string]
}

// VarSource is an entry under var_sources:.
type VarSource struct {
	// First line of the element.
	Position Position
	Name     Value[string]
	Type     Value[string]
	Config   *Node
}

// Resource is an entry under resources:.
type Resource struct {
	// First line of the element.
	Position             Position
	Name                 Value[string]
	OldName              Value[string]
	Public               Value[bool]
	WebhookToken         Value[string]
	Type                 Value[string]
	Source               *Node
	CheckEvery           Value[string]
	CheckTimeout         Value[string]
	Tags                 []Value[string]
	Version              *Node
	Icon                 Value[string]
	ExposeBuildCreatedBy Value[bool]
}

// ResourceType is an entry under resource_types: (or prototypes:).
type ResourceType struct {
	// First line of the element.
	Position   Position
	Name       Value[string]
	Type       Value[string]
	Source     *Node
	Defaults   *Node
	Privileged Value[bool]
	CheckEvery Value[string]
	Tags       []Value[string]
	Params     *Node
}

// Display is the display: block of the pipeline.
type Display struct {
	// First line of the block.
	Position         Position
	BackgroundImage  Value[string]
	BackgroundFilter Value[string]
}

// Job is an entry under jobs:.
type Job struct {
	// First line of the element.
	Position             Position
	Name                 Value[string]
	OldName              Value[string]
	Public               Value[bool]
	DisableManualTrigger Value[bool]
	DisableReruns        Value[bool]
	Serial               Value[bool]
	Interruptible        Value[bool]
	SerialGroups         []Value[string]
	MaxInFlight          Value[int]
	Tags                 []Value[string]
	BuildLogsToRetain    Value[int]
	BuildLogRetention    *BuildLogRetention
	Plan                 []*Step
	// PlanUnknown: plan: has a wrong type (CL001) or is a ((var)), so its
	// steps are unknown. A missing or empty plan is not unknown.
	PlanUnknown bool
	Hooks
}

// BuildLogRetention is the build_log_retention: block of a job.
type BuildLogRetention struct {
	// First line of the block.
	Position               Position
	Builds                 Value[int]
	MinimumSucceededBuilds Value[int]
	Days                   Value[int]
}

// Hooks are the steps that run after a job or a step, depending on how it
// ended. Job and Step embed them, so job.OnFailure and step.OnFailure both
// work.
type Hooks struct {
	OnSuccess *Step
	OnFailure *Step
	OnAbort   *Step
	OnError   *Step
	Ensure    *Step
}
