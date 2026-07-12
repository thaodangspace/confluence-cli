package markdown

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFromHTML(t *testing.T) {
	cases := []struct {
		name string
		html string
		want string
	}{
		{"empty", "", ""},
		{"whitespace", "   \n  ", ""},
		{"heading", "<h1>Title</h1>", "# Title"},
		{"h3", "<h3>Sub</h3>", "### Sub"},
		{"bold-italic", "<p><strong>b</strong> and <em>i</em></p>", "**b** and _i_"},
		{"inline-code", "<p>use <code>go test</code></p>", "use `go test`"},
		{"link", `<p><a href="https://x.io">x</a></p>`, "[x](https://x.io)"},
		{"image", `<p><img src="https://x.io/a.png" alt="pic"></p>`, "![pic](https://x.io/a.png)"},
		{"blockquote", "<blockquote><p>quoted</p></blockquote>", "> quoted"},
		{"ul", "<ul><li>a</li><li>b</li></ul>", "- a\n- b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := FromHTML(c.html)
			if err != nil {
				t.Fatalf("FromHTML(%q) error: %v", c.html, err)
			}
			if got != c.want {
				t.Fatalf("FromHTML(%q)\n got: %q\nwant: %q", c.html, got, c.want)
			}
		})
	}
}

func TestFromHTMLOrderedAndNestedList(t *testing.T) {
	html := "<ol><li>one<ul><li>sub</li></ul></li><li>two</li></ol>"
	got, err := FromHTML(html)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(got, "1. one") || !strings.Contains(got, "2. two") {
		t.Fatalf("ordered list not rendered: %q", got)
	}
	if !strings.Contains(got, "sub") {
		t.Fatalf("nested item missing: %q", got)
	}
}

func TestFromHTMLFencedCode(t *testing.T) {
	// Confluence renders code macros as <pre>; expect a fenced block.
	got, err := FromHTML("<pre>fmt.Println(\"hi\")</pre>")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(got, "```") || !strings.Contains(got, `fmt.Println("hi")`) {
		t.Fatalf("expected fenced code block, got: %q", got)
	}
}

func TestFromHTMLGFMTable(t *testing.T) {
	html := "<table><thead><tr><th>a</th><th>b</th></tr></thead>" +
		"<tbody><tr><td>1</td><td>2</td></tr></tbody></table>"
	got, err := FromHTML(html)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(got, "| a | b |") {
		t.Fatalf("table header missing: %q", got)
	}
	if !strings.Contains(got, "---") {
		t.Fatalf("table separator row missing: %q", got)
	}
	if !strings.Contains(got, "| 1 | 2 |") {
		t.Fatalf("table body missing: %q", got)
	}
}

func TestRenderNoFrontmatter(t *testing.T) {
	body := "# Hello\n\ncontent"
	if got := Render(body, nil, []string{"title"}); got != body {
		t.Fatalf("nil fm should return body verbatim, got: %q", got)
	}
	if got := Render(body, map[string]any{}, []string{"title"}); got != body {
		t.Fatalf("empty fm should return body verbatim, got: %q", got)
	}
}

func TestRenderFrontmatterOrderAndOmit(t *testing.T) {
	fm := map[string]any{
		"title":   "My Page",
		"id":      "12345",
		"spaceId": "777",
		"space":   "", // empty → omitted
		"version": 3,
	}
	order := []string{"title", "id", "space", "spaceId", "status", "version"}
	got := Render("body", fm, order)

	if !strings.HasPrefix(got, "---\n") {
		t.Fatalf("missing opening fence: %q", got)
	}
	if !strings.Contains(got, "\n---\n\nbody") {
		t.Fatalf("missing closing fence + body separation: %q", got)
	}
	if strings.Contains(got, "\nspace: ") {
		t.Fatalf("empty 'space' should be omitted: %q", got)
	}
	if !strings.Contains(got, "spaceId:") {
		t.Fatalf("present 'spaceId' should be emitted: %q", got)
	}
	if strings.Contains(got, "status:") {
		t.Fatalf("absent 'status' should be omitted: %q", got)
	}
	// Order: title before id before spaceId before version.
	iTitle := strings.Index(got, "title:")
	iID := strings.Index(got, "id:")
	iSpaceID := strings.Index(got, "spaceId:")
	iVersion := strings.Index(got, "version:")
	if !(iTitle < iID && iID < iSpaceID && iSpaceID < iVersion) {
		t.Fatalf("keys out of order: %q", got)
	}
}

func TestRenderFrontmatterReparses(t *testing.T) {
	// Values with YAML-special characters must round-trip through a real parser.
	fm := map[string]any{
		"title": "Deploy: staging #2",
		"url":   "https://x.atlassian.net/wiki/spaces/ENG/pages/12345",
	}
	got := Render("body", fm, []string{"title", "url"})

	block := got
	if i := strings.Index(got, "\n---\n\n"); i >= 0 {
		block = got[len("---\n") : i+1] // between the fences
	}
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(block), &parsed); err != nil {
		t.Fatalf("frontmatter not valid yaml: %v\n%q", err, block)
	}
	if parsed["title"] != "Deploy: staging #2" {
		t.Fatalf("title did not round-trip: %v", parsed["title"])
	}
	if parsed["url"] != "https://x.atlassian.net/wiki/spaces/ENG/pages/12345" {
		t.Fatalf("url did not round-trip: %v", parsed["url"])
	}
}
