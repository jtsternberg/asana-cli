// Package htmlnotes prepares and validates rich-text task descriptions for
// Asana's html_notes field.
//
// Asana is unusually strict about html_notes. The value must be well-formed
// XML with a single <body> root, may only use a short allowlist of elements,
// and only <a> may carry attributes. Anything else is rejected with a 400 and
// an error message that does not say which element was at fault. This package
// exists so those mistakes are caught locally, with a message that names the
// problem, before a request is made.
package htmlnotes

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// allowedElements is the complete set of elements Asana accepts in html_notes.
var allowedElements = map[string]bool{
	"body":       true,
	"strong":     true,
	"em":         true,
	"u":          true,
	"s":          true,
	"code":       true,
	"ol":         true,
	"ul":         true,
	"li":         true,
	"a":          true,
	"blockquote": true,
	"pre":        true,
	"h1":         true,
	"h2":         true,
	"hr":         true,
	"img":        true,
}

// voidElements are the allowed elements that carry no children. Callers
// habitually write them HTML-style (<hr>), which is not well-formed XML.
var voidElements = []string{"hr", "img"}

// attributableElements may carry attributes. Asana's rule is effectively
// "<a> only" — href for links, data-asana-gid for @-mentions — but <img> is
// meaningless without data-asana-gid, so it is permitted here too.
var attributableElements = map[string]bool{"a": true, "img": true}

// AllowedElements returns the allowlist, sorted, for help text and errors.
func AllowedElements() []string {
	names := make([]string, 0, len(allowedElements))
	for name := range allowedElements {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Validate reports whether s is acceptable as an Asana html_notes value.
// It does not modify s; see Normalize for the forgiving entry point.
func Validate(s string) error {
	decoder := xml.NewDecoder(strings.NewReader(s))
	// Asana's allowlist has no place for entities we would have to expand, and
	// an unknown entity should surface as an error rather than silently vanish.
	decoder.Strict = true

	depth := 0
	roots := 0

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("html notes are not well-formed XML: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			name := t.Name.Local
			if depth == 0 {
				roots++
				if roots > 1 || name != "body" {
					return errSingleBodyRoot()
				}
			}
			if !allowedElements[name] {
				return fmt.Errorf(
					"<%s> is not allowed in Asana html notes; allowed elements are: %s",
					name, strings.Join(AllowedElements(), " "),
				)
			}
			if len(t.Attr) > 0 && !attributableElements[name] {
				return fmt.Errorf(
					"<%s> carries attributes but only <a> and <img> may carry attributes in Asana html notes",
					name,
				)
			}
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && len(strings.TrimSpace(string(t))) > 0 {
				return errSingleBodyRoot()
			}
		}
	}

	if roots != 1 {
		return errSingleBodyRoot()
	}
	return nil
}

func errSingleBodyRoot() error {
	return fmt.Errorf("html notes must be a single root <body>...</body> element")
}

// Normalize repairs the mistakes that are unambiguous to repair, then
// validates. It wraps a bare fragment in <body>, closes void elements written
// HTML-style (<hr> becomes <hr/>), and escapes ampersands that are not part of
// an XML entity. Everything else — a disallowed element, a stray attribute,
// an unclosed tag — is reported rather than guessed at.
func Normalize(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("html notes are empty")
	}

	s = escapeStrayAmpersands(s)
	s = closeVoidElements(s)

	if !strings.HasPrefix(s, "<body>") && !strings.HasPrefix(s, "<body ") {
		s = "<body>" + s + "</body>"
	}

	if err := Validate(s); err != nil {
		return "", err
	}
	return s, nil
}

// Resolve interprets a flag value that may point somewhere else:
//
//	"-"      read all of stdin
//	"@path"  read the file at path
//	"@@text" the literal text "@text"
//
// Anything else is used verbatim. It performs no validation; callers pass the
// result to Normalize.
func Resolve(value string, stdin io.Reader) (string, error) {
	switch {
	case value == "-":
		if stdin == nil {
			return "", fmt.Errorf("cannot read html notes from stdin: no input stream available")
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("failed to read html notes from stdin: %w", err)
		}
		return string(data), nil
	case strings.HasPrefix(value, "@@"):
		return value[1:], nil
	case strings.HasPrefix(value, "@"):
		path := value[1:]
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read html notes from %s: %w", path, err)
		}
		return string(data), nil
	default:
		return value, nil
	}
}

// xmlEntity matches the entity references XML itself understands: the five
// predefined names plus numeric character references.
var xmlEntity = regexp.MustCompile(`^&(?:amp|lt|gt|apos|quot|#[0-9]+|#[xX][0-9a-fA-F]+);`)

// htmlEntityCodepoints maps the HTML named entities that show up most often in
// hand-written and model-written markup to numeric references, which are valid
// XML. Anything outside this map gets escaped instead of dropped.
var htmlEntityCodepoints = map[string]string{
	"nbsp":   "&#160;",
	"ndash":  "&#8211;",
	"mdash":  "&#8212;",
	"hellip": "&#8230;",
	"lsquo":  "&#8216;",
	"rsquo":  "&#8217;",
	"ldquo":  "&#8220;",
	"rdquo":  "&#8221;",
	"copy":   "&#169;",
	"reg":    "&#174;",
	"trade":  "&#8482;",
	"bull":   "&#8226;",
	"middot": "&#183;",
}

var namedEntity = regexp.MustCompile(`^&([a-zA-Z][a-zA-Z0-9]*);`)

// escapeStrayAmpersands makes every '&' XML-safe: valid XML entities are kept,
// well-known HTML named entities are converted to their numeric equivalent, and
// anything else becomes &amp; so it renders as typed instead of 400ing.
func escapeStrayAmpersands(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		if s[i] != '&' {
			b.WriteByte(s[i])
			i++
			continue
		}

		rest := s[i:]
		if match := xmlEntity.FindString(rest); match != "" {
			b.WriteString(match)
			i += len(match)
			continue
		}
		if match := namedEntity.FindStringSubmatch(rest); match != nil {
			if numeric, ok := htmlEntityCodepoints[strings.ToLower(match[1])]; ok {
				b.WriteString(numeric)
				i += len(match[0])
				continue
			}
		}

		b.WriteString("&amp;")
		i++
	}

	return b.String()
}

// closeVoidElements rewrites <hr> to <hr/> (and the same for <img>), leaving
// tags that are already self-closed untouched.
func closeVoidElements(s string) string {
	for _, name := range voidElements {
		re := regexp.MustCompile(`(?i)<` + name + `((?:\s[^<>]*?)?)\s*/?>`)
		s = re.ReplaceAllString(s, "<"+name+"$1/>")
	}
	return s
}
