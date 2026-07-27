package htmlnotes

import (
	"strings"
	"testing"
)

func TestFromMarkdown(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain paragraph",
			in:   "Just some text.",
			want: "<body>Just some text.</body>",
		},
		{
			name: "soft line breaks are preserved",
			in:   "line one\nline two",
			want: "<body>line one\nline two</body>",
		},
		{
			name: "paragraphs separated by a blank line",
			in:   "first\n\nsecond",
			want: "<body>first\n\nsecond</body>",
		},
		{
			name: "headings, with deeper levels demoted to h2",
			in:   "# One\n\n## Two\n\n#### Four",
			want: "<body><h1>One</h1>\n\n<h2>Two</h2>\n\n<h2>Four</h2></body>",
		},
		{
			name: "bullet list",
			in:   "- a\n- b",
			want: "<body><ul><li>a</li><li>b</li></ul></body>",
		},
		{
			name: "bullet list with other markers",
			in:   "* a\n+ b",
			want: "<body><ul><li>a</li><li>b</li></ul></body>",
		},
		{
			name: "ordered list",
			in:   "1. a\n2. b",
			want: "<body><ol><li>a</li><li>b</li></ol></body>",
		},
		{
			name: "nested list",
			in:   "- a\n  - a1\n  - a2\n- b",
			want: "<body><ul><li>a<ul><li>a1</li><li>a2</li></ul></li><li>b</li></ul></body>",
		},
		{
			name: "loose list keeps one list",
			in:   "- a\n\n- b",
			want: "<body><ul><li>a</li><li>b</li></ul></body>",
		},
		{
			name: "list following a paragraph",
			in:   "Intro:\n\n- a\n- b\n\nOutro",
			want: "<body>Intro:\n\n<ul><li>a</li><li>b</li></ul>\n\nOutro</body>",
		},
		{
			name: "emphasis",
			in:   "**bold** __also bold__ *em* _also em_ `code` ~~gone~~",
			want: "<body><strong>bold</strong> <strong>also bold</strong> <em>em</em> <em>also em</em> <code>code</code> <s>gone</s></body>",
		},
		{
			name: "underscores inside a word are left alone",
			in:   "call some_function_name now",
			want: "<body>call some_function_name now</body>",
		},
		{
			name: "inline link anchored on specific words",
			in:   "I mentioned this [in slack](https://example.slack.com/archives/C1/p2) earlier.",
			want: `<body>I mentioned this <a href="https://example.slack.com/archives/C1/p2">in slack</a> earlier.</body>`,
		},
		{
			name: "link with a title",
			in:   `[text](https://example.com "the title")`,
			want: `<body><a href="https://example.com">text</a></body>`,
		},
		{
			name: "link text keeps its emphasis",
			in:   "[**bold** link](https://example.com)",
			want: `<body><a href="https://example.com"><strong>bold</strong> link</a></body>`,
		},
		{
			name: "angle autolink",
			in:   "see <https://example.com> for more",
			want: `<body>see <a href="https://example.com">https://example.com</a> for more</body>`,
		},
		{
			name: "bare url is left as text",
			in:   "see https://example.com",
			want: "<body>see https://example.com</body>",
		},
		{
			name: "asana mention passes through as a link",
			in:   "ping [Chris](asana://1234)",
			want: `<body>ping <a href="asana://1234">Chris</a></body>`,
		},
		{
			name: "blockquote",
			in:   "> quoted line\n> and another",
			want: "<body><blockquote>quoted line\nand another</blockquote></body>",
		},
		{
			name: "fenced code block is not interpreted",
			in:   "```\nif a < b && c { *x* }\n```",
			want: "<body><pre>if a &lt; b &amp;&amp; c { *x* }</pre></body>",
		},
		{
			name: "fenced code block with a language tag",
			in:   "```go\nfmt.Println(\"hi\")\n```",
			want: "<body><pre>fmt.Println(\"hi\")</pre></body>",
		},
		{
			name: "thematic break",
			in:   "a\n\n---\n\nb",
			want: "<body>a\n\n<hr/>\n\nb</body>",
		},
		{
			name: "xml special characters are escaped",
			in:   "Fish & Chips <not a tag> \"quoted\"",
			want: "<body>Fish &amp; Chips &lt;not a tag&gt; \"quoted\"</body>",
		},
		{
			// Quotes stay raw in text content; Asana's own descriptions carry
			// them unescaped. Only the angle brackets and ampersands matter.
			name: "raw html is escaped, not passed through",
			in:   `<div onclick="x">hi</div>`,
			want: `<body>&lt;div onclick="x"&gt;hi&lt;/div&gt;</body>`,
		},
		{
			name: "url with an ampersand is escaped in the href",
			in:   "[q](https://example.com/s?a=1&b=2)",
			want: `<body><a href="https://example.com/s?a=1&amp;b=2">q</a></body>`,
		},
		{
			name: "the full shape an agent actually writes",
			in: "Hey Chris, two things:\n\n" +
				"- The **build** is green again\n" +
				"- Details are [in slack](https://example.slack.com/archives/C1/p2)\n\n" +
				"Thanks!",
			want: "<body>Hey Chris, two things:\n\n" +
				"<ul><li>The <strong>build</strong> is green again</li>" +
				`<li>Details are <a href="https://example.slack.com/archives/C1/p2">in slack</a></li></ul>` +
				"\n\nThanks!</body>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromMarkdown(tt.in)
			if err != nil {
				t.Fatalf("FromMarkdown() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("FromMarkdown()\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestFromMarkdown_Empty(t *testing.T) {
	if _, err := FromMarkdown("  \n\n "); err == nil {
		t.Fatal("expected an error for empty markdown")
	}
}

// Whatever the converter emits must satisfy the html_notes rules, so a
// converter bug surfaces locally instead of as an opaque 400 from Asana.
func TestFromMarkdown_OutputAlwaysValidates(t *testing.T) {
	inputs := []string{
		"# H\n\ntext **b** _i_ `c`\n\n- a\n  - b\n\n1. x\n\n> q\n\n---\n\n```\nraw < & >\n```",
		"[link](https://example.com/a?b=1&c=2) & more <stuff>",
		"weird ** unbalanced * markers _ here ~~",
		"- only a list",
		"&nbsp; &mdash; &notanentity;",
	}

	for _, in := range inputs {
		got, err := FromMarkdown(in)
		if err != nil {
			t.Fatalf("FromMarkdown(%q) error = %v", in, err)
		}
		if err := Validate(got); err != nil {
			t.Fatalf("FromMarkdown(%q) produced invalid html notes %q: %v", in, got, err)
		}
		if !strings.HasPrefix(got, "<body>") || !strings.HasSuffix(got, "</body>") {
			t.Fatalf("FromMarkdown(%q) = %q; want a <body> wrapper", in, got)
		}
	}
}
