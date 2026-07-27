package htmlnotes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string // substring; empty means valid
	}{
		{
			name: "minimal body",
			in:   "<body>Hello</body>",
		},
		{
			name: "every allowed element",
			in: "<body><h1>H</h1><h2>H</h2><strong>b</strong><em>i</em><u>u</u><s>s</s>" +
				"<code>c</code><pre>p</pre><blockquote>q</blockquote>" +
				"<ul><li>a</li></ul><ol><li>b</li></ol><hr/>" +
				`<a href="https://example.com">link</a><img/></body>`,
		},
		{
			name:    "missing body root",
			in:      "Hello",
			wantErr: "single root <body>",
		},
		{
			name:    "wrong root element",
			in:      "<div>Hello</div>",
			wantErr: "single root <body>",
		},
		{
			name:    "two roots",
			in:      "<body>a</body><body>b</body>",
			wantErr: "single root <body>",
		},
		{
			name:    "disallowed element",
			in:      "<body><p>Hello</p></body>",
			wantErr: "<p> is not allowed",
		},
		{
			name:    "br is not allowed",
			in:      "<body>one<br/>two</body>",
			wantErr: "<br> is not allowed",
		},
		{
			name:    "attributes only on anchors and images",
			in:      `<body><strong class="x">b</strong></body>`,
			wantErr: `only <a> and <img> may carry attributes`,
		},
		{
			name: "image attachment reference",
			in:   `<body><img data-asana-gid="123"/></body>`,
		},
		{
			name: "anchor mention",
			in:   `<body><a data-asana-gid="123"/></body>`,
		},
		{
			name:    "not well formed",
			in:      "<body><strong>unclosed</body>",
			wantErr: "not well-formed",
		},
		{
			name:    "bare ampersand",
			in:      "<body>Fish & Chips</body>",
			wantErr: "not well-formed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.in)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v; want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil; want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() = %v; want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{
			name: "already wrapped",
			in:   "<body>Hello</body>",
			want: "<body>Hello</body>",
		},
		{
			name: "wraps bare fragment",
			in:   "<strong>Hi</strong>",
			want: "<body><strong>Hi</strong></body>",
		},
		{
			name: "trims surrounding whitespace before wrapping",
			in:   "\n  <ul><li>a</li></ul>  \n",
			want: "<body><ul><li>a</li></ul></body>",
		},
		{
			name: "closes void hr",
			in:   "<body>a<hr>b</body>",
			want: "<body>a<hr/>b</body>",
		},
		{
			name: "closes void img with attributes",
			in:   `<body><img data-asana-gid="1"></body>`,
			want: `<body><img data-asana-gid="1"/></body>`,
		},
		{
			name: "leaves already-closed void elements alone",
			in:   "<body>a<hr/>b</body>",
			want: "<body>a<hr/>b</body>",
		},
		{
			name: "escapes bare ampersand",
			in:   "<body>Fish & Chips</body>",
			want: "<body>Fish &amp; Chips</body>",
		},
		{
			name: "keeps valid xml entities",
			in:   "<body>a &amp; b &lt; c &gt; d &#39; e &#x27; f</body>",
			want: "<body>a &amp; b &lt; c &gt; d &#39; e &#x27; f</body>",
		},
		{
			name: "converts common html entities to numeric form",
			in:   "<body>a&nbsp;b&mdash;c</body>",
			want: "<body>a&#160;b&#8212;c</body>",
		},
		{
			name: "escapes unknown entity-looking text",
			in:   "<body>Rock &amp;roll &foo; done</body>",
			want: "<body>Rock &amp;roll &amp;foo; done</body>",
		},
		{
			name:    "still rejects disallowed elements",
			in:      "<body><p>nope</p></body>",
			wantErr: "<p> is not allowed",
		},
		{
			name:    "empty input",
			in:      "   ",
			wantErr: "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize(tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Normalize() error = %v; want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Normalize() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notes.html")
	if err := os.WriteFile(file, []byte("<body>from file</body>\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("literal value", func(t *testing.T) {
		got, err := Resolve("<body>inline</body>", nil)
		if err != nil {
			t.Fatal(err)
		}
		if got != "<body>inline</body>" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("file reference", func(t *testing.T) {
		got, err := Resolve("@"+file, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got != "<body>from file</body>\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := Resolve("@"+filepath.Join(dir, "nope.html"), nil)
		if err == nil || !strings.Contains(err.Error(), "nope.html") {
			t.Fatalf("expected error naming the file, got %v", err)
		}
	})

	t.Run("stdin", func(t *testing.T) {
		got, err := Resolve("-", strings.NewReader("<body>piped</body>"))
		if err != nil {
			t.Fatal(err)
		}
		if got != "<body>piped</body>" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("stdin without a reader", func(t *testing.T) {
		_, err := Resolve("-", nil)
		if err == nil || !strings.Contains(err.Error(), "stdin") {
			t.Fatalf("expected stdin error, got %v", err)
		}
	})

	t.Run("escaped leading at-sign", func(t *testing.T) {
		got, err := Resolve("@@literal", nil)
		if err != nil {
			t.Fatal(err)
		}
		if got != "@literal" {
			t.Fatalf("got %q", got)
		}
	})
}
