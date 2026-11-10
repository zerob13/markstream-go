package markstream

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"strings"
	"testing"
)

const sampleDiff = "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,4 @@\n package main\n-func old() {}\n+func newer() {}\n+// added\n // end\n"

func TestUnifiedDiffParsingAndSplit(t *testing.T) {
	files := ParseDiff(sampleDiff)
	if len(files) != 1 || files[0].NewName != "main.go" {
		t.Fatalf("%+v", files)
	}
	lines := files[0].Lines
	if len(lines) != 6 || lines[2].Kind != DiffRemoved || lines[2].Old != 2 || lines[3].New != 2 || lines[4].New != 3 || lines[5].Old != 3 || lines[5].New != 4 {
		t.Fatalf("%+v", lines)
	}
	rows := diffRows(files, Split, false, 3, nil)
	found := false
	for _, r := range rows {
		if r.Left != nil && r.Right != nil && r.Left.Kind == DiffRemoved && r.Right.Kind == DiffAdded {
			found = true
		}
	}
	if !found {
		t.Fatal("removed/added rows were not paired")
	}
	for i := 0; i <= len(sampleDiff); i++ {
		ParseDiff(sampleDiff[:i])
	}
}
func TestNativeFileAndDiffInteractions(t *testing.T) {
	selected := 0
	tt := ui.NewTester(func(c *ui.Context) {
		File(c, FileData{Name: "main.go", Language: "go", Content: "package main\nfunc main() {}\n"}, FileOptions{OnLineClick: func(e LineEvent) { selected = e.Number }})
	}, 800, 400)
	if err := tt.Click("Copy file"); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != "package main\nfunc main() {}\n" {
		t.Fatal(tt.Clipboard())
	}
	if err := tt.Click("Line 2"); err != nil {
		t.Fatal(err)
	}
	if selected != 2 {
		t.Fatal(selected)
	}
	tt = ui.NewTester(func(c *ui.Context) { FileDiff(c, sampleDiff, FileOptions{Streaming: true}) }, 900, 500)
	for _, s := range []string{"func old() {}", "func newer() {}", "// added"} {
		if !tt.HasText(s) {
			t.Fatal(s, tt.Texts())
		}
	}
	if err := tt.Click("Split"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Unified") {
		t.Fatal("mode did not switch")
	}
	if err := tt.Click("Copy diff"); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != sampleDiff {
		t.Fatal("diff source copy changed")
	}
}
func TestDiffUnchangedRegionExpansion(t *testing.T) {
	source := "--- a/file\n+++ b/file\n@@ -1,20 +1,20 @@\n"
	for i := range 20 {
		source += fmt.Sprintf(" line %d\n", i)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		FileDiff(c, source, FileOptions{Streaming: true, HideUnchanged: true, ContextLines: 2, MaxHeight: 800})
	}, 800, 900)
	if !tt.HasText("Expand 16 unchanged lines") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("Expand 16 unchanged lines"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("line 10") {
		t.Fatal(tt.Texts())
	}
}
func TestFileVirtualizesGrowingContent(t *testing.T) {
	src := strings.Repeat("const value = 42\n", 10000)
	built := 0
	tt := ui.NewTester(func(c *ui.Context) {
		File(c, FileData{Language: "js", Content: src}, FileOptions{Annotation: func(*ui.Context, LineEvent) { built++ }})
	}, 800, 600)
	if built > 100 {
		t.Fatalf("built %d lines", built)
	}
	if !tt.HasText("const value = 42") {
		t.Fatal("missing source")
	}
}

func TestFullNativePatchSurfaceAndLooseDiff(t *testing.T) {
	loose := ParseDiff("-const old = 1\n+const updated = 2\n")
	if len(loose) != 1 || len(loose[0].Lines) != 2 || loose[0].Lines[0].Kind != DiffRemoved || loose[0].Lines[1].Kind != DiffAdded {
		t.Fatalf("loose diff not parsed: %+v", loose)
	}
	var controller *editor.DiffState
	tt := ui.NewTester(func(c *ui.Context) {
		FileDiff(c, sampleDiff, FileOptions{Compare: &CompareOptions{Controller: func(d *editor.DiffState) { controller = d }}})
	}, 1000, 700)
	if controller == nil {
		t.Fatal("native diff controller missing")
	}
	added, deleted := controller.Stats()
	if added != 2 || deleted != 1 {
		t.Fatalf("diff stats=%d/%d", added, deleted)
	}
	if !strings.Contains(controller.Old.String(), "func old()") || !strings.Contains(controller.New.String(), "func newer()") {
		t.Fatal("patch texts lost")
	}
	if err := tt.Click("Split"); err != nil {
		t.Fatal(err)
	}
	if controller.Layout != editor.DiffSplit {
		t.Fatal("native split mode not applied")
	}
	img := tt.Image()
	var nonzero bool
	first := img.RGBAAt(0, 0)
	for y := 0; y < img.Bounds().Dy() && !nonzero; y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y) != first {
				nonzero = true
				break
			}
		}
	}
	if !nonzero {
		t.Fatal("native diff is blank")
	}
}

func TestFinalDiffHandoffKeepsFoldedViewportHeight(t *testing.T) {
	source := "--- a/file\n+++ b/file\n@@ -1,40 +1,40 @@\n" + strings.Repeat(" context\n", 20) + "-old\n+new\n" + strings.Repeat(" context\n", 19)
	streaming := true
	var height float32
	tt := ui.NewTester(func(c *ui.Context) {
		height = FileDiff(c, source, FileOptions{Streaming: streaming, DiffMode: Split, HideUnchanged: true, ContextLines: 2, MaxHeight: 600}).Bounds().H
	}, 900, 900)
	tt.Frame()
	if err := tt.Click("Unified"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	before := height
	streaming = false
	for range 4 {
		tt.Frame()
	}
	if height-before > 1 || before-height > 1 {
		t.Fatalf("final diff changed viewport height: streaming=%g final=%g", before, height)
	}
	if !tt.HasText("Split") {
		t.Fatal("finalization discarded the user's layout choice")
	}
}
