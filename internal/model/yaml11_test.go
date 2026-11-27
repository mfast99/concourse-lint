package model_test

import (
	"testing"

	"github.com/mfast99/concourse-lint/internal/model"
)

// TestResolveYAML11: how Concourse (fly 8.3.4) reads an unquoted value, and
// which text ends up in a string field outside steps. Every expected text
// was read back from fly (validate-pipeline --output and its identifier
// messages).
func TestResolveYAML11(t *testing.T) {
	testCases := []struct {
		text     string
		wantKind model.ScalarKind
		wantText string
	}{
		{"1.10", model.FloatScalar, "1.1"},
		{"1.0", model.FloatScalar, "1"},
		{"3.20", model.FloatScalar, "3.2"},
		{"12", model.IntScalar, "12"},
		{"0777", model.IntScalar, "511"},
		{"08", model.FloatScalar, "8"},
		{"1e3", model.FloatScalar, "1000"},
		{"1_000", model.IntScalar, "1000"},
		{"+1", model.IntScalar, "1"},
		{".5", model.FloatScalar, "0.5"},
		{"0x1F", model.IntScalar, "31"},
		{"0b101", model.IntScalar, "5"},
		{"12345678901234567890", model.IntScalar, "12345678901234567890"},
		{"1.23456789", model.FloatScalar, "1.2345679"},
		{"1234567.0", model.FloatScalar, "1.234567e+06"},
		{"0.000001", model.FloatScalar, "1e-06"},
		{"yes", model.BoolScalar, "true"},
		{"Off", model.BoolScalar, "false"},
		{"true", model.BoolScalar, "true"},
		{"~", model.NullScalar, ""},
		{"null", model.NullScalar, ""},
		{"1.10.0", model.StringScalar, "1.10.0"},
		{"v1.10", model.StringScalar, "v1.10"},
		{"1:20", model.StringScalar, "1:20"},
		{"2024-01-01", model.StringScalar, "2024-01-01"},
		{"yEs", model.StringScalar, "yEs"},
		{"repo", model.StringScalar, "repo"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.text, func(t *testing.T) {
			scalar := model.ResolveYAML11(testCase.text)
			if scalar.Kind != testCase.wantKind || scalar.AsString() != testCase.wantText {
				t.Errorf("ResolveYAML11(%q) = kind %v, text %q; want kind %v, text %q",
					testCase.text, scalar.Kind, scalar.AsString(), testCase.wantKind, testCase.wantText)
			}
		})
	}
}

func TestAsJSON(t *testing.T) {
	tests := map[string]string{
		"1.10": "1.1", "2.0": "2", "0022": "18", "08": "8", "1e3": "1000", "1_000": "1000",
		"+1": "1", "-0": "0", ".5": "0.5", "0x1F": "31", "0o17": "15", "0b101": "5",
		"1.5": "1.5", "10": "10", "0": "0", "1.10.0": "1.10.0", "yes": "yes",
	}
	for text, want := range tests {
		if got := model.ResolveYAML11(text).AsJSON(); got != want {
			t.Errorf("ResolveYAML11(%q).AsJSON() = %q, want %q", text, got, want)
		}
	}
}
