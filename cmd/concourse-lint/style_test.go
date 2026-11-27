package main

import (
	"bytes"
	"regexp"
	"testing"
)

// TestColorsAllowed: NO_COLOR (when not empty) and TERM=dumb switch colors
// off (spec/README.md, "Rules and explain").
func TestColorsAllowed(t *testing.T) {
	cases := []struct {
		environment map[string]string
		want        bool
	}{
		{map[string]string{}, true},
		{map[string]string{"TERM": "xterm-kitty"}, true},
		{map[string]string{"NO_COLOR": ""}, true},
		{map[string]string{"NO_COLOR": "1"}, false},
		{map[string]string{"TERM": "dumb"}, false},
	}
	for _, testCase := range cases {
		getenv := func(name string) string { return testCase.environment[name] }
		if got := colorsAllowed(getenv); got != testCase.want {
			t.Errorf("environment %v: colorsAllowed = %v, want %v", testCase.environment, got, testCase.want)
		}
	}
}

// TestNoColorsWithoutTerminal: a writer that is no file gets no colors.
func TestNoColorsWithoutTerminal(t *testing.T) {
	if colorsFor(&bytes.Buffer{}) {
		t.Error("colors for a buffer")
	}
}

// escapeSequence matches the SGR sequences the styles write.
var escapeSequence = regexp.MustCompile("\x1b\\[[0-9;]*m")

// TestStylesOnlyAddEscapes: colors only add escape sequences, the text
// stays the same; without colors nothing is added.
func TestStylesOnlyAddEscapes(t *testing.T) {
	plain, colored := textStyle{}, textStyle{colors: true}
	cases := []struct {
		name  string
		apply func(textStyle) string
	}{
		{"bold", func(style textStyle) string { return style.bold("CL002") }},
		{"heading", func(style textStyle) string { return style.heading("Fix") }},
		{"code", func(style textStyle) string { return style.code("`get:`") }},
		{"link", func(style textStyle) string { return style.link("https://example.com") }},
		{"error", func(style textStyle) string { return style.severity("error") }},
		{"warning", func(style textStyle) string { return style.severity("warning") }},
		{"info", func(style textStyle) string { return style.severity("info") }},
		{"off", func(style textStyle) string { return style.severity("off") }},
	}
	for _, testCase := range cases {
		withoutColors, withColors := testCase.apply(plain), testCase.apply(colored)
		if escapeSequence.MatchString(withoutColors) {
			t.Errorf("%s: escape sequence without colors: %q", testCase.name, withoutColors)
		}
		if stripped := escapeSequence.ReplaceAllString(withColors, ""); stripped != withoutColors {
			t.Errorf("%s: %q without escapes is %q, want %q", testCase.name, withColors, stripped, withoutColors)
		}
		if testCase.name != "off" && withColors == withoutColors {
			t.Errorf("%s: no style with colors", testCase.name)
		}
	}
	if got := colored.severity("error"); got != "\x1b[1;31merror\x1b[0m" {
		t.Errorf("error = %q, want bold red", got)
	}
}
