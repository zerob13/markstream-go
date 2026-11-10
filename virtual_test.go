package markstream

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSingleDocumentVirtualization(t *testing.T) {
	var source strings.Builder
	for i := range 2000 {
		fmt.Fprintf(&source, "## Section %d\n\nParagraph **%d** with a [shared reference][go].\n\n", i, i)
	}
	source.WriteString("[go]: https://go.dev\n\nFootnote[^n].\n\n[^n]: Shared footnote\n")
	virtual := new(VirtualState)
	built := 0
	props := Props{Virtual: virtual, Reveal: instant, Block: func(c *ui.Context, d *Document, i int) bool { built++; return false }}
	tt := ui.NewTester(func(c *ui.Context) { View(c, source.String(), props) }, 800, 600)
	if !tt.HasText("Section 0") {
		t.Fatal(tt.Texts())
	}
	if built > 100 {
		t.Fatalf("built %d blocks instead of visible range", built)
	}
	if virtual.Metrics().NodeCount < 4000 {
		t.Fatalf("missing nodes: %+v", virtual.Metrics())
	}
	virtual.ScrollTo(2000, ui.Start)
	tt.Frame()
	tt.Frame()
	if !strings.Contains(strings.Join(tt.Texts(), " "), "Section 1000") {
		t.Fatal("could not scroll to middle of single document")
	}
	snapshot := virtual.Capture()
	bytes, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var saved VirtualSnapshot
	if err = json.Unmarshal(bytes, &saved); err != nil {
		t.Fatal(err)
	}
	virtual.Reset()
	virtual.Restore(saved)
	tt.Frame()
	tt.Frame()
	tt.Frame()
	if first, _ := virtual.List.Visible(); first < 1995 || first > 2005 {
		t.Fatalf("anchor not restored: %d snapshot=%+v", first, snapshot)
	}
}
func TestVirtualDocumentPreservesGlobalReferencesAndFootnotes(t *testing.T) {
	src := "# Top\n\nSee [Go][ref].\n\n" + strings.Repeat("A paragraph.\n\n", 100) + "\nFootnote[^n].\n\n[ref]: https://go.dev\n\n[^n]: Footnote content\n"
	v := new(VirtualState)
	tt := ui.NewTester(func(c *ui.Context) { View(c, src, Props{Virtual: v, Reveal: instant}) }, 700, 400)
	if err := tt.Click("Go"); err != nil {
		t.Fatal(err)
	}
	if len(tt.OpenedURLs()) != 1 || tt.OpenedURLs()[0] != "https://go.dev" {
		t.Fatal(tt.OpenedURLs())
	}
	v.ScrollToEnd()
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Footnote content") {
		t.Fatal("global footnote lost by block virtualization", tt.Texts())
	}
}

func TestVirtualFootnoteNavigationRoundTrip(t *testing.T) {
	source := "First reference[^n].\n\n" + strings.Repeat("Paragraph.\n\n", 150) + "[^n]: Final footnote.\n"
	virtual := new(VirtualState)
	tt := ui.NewTester(func(c *ui.Context) { View(c, source, Props{Virtual: virtual}) }, 800, 400)
	if err := tt.Click("[1]"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Final footnote.") {
		t.Fatal("forward footnote link failed", tt.Texts())
	}
	if err := tt.Click("Back to footnote reference n"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !strings.Contains(strings.Join(tt.Texts(), " "), "First reference") {
		t.Fatal("backlink failed", tt.Texts())
	}
}

func TestVirtualAnchorSurvivesPrepend(t *testing.T) {
	source := ""
	for i := range 800 {
		source += fmt.Sprintf("Paragraph %04d.\n\n", i)
	}
	virtual := new(VirtualState)
	tt := ui.NewTester(func(c *ui.Context) { View(c, source, Props{Virtual: virtual, Reveal: instant}) }, 800, 500)
	virtual.ScrollTo(400, ui.Start)
	tt.Frame()
	tt.Frame()
	before := strings.Join(tt.Texts(), " ")
	source = "Inserted paragraph.\n\n" + source
	tt.Frame()
	tt.Frame()
	after := strings.Join(tt.Texts(), " ")
	if !strings.Contains(before, "Paragraph 0400.") || !strings.Contains(after, "Paragraph 0400.") {
		t.Fatal("prepend displaced the visible anchor", before, after)
	}
}

func TestVirtualRestoreRejectsIncompatibleWidthBeforeScrolling(t *testing.T) {
	source := strings.Repeat("Paragraph.\n\n", 800)
	virtual := new(VirtualState)
	tt := ui.NewTester(func(c *ui.Context) { View(c, source, Props{Virtual: virtual}) }, 800, 500)
	virtual.ScrollTo(400, ui.Start)
	tt.Frame()
	tt.Frame()
	saved := virtual.Capture()
	saved.Width += 200
	virtual.Reset()
	virtual.Restore(saved)
	tt.Frame()
	tt.Frame()
	tt.Frame()
	if virtual.restore != nil || virtual.metrics.First > 10 {
		t.Fatal("invalid-width cache moved the viewport", virtual.metrics)
	}
}

func TestVirtualRestorePreservesPartialRowOffset(t *testing.T) {
	source := strings.Repeat("A paragraph with **formatted text**.\n\n", 800)
	v := new(VirtualState)
	tt := ui.NewTester(func(c *ui.Context) { View(c, source, Props{Virtual: v}) }, 800, 500)
	v.ScrollTo(400, ui.Start)
	tt.Frame()
	tt.Frame()
	v.Scroll.Y += 23
	tt.Frame()
	tt.Frame()
	saved := v.Capture()
	if saved.Offset < 1 {
		t.Fatal("fixture did not scroll inside row", saved.Offset)
	}
	v.Reset()
	v.Restore(saved)
	for range 6 {
		tt.Frame()
	}
	restored := v.Capture()
	if restored.Anchor != saved.Anchor || restored.Offset-saved.Offset > 1 || saved.Offset-restored.Offset > 1 {
		t.Fatalf("partial anchor lost: before=%d/%g after=%d/%g", saved.Anchor, saved.Offset, restored.Anchor, restored.Offset)
	}
}

func BenchmarkVirtualDocumentFrame(b *testing.B) {
	var source strings.Builder
	for i := range 2500 {
		fmt.Fprintf(&source, "## Section %d\n\nA **native** paragraph with `code` and [Go](https://go.dev).\n\n", i)
	}
	text := source.String()
	virtual := new(VirtualState)
	tt := ui.NewTester(func(c *ui.Context) { View(c, text, Props{Virtual: virtual}) }, 800, 600)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tt.Frame()
	}
}
