package model

// Step is a step in a plan, a hook or a nested step. For a valid step
// exactly one of the step kind pointers is set; a step with none or with two
// is reported as CL001 and stays in the model.
//
// Concourse treats across, attempts, timeout and the hooks as wrappers around
// the step kind; here they are plain fields next to it.
type Step struct {
	// First line of the step.
	Position Position

	// Step kinds.
	Get         *GetStep
	Put         *PutStep
	Task        *TaskStep
	Run         *RunStep
	SetPipeline *SetPipelineStep
	LoadVar     *LoadVarStep
	InParallel  *InParallelStep
	Do          *DoStep
	Try         *TryStep

	// Modifiers that apply to the step kind.
	Across   *Across
	Attempts Value[int]
	Timeout  Value[string]
	Tags     []Value[string]
	Hooks
}

// GetStep is the get-specific part of a step.
type GetStep struct {
	// Artifact name; also the resource unless resource: is set.
	Name     Value[string]
	Resource Value[string]
	// "latest", "every" or a map of version fields.
	Version *Node
	Params  *Node
	Passed  []Value[string]
	Trigger Value[bool]
}

// PutStep is the put-specific part of a step.
type PutStep struct {
	// Artifact name of the implicit get; also the resource unless resource:
	// is set.
	Name     Value[string]
	Resource Value[string]
	Params   *Node
	// "all", "detect" or a list of artifact names.
	Inputs    *Node
	GetParams *Node
	NoGet     Value[bool]
}

// TaskStep is the task-specific part of a step.
type TaskStep struct {
	Name       Value[string]
	Privileged Value[bool]
	Hermetic   Value[bool]
	// Path of the task config, starting with an artifact name.
	File            Value[string]
	ContainerLimits *Node
	Config          *TaskConfig
	Params          *Node
	Vars            *Node
	InputMapping    []*ArtifactMapping
	OutputMapping   []*ArtifactMapping
	// Artifact to use as the task image.
	Image Value[string]
}

// ArtifactMapping is one entry of input_mapping or output_mapping.
type ArtifactMapping struct {
	// The name inside the task config (the key).
	TaskName Value[string]
	// The name in the plan (the value).
	PlanName Value[string]
}

// RunStep is the run-specific part of a step (runs a prototype).
type RunStep struct {
	// The value of run:, the message sent to the prototype.
	Message         Value[string]
	Type            Value[string]
	Params          *Node
	Privileged      Value[bool]
	ContainerLimits *Node
}

// SetPipelineStep is the set_pipeline-specific part of a step.
type SetPipelineStep struct {
	// Pipeline name, or "self".
	Name         Value[string]
	File         Value[string]
	Team         Value[string]
	Vars         *Node
	VarFiles     []Value[string]
	InstanceVars *Node
}

// LoadVarStep is the load_var-specific part of a step.
type LoadVarStep struct {
	// Name of the local variable, used as ((.:name)).
	Name   Value[string]
	File   Value[string]
	Format Value[string]
	Reveal Value[bool]
}

// InParallelStep runs its steps in parallel. Both forms (a plain list, or a
// block with steps:, limit: and fail_fast:) end up here.
type InParallelStep struct {
	Steps    []*Step
	Limit    Value[int]
	FailFast Value[bool]
}

// DoStep runs its steps one after another.
type DoStep struct {
	Steps []*Step
}

// TryStep runs its step and ignores a failure.
type TryStep struct {
	Step *Step
}

// Across runs the step once for every combination of the variables.
type Across struct {
	Variables []*AcrossVariable
	FailFast  Value[bool]
}

// AcrossVariable is one entry under across:.
type AcrossVariable struct {
	// First line of the element.
	Position Position
	// Name of the local variable (var:), used as ((.:name)).
	Name   Value[string]
	Values *Node
	// A number or "all".
	MaxInFlight *Node
}

// TaskConfig is the inline config: of a task (atc.TaskConfig).
type TaskConfig struct {
	// First line of the block.
	Position        Position
	Platform        Value[string]
	RootfsURI       Value[string]
	ImageResource   *ImageResource
	ContainerLimits *Node
	Params          *Node
	Run             *TaskRun
	Inputs          []*TaskInput
	Outputs         []*TaskOutput
	Caches          []*TaskCache
}

// ImageResource is the image_resource: block of a task config.
type ImageResource struct {
	// First line of the block.
	Position Position
	Name     Value[string]
	Type     Value[string]
	Source   *Node
	Version  *Node
	Params   *Node
	Tags     []Value[string]
}

// TaskRun is the run: block of a task config.
type TaskRun struct {
	// First line of the block.
	Position Position
	Path     Value[string]
	Args     []Value[string]
	Dir      Value[string]
	User     Value[string]
}

// TaskInput is an entry under inputs: of a task config.
type TaskInput struct {
	// First line of the element.
	Position Position
	Name     Value[string]
	Path     Value[string]
	Optional Value[bool]
}

// TaskOutput is an entry under outputs: of a task config.
type TaskOutput struct {
	// First line of the element.
	Position Position
	Name     Value[string]
	Path     Value[string]
}

// TaskCache is an entry under caches: of a task config.
type TaskCache struct {
	// First line of the element.
	Position Position
	Path     Value[string]
	TTL      Value[string]
}

// HasOneKind reports whether exactly one step kind is set. Other steps are
// invalid (CL001) and rules should not draw follow-up findings from them.
func (step *Step) HasOneKind() bool {
	return step.kindCount() == 1
}

// HasNoKind reports whether no step kind is set (e.g. the removed aggregate:
// or a typo). What such a step does is unknown: it may fetch artifacts and
// use resources the model cannot see.
func (step *Step) HasNoKind() bool {
	return step.kindCount() == 0
}

func (step *Step) kindCount() int {
	kinds := 0
	for _, isSet := range []bool{
		step.Get != nil, step.Put != nil, step.Task != nil, step.Run != nil,
		step.SetPipeline != nil, step.LoadVar != nil, step.InParallel != nil,
		step.Do != nil, step.Try != nil,
	} {
		if isSet {
			kinds++
		}
	}
	return kinds
}
