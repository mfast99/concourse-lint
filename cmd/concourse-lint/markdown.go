package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// lineWidth is where paragraphs and list items are wrapped. Every terminal
// is at least this wide, and longer lines are hard to read.
const lineWidth = 80

// codeIndent starts every line of a code block.
const codeIndent = "    "

// codeFence opens and closes a code block.
const codeFence = "```"

// renderMarkdown renders the Markdown of the rule pages for the terminal
// (spec/README.md, "Rules and explain"): headings followed by a line of
// "-", "**" dropped, inline code with its backticks, code blocks indented,
// links as "text (url)", list items with "- ", paragraphs and list items
// wrapped at lineWidth. It knows only what checkMarkdownSubset accepts.
func renderMarkdown(markdown string, style textStyle) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var blocks []string
	var paragraph []string
	finishParagraph := func() {
		if len(paragraph) > 0 {
			blocks = append(blocks, wrap(inlineWords(strings.Join(paragraph, " ")), "", "", style))
			paragraph = nil
		}
	}
	for index := 0; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		switch {
		case trimmed == "":
			finishParagraph()
		case strings.HasPrefix(trimmed, codeFence):
			finishParagraph()
			var code []string
			for index++; index < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[index]), codeFence); index++ {
				code = append(code, renderCodeLine(lines[index], style))
			}
			blocks = append(blocks, strings.Join(code, "\n"))
		case headingText(trimmed) != "":
			finishParagraph()
			blocks = append(blocks, renderHeading(headingText(trimmed), style))
		case isListItem(trimmed):
			finishParagraph()
			var items []string
			index, items = collectListItems(lines, index)
			var rendered []string
			for _, item := range items {
				rendered = append(rendered, wrap(inlineWords(item), "- ", "  ", style))
			}
			blocks = append(blocks, strings.Join(rendered, "\n"))
		default:
			paragraph = append(paragraph, trimmed)
		}
	}
	finishParagraph()
	return strings.Join(blocks, "\n\n")
}

// renderHeading is the heading and a line of "-" as long as it.
func renderHeading(text string, style textStyle) string {
	return style.heading(text) + "\n" + strings.Repeat("-", utf8.RuneCountInString(text))
}

func renderCodeLine(line string, style textStyle) string {
	if strings.TrimSpace(line) == "" {
		return ""
	}
	return codeIndent + style.code(line)
}

// headingText is the text of a heading line "# …" or "## …", else "".
func headingText(line string) string {
	for _, marker := range []string{"# ", "## "} {
		if strings.HasPrefix(line, marker) {
			return strings.TrimSpace(strings.TrimPrefix(line, marker))
		}
	}
	return ""
}

// isIndented reports whether the line starts with a space or a tab.
func isIndented(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

func isListItem(line string) bool {
	return strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ")
}

// collectListItems reads the list that starts at lines[start]: items and
// their indented continuation lines, up to a blank line or other text. It
// returns the index of the last line of the list and the text of each item.
func collectListItems(lines []string, start int) (last int, items []string) {
	last = start
	for index := start; index < len(lines); index++ {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		switch {
		case isListItem(trimmed) && !isIndented(line):
			items = append(items, strings.TrimSpace(trimmed[2:]))
		case trimmed != "" && isIndented(line) && len(items) > 0:
			items[len(items)-1] += " " + trimmed
		default:
			return last, items
		}
		last = index
	}
	return last, items
}

// pieceKind says how a piece of inline text is styled.
type pieceKind int

const (
	plainPiece pieceKind = iota
	boldPiece
	// Code keeps its backticks and is never broken across lines.
	codePiece
	// The target of a link, never broken across lines.
	linkPiece
)

type piece struct {
	text string
	kind pieceKind
}

// word is the pieces between two spaces, e.g. "(`build{`," is three pieces.
type word []piece

func (pieces word) width() int {
	total := 0
	for _, part := range pieces {
		total += utf8.RuneCountInString(part.text)
	}
	return total
}

func (pieces word) render(style textStyle) string {
	var rendered strings.Builder
	for _, part := range pieces {
		switch part.kind {
		case boldPiece:
			rendered.WriteString(style.bold(part.text))
		case codePiece:
			rendered.WriteString(style.code(part.text))
		case linkPiece:
			rendered.WriteString(style.link(part.text))
		default:
			rendered.WriteString(part.text)
		}
	}
	return rendered.String()
}

// inlineWords splits inline Markdown into words: code spans and link
// targets stay whole, other text breaks at spaces.
func inlineWords(text string) []word {
	var words []word
	var current word
	finishWord := func() {
		if len(current) > 0 {
			words = append(words, current)
			current = nil
		}
	}
	for _, part := range inlinePieces(text) {
		if part.kind == codePiece || part.kind == linkPiece {
			current = append(current, part)
			continue
		}
		for index, segment := range strings.Split(part.text, " ") {
			if index > 0 {
				finishWord()
			}
			if segment != "" {
				current = append(current, piece{segment, part.kind})
			}
		}
	}
	finishWord()
	return words
}

// inlinePieces parses code spans, bold text and links.
func inlinePieces(text string) []piece {
	var pieces []piece
	var plain strings.Builder
	addPiece := func(text string, kind pieceKind) {
		if plain.Len() > 0 {
			pieces = append(pieces, piece{plain.String(), plainPiece})
			plain.Reset()
		}
		pieces = append(pieces, piece{text, kind})
	}
	for len(text) > 0 {
		switch {
		case strings.HasPrefix(text, "`"):
			if end := strings.Index(text[1:], "`"); end >= 0 {
				addPiece(text[:end+2], codePiece)
				text = text[end+2:]
				continue
			}
		case strings.HasPrefix(text, "**"):
			if end := strings.Index(text[2:], "**"); end >= 0 {
				for _, inner := range inlinePieces(text[2 : end+2]) {
					if inner.kind == plainPiece {
						inner.kind = boldPiece
					}
					addPiece(inner.text, inner.kind)
				}
				text = text[end+4:]
				continue
			}
		case strings.HasPrefix(text, "["):
			if match := markdownLink.FindStringSubmatch(text); match != nil {
				plain.WriteString(match[1] + " (")
				addPiece(match[2], linkPiece)
				plain.WriteString(")")
				text = text[len(match[0]):]
				continue
			}
		}
		character, size := utf8.DecodeRuneInString(text)
		plain.WriteRune(character)
		text = text[size:]
	}
	if plain.Len() > 0 {
		pieces = append(pieces, piece{plain.String(), plainPiece})
	}
	return pieces
}

// markdownLink matches a link at the start of the text: [text](target).
var markdownLink = regexp.MustCompile(`^\[([^\]]+)\]\(([^)\s]+)\)`)

// wrap joins the words into lines of at most lineWidth columns. The first
// line starts with firstPrefix, the others with nextPrefix. A word longer
// than a line gets a line of its own.
func wrap(words []word, firstPrefix, nextPrefix string, style textStyle) string {
	var lines []string
	line, width, empty := firstPrefix, len(firstPrefix), true
	for _, next := range words {
		if !empty && width+1+next.width() > lineWidth {
			lines = append(lines, line)
			line, width, empty = nextPrefix, len(nextPrefix), true
		}
		if !empty {
			line += " "
			width++
		}
		line += next.render(style)
		width += next.width()
		empty = false
	}
	return strings.Join(append(lines, line), "\n")
}

// Patterns for checkMarkdownSubset.
var (
	codeSpan           = regexp.MustCompile("`[^`]*`")
	allowedLink        = regexp.MustCompile(`\[[^\]]+\]\([^)\s]+\)`)
	allowedBold        = regexp.MustCompile(`\*\*[^*]+\*\*`)
	numberedListItem   = regexp.MustCompile(`^\d+[.)] `)
	underscoreEmphasis = regexp.MustCompile(`(^|[\s(])_[^_\s]`)
)

// checkMarkdownSubset reports the first construct renderMarkdown does not
// know: tables, quotes, HTML, deeper headings, numbered lists, rules,
// indented code, emphasis with one star or underscore, escapes, images,
// unclosed code spans and code blocks.
func checkMarkdownSubset(markdown string) error {
	inCode, inList := false, false
	for number, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, codeFence) {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		problem := lineProblem(line, trimmed, inList)
		if problem != "" {
			return fmt.Errorf("line %d: %s: %q", number+1, problem, line)
		}
		if trimmed == "" {
			inList = false
		} else if isListItem(trimmed) {
			inList = true
		}
	}
	if inCode {
		return fmt.Errorf("a code block is never closed")
	}
	return nil
}

func lineProblem(line, trimmed string, inList bool) string {
	switch {
	case trimmed == "":
		return ""
	case isIndented(line) && !inList:
		return "indented text (only list items may continue indented)"
	case strings.HasPrefix(trimmed, "|"):
		return "table"
	case strings.HasPrefix(trimmed, ">"):
		return "quote"
	case strings.HasPrefix(trimmed, "#") && headingText(trimmed) == "":
		return "heading deeper than ##"
	case numberedListItem.MatchString(trimmed):
		return "numbered list"
	case trimmed == "---" || trimmed == "***" || trimmed == "___":
		return "horizontal rule"
	}
	text := codeSpan.ReplaceAllString(trimmed, "")
	if strings.Contains(text, "`") {
		return "unclosed code span"
	}
	if strings.Contains(text, "![") {
		return "image"
	}
	text = allowedLink.ReplaceAllString(text, "")
	text = allowedBold.ReplaceAllString(text, "")
	if isListItem(text) {
		text = text[2:]
	}
	switch {
	case strings.Contains(text, "*"):
		return "emphasis with one star"
	case underscoreEmphasis.MatchString(text):
		return "emphasis with an underscore"
	case strings.Contains(text, "<"):
		return "HTML"
	case strings.Contains(text, `\`):
		return "escape"
	}
	return ""
}
