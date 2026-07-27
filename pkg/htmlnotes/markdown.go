package htmlnotes

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// FromMarkdown converts a practical subset of Markdown to an Asana html_notes
// value. Markdown is what agents and humans naturally write; pasting it into a
// plain-text description leaves literal "**bold**" and "[text](url)" on the
// task, which is the problem this solves.
//
// Supported: ATX headings, bullet and ordered lists (including nesting),
// blockquotes, fenced code blocks, thematic breaks, paragraphs, and inline
// strong/em/code/strikethrough/links/angle-autolinks.
//
// Deliberately not supported, because Asana's element allowlist has no room for
// them: tables, images, footnotes, reference-style links, and headings below
// level 2 (### and deeper are rendered as <h2>). Raw HTML in the input is
// escaped rather than passed through. Bare URLs are left as text; wrap them in
// <> or use [text](url) to link them.
//
// Line breaks follow Asana's own convention rather than CommonMark's: a single
// newline inside a paragraph stays a line break, and blocks are separated by a
// blank line. The result is passed through Normalize, so a bug here surfaces as
// a local validation error rather than an opaque 400 from the API.
func FromMarkdown(md string) (string, error) {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	if strings.TrimSpace(md) == "" {
		return "", fmt.Errorf("markdown notes are empty")
	}

	lines := strings.Split(md, "\n")
	var blocks []string

	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "":
			i++

		case isFence(trimmed):
			block, next := parseFence(lines, i)
			blocks = append(blocks, block)
			i = next

		case thematicBreakRe.MatchString(trimmed):
			blocks = append(blocks, "<hr/>")
			i++

		case headingRe.MatchString(trimmed):
			m := headingRe.FindStringSubmatch(trimmed)
			tag := "h2"
			if len(m[1]) == 1 {
				tag = "h1"
			}
			blocks = append(blocks, "<"+tag+">"+inline(m[2])+"</"+tag+">")
			i++

		case strings.HasPrefix(trimmed, ">"):
			block, next := parseBlockquote(lines, i)
			blocks = append(blocks, block)
			i = next

		case listItemRe.MatchString(line):
			block, next := parseList(lines, i)
			blocks = append(blocks, block)
			i = next

		default:
			block, next := parseParagraph(lines, i)
			blocks = append(blocks, block)
			i = next
		}
	}

	if len(blocks) == 0 {
		return "", fmt.Errorf("markdown notes are empty")
	}

	return Normalize("<body>" + strings.Join(blocks, "\n\n") + "</body>")
}

var (
	headingRe       = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*$`)
	thematicBreakRe = regexp.MustCompile(`^(?:-{3,}|\*{3,}|_{3,})$`)
	listItemRe      = regexp.MustCompile(`^([ \t]*)([-*+]|\d+[.)])[ \t]+(.*)$`)
	fenceRe         = regexp.MustCompile("^(```|~~~)")
)

func isFence(trimmed string) bool {
	return fenceRe.MatchString(trimmed)
}

// parseFence consumes a fenced code block. Its contents are escaped verbatim —
// no inline markdown is interpreted inside code.
func parseFence(lines []string, i int) (string, int) {
	marker := strings.TrimSpace(lines[i])[:3]
	i++

	var body []string
	for i < len(lines) {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), marker) {
			i++
			break
		}
		body = append(body, lines[i])
		i++
	}

	return "<pre>" + escapeText(strings.Join(body, "\n")) + "</pre>", i
}

func parseBlockquote(lines []string, i int) (string, int) {
	var body []string
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, ">") {
			break
		}
		body = append(body, inline(strings.TrimPrefix(strings.TrimPrefix(trimmed, ">"), " ")))
		i++
	}
	return "<blockquote>" + strings.Join(body, "\n") + "</blockquote>", i
}

// parseParagraph consumes lines until a blank line or the start of another
// block. Single newlines are kept: Asana renders them as line breaks, and
// someone who typed two lines almost always wanted two lines.
func parseParagraph(lines []string, i int) (string, int) {
	var body []string
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || startsBlock(line, trimmed) {
			break
		}
		body = append(body, inline(trimmed))
		i++
	}
	return strings.Join(body, "\n"), i
}

func startsBlock(line, trimmed string) bool {
	return isFence(trimmed) ||
		thematicBreakRe.MatchString(trimmed) ||
		headingRe.MatchString(trimmed) ||
		strings.HasPrefix(trimmed, ">") ||
		listItemRe.MatchString(line)
}

// parseList consumes one list, recursing for deeper indentation levels.
func parseList(lines []string, i int) (string, int) {
	base := listIndent(lines[i])
	ordered := isOrderedMarker(lines[i])

	var items []string
	for i < len(lines) {
		// A single blank line inside a list is a loose list, not the end of it.
		if strings.TrimSpace(lines[i]) == "" {
			next := i + 1
			for next < len(lines) && strings.TrimSpace(lines[next]) == "" {
				next++
			}
			if next < len(lines) && listItemRe.MatchString(lines[next]) && listIndent(lines[next]) >= base {
				i = next
				continue
			}
			break
		}

		m := listItemRe.FindStringSubmatch(lines[i])
		if m == nil {
			break
		}

		indent := listIndent(lines[i])
		if indent < base {
			break
		}
		if indent > base {
			nested, next := parseList(lines, i)
			if len(items) > 0 {
				items[len(items)-1] += nested
			} else {
				items = append(items, nested)
			}
			i = next
			continue
		}
		if isOrderedMarker(lines[i]) != ordered {
			break
		}

		items = append(items, inline(strings.TrimSpace(m[3])))
		i++
	}

	tag := "ul"
	if ordered {
		tag = "ol"
	}

	var b strings.Builder
	b.WriteString("<" + tag + ">")
	for _, item := range items {
		b.WriteString("<li>" + item + "</li>")
	}
	b.WriteString("</" + tag + ">")

	return b.String(), i
}

// listIndent measures a list item's indentation with tabs counted as 4 columns.
func listIndent(line string) int {
	m := listItemRe.FindStringSubmatch(line)
	if m == nil {
		return 0
	}
	width := 0
	for _, r := range m[1] {
		if r == '\t' {
			width += 4
		} else {
			width++
		}
	}
	return width
}

func isOrderedMarker(line string) bool {
	m := listItemRe.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	_, err := strconv.Atoi(strings.TrimRight(m[2], ".)"))
	return err == nil
}

var (
	codeSpanRe  = regexp.MustCompile("`([^`\n]+)`")
	linkRe      = regexp.MustCompile(`\[([^\]]*)\]\(\s*([^)\s]+)(?:\s+"[^"]*")?\s*\)`)
	autolinkRe  = regexp.MustCompile(`<((?:https?|mailto|asana)://?[^>\s]+)>`)
	strongRe    = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	strongAltRe = regexp.MustCompile(`__([^_]+)__`)
	strikeRe    = regexp.MustCompile(`~~([^~]+)~~`)
	emRe        = regexp.MustCompile(`\*([^*\n]+)\*`)
	emAltRe     = regexp.MustCompile(`(^|[^\pL\pN_])_([^_\n]+)_($|[^\pL\pN_])`)
)

// inline renders one line of markdown inline content.
//
// Spans that must not be reinterpreted (code, links, autolinks) are lifted out
// into placeholders first, so escaping and emphasis cannot reach inside them.
// NUL is used as the placeholder delimiter because it cannot appear in the
// surrounding markdown.
func inline(s string) string {
	var held []string

	hold := func(html string) string {
		held = append(held, html)
		return "\x00" + strconv.Itoa(len(held)-1) + "\x00"
	}

	s = codeSpanRe.ReplaceAllStringFunc(s, func(match string) string {
		content := codeSpanRe.FindStringSubmatch(match)[1]
		return hold("<code>" + escapeText(content) + "</code>")
	})

	s = linkRe.ReplaceAllStringFunc(s, func(match string) string {
		m := linkRe.FindStringSubmatch(match)
		return hold(anchor(m[2], emphasize(escapeText(m[1]))))
	})

	s = autolinkRe.ReplaceAllStringFunc(s, func(match string) string {
		url := autolinkRe.FindStringSubmatch(match)[1]
		return hold(anchor(url, escapeText(url)))
	})

	s = emphasize(escapeText(s))

	for i, html := range held {
		s = strings.ReplaceAll(s, "\x00"+strconv.Itoa(i)+"\x00", html)
	}
	return s
}

func anchor(url, text string) string {
	if text == "" {
		text = escapeText(url)
	}
	return `<a href="` + escapeAttr(url) + `">` + text + `</a>`
}

// emphasize applies the emphasis markers. It runs on already-escaped text, so
// the only markers left are the markdown ones.
func emphasize(s string) string {
	s = strongRe.ReplaceAllString(s, "<strong>$1</strong>")
	s = strongAltRe.ReplaceAllString(s, "<strong>$1</strong>")
	s = strikeRe.ReplaceAllString(s, "<s>$1</s>")
	s = emRe.ReplaceAllString(s, "<em>$1</em>")
	s = emAltRe.ReplaceAllString(s, "$1<em>$2</em>$3")
	return s
}

// escapeText makes arbitrary text safe as XML character data. Quotes are left
// alone in text content — Asana's own descriptions carry them raw.
func escapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func escapeAttr(s string) string {
	s = escapeText(s)
	s = strings.ReplaceAll(s, `"`, "&#34;")
	return s
}
