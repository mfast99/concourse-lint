package model

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// yaml11Booleans are the spellings YAML 1.1 reads as booleans: the table of
// the YAML library fly uses, checked with fly 8.3.4 (spec/README.md, "Verified
// with fly"). Mixed case like "yEs" is not in it, and fly rejects it.
var yaml11Booleans = map[string]bool{
	"y": true, "Y": true,
	"yes": true, "Yes": true, "YES": true,
	"on": true, "On": true, "ON": true,
	"true": true, "True": true, "TRUE": true,
	"n": false, "N": false,
	"no": false, "No": false, "NO": false,
	"off": false, "Off": false, "OFF": false,
	"false": false, "False": false, "FALSE": false,
}

// YAML11Boolean reports whether the text of an unquoted value is a boolean
// in YAML 1.1, and which one. goccy reads YAML 1.2, where only true and false
// are booleans; Concourse reads YAML 1.1.
func YAML11Boolean(text string) (value bool, isBoolean bool) {
	value, isBoolean = yaml11Booleans[text]
	return value, isBoolean
}

// ScalarKind is what YAML 1.1 reads an unquoted value as.
type ScalarKind int

const (
	StringScalar ScalarKind = iota
	NullScalar
	BoolScalar
	IntScalar
	FloatScalar
)

// YAML11Scalar is an unquoted value as Concourse reads it (YAML 1.1, the
// rules of the yaml.v2 library fly uses).
type YAML11Scalar struct {
	Kind ScalarKind
	// The text as written.
	Text string
	// BoolScalar: the value.
	Bool bool
	// IntScalar: the number in decimal, e.g. "511" for 0777.
	Integer string
	// FloatScalar: the number.
	Float float64
}

// yamlStyleFloat is the float syntax of YAML 1.1 as yaml.v2 accepts it.
var yamlStyleFloat = regexp.MustCompile(`^[-+]?(\.[0-9]+|[0-9]+(\.[0-9]*)?)([eE][-+]?[0-9]+)?$`)

// ResolveYAML11 reads an unquoted value the way Concourse does. goccy reads
// YAML 1.2, where for example 0777 and yes are no numbers or booleans.
func ResolveYAML11(text string) YAML11Scalar {
	scalar := YAML11Scalar{Kind: StringScalar, Text: text}
	switch text {
	case "", "~", "null", "Null", "NULL":
		scalar.Kind = NullScalar
		return scalar
	}
	if boolean, isBoolean := YAML11Boolean(text); isBoolean {
		scalar.Kind, scalar.Bool = BoolScalar, boolean
		return scalar
	}
	// Only values that start like a number can be one.
	if !strings.ContainsRune("+-.0123456789", rune(text[0])) {
		return scalar
	}
	plain := strings.ReplaceAll(text, "_", "")
	// Base 0 reads 0x as hex, 0b as binary, 0o and a leading 0 as octal.
	if integer, err := strconv.ParseInt(plain, 0, 64); err == nil {
		scalar.Kind, scalar.Integer = IntScalar, strconv.FormatInt(integer, 10)
		return scalar
	}
	if integer, err := strconv.ParseUint(plain, 0, 64); err == nil {
		scalar.Kind, scalar.Integer = IntScalar, strconv.FormatUint(integer, 10)
		return scalar
	}
	if yamlStyleFloat.MatchString(plain) {
		if number, err := strconv.ParseFloat(plain, 64); err == nil {
			scalar.Kind, scalar.Float = FloatScalar, number
		}
	}
	return scalar
}

// AsString is the text that Concourse puts into a string field outside
// steps. Its YAML library (sigs.k8s.io/yaml) turns numbers and booleans into
// text when the field is a string, floats with 32-bit precision ("1.10"
// becomes "1.1", "1.23456789" becomes "1.2345679"); checked with fly.
func (scalar YAML11Scalar) AsString() string {
	switch scalar.Kind {
	case NullScalar:
		return ""
	case BoolScalar:
		return strconv.FormatBool(scalar.Bool)
	case IntScalar:
		return scalar.Integer
	case FloatScalar:
		return strconv.FormatFloat(scalar.Float, 'g', -1, 32)
	}
	return scalar.Text
}

// AsJSON is the text of the number that Concourse passes on in a free-form
// block (source, params, vars, ...): fly turns these blocks into JSON, so a
// float is written like encoding/json does it ("1.10" becomes 1.1, "1e3"
// becomes 1000). For anything but a number it returns the text unchanged.
func (scalar YAML11Scalar) AsJSON() string {
	switch scalar.Kind {
	case IntScalar:
		return scalar.Integer
	case FloatScalar:
		absolute := math.Abs(scalar.Float)
		format := byte('f')
		if absolute != 0 && (absolute < 1e-6 || absolute >= 1e21) {
			format = 'e'
		}
		text := strconv.FormatFloat(scalar.Float, format, -1, 64)
		if format == 'e' {
			// encoding/json writes 1e-07 as 1e-7.
			if length := len(text); length >= 4 && text[length-4] == 'e' && text[length-3] == '-' && text[length-2] == '0' {
				text = text[:length-2] + text[length-1:]
			}
		}
		return text
	}
	return scalar.Text
}
