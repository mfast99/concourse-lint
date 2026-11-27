package complete

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// The place at the cursor is found in the text, not in the model: while the
// user types, the YAML is often broken and the parser returns nothing. Block
// style is followed by indentation, flow
// style ([a, b] and {a: b}) only on the cursor's line.

// segment is one step of the path from the top of the file to the cursor:
// a key, or the item of a list.
type segment struct {
	key    string
	isItem bool
}

// place is what the text says about the cursor.
type place struct {
	// The keys and list items from the top to the mapping (at a key) or to
	// the value (at a value) of the cursor.
	path []segment
	// atKey: the cursor is on a key of the mapping at the end of path.
	atKey bool
	// valueKey is the key whose value the cursor is in (not at a key).
	valueKey string
	// inList: the value is an item of a list under valueKey.
	inList bool
	// inFlowMapping: the value is in a flow mapping under valueKey, e.g.
	// input_mapping: {source: re}.
	inFlowMapping bool
	// siblings are the other keys of the mapping at the cursor.
	siblings []string
	// wordStart is the column (from 1) where the word at the cursor starts.
	wordStart int
}

// lineShape is a line read as "<indentation><dashes><key>:" or
// "<indentation><dashes><scalar>".
type lineShape struct {
	// Columns (from 1) of the "- " list markers.
	dashColumns []int
	// The key and its column, "" when the line has no key.
	key       string
	keyColumn int
	// contentColumn is where the first thing after the dashes starts.
	contentColumn int
	empty         bool
}

var (
	dashPrefix = regexp.MustCompile(`^-(?:\s+|$)`)
	keyPattern = regexp.MustCompile(`^("[^"]*"|'[^']*'|[^\s#'"{}\[\],:][^#{}\[\],:]*?)\s*:(?:\s|$)`)
)

// shapeOf reads the start of a line.
func shapeOf(line string) lineShape {
	shape := lineShape{}
	rest := strings.TrimLeft(line, " ")
	column := 1 + len(line) - len(rest)
	if rest == "" || strings.HasPrefix(rest, "#") {
		shape.empty = true
		return shape
	}
	for {
		match := dashPrefix.FindString(rest)
		if match == "" {
			break
		}
		shape.dashColumns = append(shape.dashColumns, column)
		column += utf8.RuneCountInString(match)
		rest = rest[len(match):]
	}
	shape.contentColumn = column
	if match := keyPattern.FindStringSubmatch(rest); match != nil {
		shape.key = strings.Trim(match[1], `"'`)
		shape.keyColumn = column
	}
	return shape
}

// startsItem reports whether the line's key is the first key of a list
// item ("- key:").
func (shape lineShape) startsItem() bool {
	return shape.key != "" && len(shape.dashColumns) > 0
}

func (shape lineShape) lastDash() int {
	return shape.dashColumns[len(shape.dashColumns)-1]
}

var (
	// "<indentation><dashes>word" at the cursor: a key being typed, or a
	// scalar item of a block list.
	keyBeingTyped = regexp.MustCompile(`^(\s*)((?:-\s+)*)([\w./-]*)$`)
	// "<indentation><dashes>key: value-so-far" at the cursor.
	valueBeingTyped = regexp.MustCompile(`^(\s*)((?:-\s+)*)([\w./-]+):\s+(.*)$`)
	// The word right before the cursor.
	wordBeforeCursor = regexp.MustCompile(`[\w./-]*$`)
)

// placeAt finds the place of the cursor (line and column from 1, the column
// in characters). found is false where no completion makes sense (e.g. in
// a comment).
func placeAt(lines []string, line, column int) (place, bool) {
	if line < 1 || line > len(lines) {
		return place{}, false
	}
	current := lines[line-1]
	runes := []rune(current)
	prefix := string(runes[:min(max(column-1, 0), len(runes))])
	if strings.Contains(prefix, "#") && !strings.ContainsAny(prefix, `"'`) {
		return place{}, false
	}
	word := wordBeforeCursor.FindString(prefix)
	result := place{wordStart: column - utf8.RuneCountInString(word)}

	if match := keyBeingTyped.FindStringSubmatch(prefix); match != nil {
		indentation := utf8.RuneCountInString(match[1])
		dashes := utf8.RuneCountInString(match[2])
		keyColumn := indentation + dashes + 1
		if dashes > 0 {
			// A new list item: its first key, or a scalar item of a list.
			result.path = ownerPath(lines, line, lastDashColumn(match[1]+match[2]))
			result.path = append(result.path, segment{isItem: true})
			result.atKey = true
			result.siblings = siblingsBelow(lines, line, keyColumn)
			return result, true
		}
		result.path, result.siblings = mappingPath(lines, line, keyColumn)
		result.siblings = append(result.siblings, siblingsBelow(lines, line, keyColumn)...)
		result.atKey = true
		return result, true
	}

	if match := valueBeingTyped.FindStringSubmatch(prefix); match != nil {
		indentation := utf8.RuneCountInString(match[1])
		dashes := utf8.RuneCountInString(match[2])
		keyColumn := indentation + dashes + 1
		key, value := match[3], match[4]
		if dashes > 0 {
			result.path = ownerPath(lines, line, lastDashColumn(match[1]+match[2]))
			result.path = append(result.path, segment{isItem: true})
		} else {
			result.path, result.siblings = mappingPath(lines, line, keyColumn)
		}
		result.siblings = append(result.siblings, key)
		result.siblings = append(result.siblings, siblingsBelow(lines, line, keyColumn)...)
		result.valueKey = key
		flowValue(&result, value)
		return result, true
	}
	return place{}, false
}

// flowValue reads a value in flow style on the cursor's line: inside
// [a, b] the value is a list item; inside {a: b} it is the value of the
// inner key, under the outer key.
func flowValue(result *place, value string) {
	open := strings.LastIndexAny(value, "[{")
	if open < 0 || strings.ContainsAny(value[open:], "]}") {
		return
	}
	if value[open] == '[' {
		result.inList = true
		return
	}
	inner := value[open+1:]
	if comma := strings.LastIndex(inner, ","); comma >= 0 {
		inner = inner[comma+1:]
	}
	if strings.Contains(inner, ":") {
		result.inFlowMapping = true
	}
}

// lastDashColumn returns the column (from 1) of the last "-" in the
// indentation-and-dashes text.
func lastDashColumn(indentationAndDashes string) int {
	return utf8.RuneCountInString(indentationAndDashes[:strings.LastIndex(indentationAndDashes, "-")]) + 1
}

// mappingPath returns the path to the mapping whose keys start at
// keyColumn, seen from the line, and the keys of that mapping above the
// line.
func mappingPath(lines []string, line, keyColumn int) ([]segment, []string) {
	var siblings []string
	for index := line - 2; index >= 0; index-- {
		shape := shapeOf(lines[index])
		if shape.empty || shape.key == "" || shape.keyColumn > keyColumn {
			continue
		}
		if shape.keyColumn == keyColumn {
			siblings = append(siblings, shape.key)
			if shape.startsItem() {
				path := ownerPath(lines, index+1, shape.lastDash())
				return append(path, segment{isItem: true}), siblings
			}
			continue
		}
		// A key further left: the mapping is its value.
		path := pathTo(lines, index+1, shape)
		return append(path, segment{key: shape.key}), siblings
	}
	return nil, siblings
}

// ownerPath returns the path to the key that owns the list whose items
// start with a dash at dashColumn, seen from the line: the nearest key
// above at that column or further left that is no item of the list.
func ownerPath(lines []string, line, dashColumn int) []segment {
	for index := line - 2; index >= 0; index-- {
		shape := shapeOf(lines[index])
		if shape.empty || containsColumn(shape.dashColumns, dashColumn) || shape.key == "" || shape.keyColumn > dashColumn {
			continue
		}
		path := pathTo(lines, index+1, shape)
		return append(path, segment{key: shape.key})
	}
	return nil
}

// pathTo returns the path to the mapping that holds the key of the line.
func pathTo(lines []string, line int, shape lineShape) []segment {
	if shape.startsItem() {
		path := ownerPath(lines, line, shape.lastDash())
		return append(path, segment{isItem: true})
	}
	path, _ := mappingPath(lines, line, shape.keyColumn)
	return path
}

// siblingsBelow returns the keys of the mapping at keyColumn below the
// line, up to the end of the mapping.
func siblingsBelow(lines []string, line, keyColumn int) []string {
	var siblings []string
	for index := line; index < len(lines); index++ {
		shape := shapeOf(lines[index])
		if shape.empty {
			continue
		}
		if shape.contentColumn < keyColumn || len(shape.dashColumns) > 0 && shape.dashColumns[0] < keyColumn {
			break
		}
		if shape.key != "" && shape.keyColumn == keyColumn && len(shape.dashColumns) == 0 {
			siblings = append(siblings, shape.key)
		}
	}
	return siblings
}

func containsColumn(columns []int, column int) bool {
	for _, each := range columns {
		if each == column {
			return true
		}
	}
	return false
}
