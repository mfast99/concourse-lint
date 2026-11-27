package main

import (
	"io"
	"os"
	"strings"
)

// Select Graphic Rendition codes. Only bold, underline and the 16 basic
// colors: every terminal knows them, and the basic colors follow the
// user's theme (light or dark).
const (
	styleBold      = "1"
	styleUnderline = "4"
	colorRed       = "31"
	colorYellow    = "33"
	colorBlue      = "34"
	colorCyan      = "36"
)

// textStyle adds colors to text output, or nothing. Styles only wrap text
// in escape sequences: without them the text is the same as without colors
// (spec/README.md, "Rules and explain").
type textStyle struct {
	colors bool
}

// styleFor returns the style for output to the writer.
func styleFor(output io.Writer) textStyle {
	return textStyle{colors: colorsFor(output)}
}

func (style textStyle) apply(text string, codes ...string) string {
	if !style.colors || text == "" || len(codes) == 0 {
		return text
	}
	return "\x1b[" + strings.Join(codes, ";") + "m" + text + "\x1b[0m"
}

func (style textStyle) bold(text string) string {
	return style.apply(text, styleBold)
}

func (style textStyle) heading(text string) string {
	return style.apply(text, styleBold, styleUnderline)
}

func (style textStyle) code(text string) string {
	return style.apply(text, colorCyan)
}

// severity colors a severity by its weight; "off" stays plain.
func (style textStyle) severity(severity string) string {
	switch severity {
	case "error":
		return style.apply(severity, styleBold, colorRed)
	case "warning":
		return style.apply(severity, colorYellow)
	case "info":
		return style.apply(severity, colorBlue)
	default:
		return severity
	}
}

// colorsFor reports whether output to the writer gets colors: only a
// terminal, only when the environment allows it, and on Windows only when
// the console accepts escape sequences.
func colorsFor(output io.Writer) bool {
	file, isFile := output.(*os.File)
	if !isFile || !colorsAllowed(os.Getenv) {
		return false
	}
	return enableColors(file)
}

// colorsAllowed follows https://no-color.org (NO_COLOR set and not empty)
// and TERM=dumb.
func colorsAllowed(getenv func(string) string) bool {
	return getenv("NO_COLOR") == "" && getenv("TERM") != "dumb"
}

func (style textStyle) link(text string) string {
	return style.apply(text, styleUnderline)
}
