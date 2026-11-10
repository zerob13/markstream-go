package markstream

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
)

func TestExtendedMarkdownAndEmoji(t *testing.T) {
	source := "==highlight **bold**== and ++inserted++ H~2~O x^2^ :rocket: `==literal==`\n"
	tt := ui.NewTester(func(c *ui.Context) { View(c, source, Props{Emoji: true}) }, 900, 400)
	text := strings.Join(tt.Texts(), " ")
	for _, want := range []string{"highlight bold", "inserted", "H2O", "x2", "🚀", "==literal=="} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %q", want, text)
		}
	}
	for _, unwanted := range []string{"++inserted++", "H~2~O", "x^2^", ":rocket:"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("syntax leaked: %q", text)
		}
	}
}
func TestExtendedSyntaxHealsThenFinalizesStrictly(t *testing.T) {
	s := new(Session)
	s.Append("==highlight")
	tt := ui.NewTester(func(c *ui.Context) { Stream(c, s, Props{Reveal: instant}) }, 700, 300)
	if !tt.HasText("highlight") {
		t.Fatal(tt.Texts())
	}
	s.Finish()
	tt.Frame()
	if !tt.HasText("==highlight") {
		t.Fatal("strict finalization did not restore literal", tt.Texts())
	}
}
func TestContainersDefinitionsAndNestedStreaming(t *testing.T) {
	source := "::: tip Important\n\nHello **container**.\n\n::: warning Nested\n\nNested content.\n\n:::\n\n:::\n\nTerm\n: A **definition**\n"
	for _, virtual := range []*VirtualState{nil, new(VirtualState)} {
		tt := ui.NewTester(func(c *ui.Context) { View(c, source, Props{Virtual: virtual, Reveal: instant}) }, 900, 1500)
		text := strings.Join(tt.Texts(), " ")
		for _, want := range []string{"Important", "Hello container.", "Nested", "Nested content.", "Term", "A definition"} {
			if !strings.Contains(text, want) {
				t.Errorf("missing %q in %q", want, text)
			}
		}
		if strings.Contains(text, ":::") || strings.Contains(text, ": A") {
			t.Errorf("container markers leaked: %q", text)
		}
	}
	s := new(Session)
	tt := ui.NewTester(func(c *ui.Context) { Stream(c, s, Props{Reveal: instant}) }, 900, 700)
	for _, chunk := range []string{"::: tip\n\n", "A paragraph.\n\n", "Another paragraph.\n\n", ":::\n"} {
		s.Append(chunk)
		tt.Frame()
	}
	s.Finish()
	tt.Frame()
	if !tt.HasText("Another paragraph.") {
		t.Fatal("projection corrupted append parser", tt.Texts())
	}
}
func TestParseTransformHooksAndParsedInput(t *testing.T) {
	before, after := 0, 0
	p := Props{SourceTransform: func(s string) string { before++; return strings.ReplaceAll(s, "TOKEN", "replacement") }, DocumentTransform: func(d *Document) { after++ }}
	tt := ui.NewTester(func(c *ui.Context) { View(c, "# TOKEN", p) }, 700, 300)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("replacement") || before != 1 || after != 1 {
		t.Fatalf("hooks=%d/%d text=%q", before, after, tt.Texts())
	}
	p.ParseRevision++
	tt.Frame()
	if before != 2 || after != 2 {
		t.Fatal("parse revision did not invalidate hooks")
	}
	d := Parse("# Parsed document\n\nFrom shared AST.")
	tt = ui.NewTester(func(c *ui.Context) { View(c, d.Source(), Props{Document: d}) }, 700, 400)
	if !tt.HasText("Parsed document") || !tt.HasText("From shared AST.") {
		t.Fatal(tt.Texts())
	}
}
