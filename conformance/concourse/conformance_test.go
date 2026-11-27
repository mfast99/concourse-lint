package concourse_test

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/concourse/concourse/atc"

	"github.com/mfast99/concourse-lint/internal/parse"
)

// objectTypes maps every object of parse.Fields to the atc struct that
// Concourse decodes it into. Steps are not listed here: they come from
// atc.StepPrecedence, so a new step kind is found without a change here.
var objectTypes = map[string]reflect.Type{
	"pipeline":            reflect.TypeFor[atc.Config](),
	"resource":            reflect.TypeFor[atc.ResourceConfig](),
	"resource_type":       reflect.TypeFor[atc.ResourceType](),
	"job":                 reflect.TypeFor[atc.JobConfig](),
	"group":               reflect.TypeFor[atc.GroupConfig](),
	"var_source":          reflect.TypeFor[atc.VarSourceConfig](),
	"display":             reflect.TypeFor[atc.DisplayConfig](),
	"build_log_retention": reflect.TypeFor[atc.BuildLogRetention](),
	"in_parallel":         reflect.TypeFor[atc.InParallelConfig](),
	"across_variable":     reflect.TypeFor[atc.AcrossVarConfig](),
	"task_config":         reflect.TypeFor[atc.TaskConfig](),
	"image_resource":      reflect.TypeFor[atc.ImageResource](),
	"task_run":            reflect.TypeFor[atc.TaskRunConfig](),
	"task_input":          reflect.TypeFor[atc.TaskInputConfig](),
	"task_output":         reflect.TypeFor[atc.TaskOutputConfig](),
	"task_cache":          reflect.TypeFor[atc.TaskCacheConfig](),
}

// knownDifference is a key on which the linter deliberately differs from
// atc. Every entry says why.
type knownDifference struct {
	// object is a name of parse.Fields, or "every step" for "step" and all
	// "step:<kind>".
	object string
	key    string
	reason string
}

// knownDifferences lists the deliberate differences. Keep it short: every
// entry is a place where the linter and Concourse disagree.
var knownDifferences = []knownDifference{
	{
		object: "every step",
		key:    "fail_fast",
		reason: "atc lists it in AcrossStep, so it only counts on a step with across; " +
			"the parser reads it there and reports CL001 anywhere else, like Concourse",
	},
}

// TestObjectsMatchConcourse: every object of parse.Fields has exactly the
// keys of its atc struct, and every object the parser knows is checked.
func TestObjectsMatchConcourse(t *testing.T) {
	for object, structType := range objectTypes {
		compareKeys(t, object, parse.Fields(object), jsonKeys(structType))
	}
}

// TestPrototypesMatchConcourse: a prototype has the keys of a resource type
// in the parser, so atc.Prototype must have them too.
func TestPrototypesMatchConcourse(t *testing.T) {
	compareKeys(t, "prototype", parse.Fields("resource_type"), jsonKeys(reflect.TypeFor[atc.Prototype]()))
}

// TestStepKindsMatchConcourse: the parser knows exactly the step kinds of
// atc.StepPrecedence, and a step of each kind has its kind key, its fields
// and the keys of every modifier (hooks, across, attempts, timeout).
func TestStepKindsMatchConcourse(t *testing.T) {
	kinds, modifierKeys := concourseSteps()
	compareKeys(t, "step kinds", parse.StepKinds(), slices.Sorted(maps.Keys(kinds)))
	compareKeys(t, "step", parse.Fields("step"), union(slices.Collect(maps.Keys(kinds)), modifierKeys))
	for kind, kindKeys := range kinds {
		compareKeys(t, "step:"+kind, parse.Fields("step:"+kind), union(kindKeys, modifierKeys))
	}
}

// concourseSteps splits atc.StepPrecedence into the step kinds, each with
// its keys, and the modifiers, which wrap another step (atc.StepWrapper):
// their keys are allowed on a step of every kind.
func concourseSteps() (kinds map[string][]string, modifierKeys []string) {
	kinds = map[string][]string{}
	for _, detector := range atc.StepPrecedence {
		stepConfig := detector.New()
		stepKeys := jsonKeys(reflect.TypeOf(stepConfig).Elem())
		if _, isModifier := stepConfig.(atc.StepWrapper); isModifier {
			modifierKeys = append(modifierKeys, stepKeys...)
			continue
		}
		kinds[detector.Key] = stepKeys
	}
	return kinds, modifierKeys
}

// jsonKeys returns the keys encoding/json reads into a struct: the name of
// the json tag, or the field name without one. Embedded structs without a
// tag add their keys; "-" and unexported fields are skipped.
func jsonKeys(structType reflect.Type) []string {
	var keys []string
	for field := range structType.Fields() {
		tagName, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if tagName == "-" {
			continue
		}
		if field.Anonymous && tagName == "" {
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Pointer {
				embeddedType = embeddedType.Elem()
			}
			if embeddedType.Kind() == reflect.Struct {
				keys = append(keys, jsonKeys(embeddedType)...)
				continue
			}
		}
		if !field.IsExported() {
			continue
		}
		if tagName == "" {
			tagName = field.Name
		}
		keys = append(keys, tagName)
	}
	return union(keys)
}

// compareKeys reports every key that only one side has, unless it is a
// known difference.
func compareKeys(t *testing.T, object string, linterKeys, concourseKeys []string) {
	t.Helper()
	for _, key := range concourseKeys {
		if !slices.Contains(linterKeys, key) && !isKnownDifference(object, key) {
			t.Errorf("%s: Concourse knows %q, the linter does not (CL018 would report it)", object, key)
		}
	}
	for _, key := range linterKeys {
		if !slices.Contains(concourseKeys, key) && !isKnownDifference(object, key) {
			t.Errorf("%s: the linter knows %q, Concourse does not (a typo would pass)", object, key)
		}
	}
}

func isKnownDifference(object, key string) bool {
	return slices.ContainsFunc(knownDifferences, func(difference knownDifference) bool {
		isStep := object == "step" || strings.HasPrefix(object, "step:")
		matchesObject := difference.object == object || (difference.object == "every step" && isStep)
		return matchesObject && difference.key == key
	})
}

// union returns the keys of all lists, sorted and without duplicates.
func union(lists ...[]string) []string {
	var keys []string
	for _, list := range lists {
		keys = append(keys, list...)
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}
