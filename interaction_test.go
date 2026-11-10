package markstream

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

func TestInlineObjectsActuallyPaintInParagraph(t *testing.T) {
	draws := 0
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, "Before $x$ after.", Props{InlineMath: func(string, float32) (InlineObject, bool) {
			return InlineObject{Width: 40, Height: 24, Draw: func(p *ui.Painter, r ui.Rect) { draws++; p.Fill(r, ui.Hex("#ff0000"), 0) }}, true
		}})
	}, 700, 200)
	img := tt.Image()
	red := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r > 60000 && g < 1000 && b < 1000 {
				red++
			}
		}
	}
	if draws == 0 || red < 500 {
		t.Fatalf("inline object was not painted: draws=%d pixels=%d", draws, red)
	}
}
func TestClipboardKeepsOnlySelectedMarkdownAndFormats(t *testing.T) {
	var view *viewState
	ui.NewTester(func(c *ui.Context) {
		el := View(c, "# Unselected heading\n\nHello **bold** and [Go](https://go.dev).\n\nUnselected tail.", Props{})
		view = ui.Local(el, "markstream-owner", func() ownerState { return ownerState{} }).view
	}, 800, 600)
	var id uint64
	for key, region := range view.sources {
		if region.node != nil && strings.Contains(region.node.Raw, "Hello") {
			id = key
		}
	}
	if id == 0 {
		t.Fatal("missing source region")
	}
	data := view.copySelection([]ui.SelectedText{{ID: id, Text: "Hello bold and Go.", Start: 6, End: 10}})
	item := data.Items()[0]
	plain, _ := item.Read(transfer.Text)
	md, _ := item.Read(transfer.Markdown)
	ht, _ := item.Read(transfer.HTML)
	if string(plain) != "bold" || string(md) != "**bold**" || !strings.Contains(string(ht), "<strong>bold</strong>") {
		t.Fatalf("clipboard: %s %s %s", plain, md, ht)
	}
	if strings.Contains(string(md), "Unselected") {
		t.Fatal("unselected content copied")
	}
	data = view.copySelection([]ui.SelectedText{{ID: id, Text: "Hello bold and Go.", Start: 0, End: 18}})
	md, _ = data.Items()[0].Read(transfer.Markdown)
	if string(md) != "Hello **bold** and [Go](https://go.dev)." {
		t.Fatalf("full paragraph did not preserve source: %q", md)
	}
}

func TestCodeSelectionUsesOneFenceAndPreservesNewlines(t *testing.T) {
	var view *viewState
	ui.NewTester(func(c *ui.Context) {
		el := View(c, "```go\nfirst := 1\nsecond := 2\n```", Props{})
		view = ui.Local(el, "markstream-owner", func() ownerState { return ownerState{} }).view
	}, 800, 400)
	var first, second uint64
	for id, region := range view.sources {
		if region.start == 0 {
			first = id
		} else {
			second = id
		}
	}
	md := view.selectedMarkdown([]ui.SelectedText{{ID: first, Text: "first := 1", Start: 0, End: 10}, {ID: second, Text: "second := 2", Start: 0, End: 11}})
	if md != "```go\nfirst := 1\nsecond := 2\n```" {
		t.Fatalf("broken code selection: %q", md)
	}
}
func TestCustomInlineHTMLAndAttributes(t *testing.T) {
	var attrs map[string]string
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, `Before <badge status="ok">**Ready**</badge> after.`, Props{CustomHTML: map[string]func(*ui.Context, *Node){"badge": func(c *ui.Context, n *Node) {
			attrs = n.Attributes
			ui.RichText(c, ui.Span{Text: "Native " + plainNodes(n.Children)})
		}}})
	}, 800, 300)
	if !tt.HasText("Before Native Ready after.") || attrs["status"] != "ok" {
		t.Fatalf("text=%q attrs=%v", tt.Texts(), attrs)
	}
}
func TestFadeVisuallyChangesThenFinalizes(t *testing.T) {
	s := new(Session)
	s.Append("Visible text")
	finals := 0
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(ui.Hex("#ffffff")).Children(func() {
			Stream(c, s, Props{Reveal: &Reveal{Unit: AtOnce, Fade: 200 * time.Millisecond}, OnFinal: func(RenderStatus) { finals++ }})
		})
	}, 600, 200)
	brightness := func() uint64 {
		img := tt.Image()
		sum := uint64(0)
		for y := 0; y < 60; y++ {
			for x := 0; x < 160; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				sum += uint64(r + g + b)
			}
		}
		return sum
	}
	first := brightness()
	s.Finish()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	middle := brightness()
	if finals != 0 || middle >= first {
		t.Fatalf("fade not visible or finished early: %d %d finals=%d", first, middle, finals)
	}
	tt.Advance(150 * time.Millisecond)
	tt.Frame()
	last := brightness()
	if finals != 1 || last >= middle {
		t.Fatalf("fade not drained: %d %d finals=%d", middle, last, finals)
	}
}
func TestRevealRespectsGraphemeClusters(t *testing.T) {
	text := "👨‍👩‍👧‍👦é🇨🇳 done"
	var state revealState
	var shown string
	tt := ui.NewTester(func(c *ui.Context) {
		shown, _, _ = state.step(c, text, true, &Reveal{Unit: ByCharacter, CatchUp: time.Second})
		ui.Text(c, shown)
	}, 800, 200)
	tt.Advance(80 * time.Millisecond)
	tt.Frame()
	if shown != "👨‍👩‍👧‍👦" {
		t.Fatalf("split grapheme: %q", shown)
	}
}
func TestChineseLabelsAndOverride(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, "```go\nvalue := 42\n```", Props{Locale: "zh-CN", Labels: Labels{"Copy": "复制代码"}, CodeOptions: &CodeOptions{Collapsible: true}})
	}, 800, 400)
	if !tt.HasText("复制代码") || !tt.HasText("收起") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("复制代码"); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != "value := 42\n" {
		t.Fatal(tt.Clipboard())
	}
}
