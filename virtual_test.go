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
