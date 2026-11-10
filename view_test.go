package markstream

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

var instant = &Reveal{Unit: AtOnce, Fade: -1}

func TestNativeMarkdownFeatures(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) { View(c, corpus, Props{}) }, 800, 1800)
	for _, text := range []string{"流式 Markdown", "bold", "italic", "deleted", "ready", "pending", "nested", "Keep old blocks.", "Name", "Value", "42", "fmt.Println(\"你好\")", "footnote", "Copy"} {
		if !strings.Contains(strings.Join(tt.Texts(), "\n"), text) {
			t.Errorf("missing %q in %q", text, tt.Texts())
		}
	}
	if err := tt.Click("Copy"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "fmt.Println(\"你好\")\n" {
		t.Fatal(got)
	}
}
func TestNativeStreamFinalizationAndReset(t *testing.T) {
	s := new(Session)
	s.Append("Hello **world")
	var revealed bool
	tt := ui.NewTester(func(c *ui.Context) { revealed = Revealed(Stream(c, s, Props{Reveal: instant})) }, 600, 400)
	if !tt.HasText("Hello world") {
		t.Fatal(tt.Texts())
	}
	s.Finish()
	tt.Frame()
	if !tt.HasText("Hello **world") || !revealed {
		t.Fatalf("not strictly finalized: %q", tt.Texts())
	}
	s.Reset()
	s.Append("new reply")
	tt.Frame()
	if !tt.HasText("new reply") {
		t.Fatal(tt.Texts())
	}
}
func TestNativeRevealDrainsAfterEOF(t *testing.T) {
	s := new(Session)
	s.Append("The final words must eventually arrive.")
	var shown bool
	tt := ui.NewTester(func(c *ui.Context) { shown = Revealed(Stream(c, s, Props{})) }, 600, 400)
	if shown {
		t.Fatal("revealed immediately")
	}
	s.Finish()
	for range 60 {
		tt.Advance(30 * time.Millisecond)
		tt.Frame()
	}
	if !shown || !tt.HasText("The final words must eventually arrive.") {
		t.Fatalf("shown=%v texts=%q", shown, tt.Texts())
	}
}
func TestCustomFenceWaitsUntilClosed(t *testing.T) {
	source := "```mermaid\ngraph TD\nA --> B\n"
	streaming := true
	calls := 0
	p := Props{Reveal: instant, Code: map[string]func(*ui.Context, *CodeBlock){"mermaid": func(c *ui.Context, b *CodeBlock) { calls++; ui.Text(c, "native diagram") }}}
	tt := ui.NewTester(func(c *ui.Context) { p.Streaming = streaming; View(c, source, p) }, 700, 500)
	if calls != 0 || !tt.HasText("A --> B") {
		t.Fatalf("premature renderer: %d %q", calls, tt.Texts())
	}
	source += "```\n"
	tt.Frame()
	if calls == 0 || !tt.HasText("native diagram") {
		t.Fatal("closed fence not rendered")
	}
	source = "```mermaid\ngraph TD\nA --> B"
	streaming = false
	tt.Frame()
	if !tt.HasText("native diagram") {
		t.Fatal("final unclosed fence not released")
	}
}
func TestSafeCustomLinks(t *testing.T) {
	var opened []string
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, "[good](https://go.dev) [bad](javascript:alert%281%29)", Props{OnLink: func(s string) { opened = append(opened, s) }})
	}, 700, 300)
	if e := tt.Click("good"); e != nil {
		t.Fatal(e)
	}
	if e := tt.Click("bad"); e != nil {
		t.Fatal(e)
	}
	if fmt.Sprint(opened) != "[https://go.dev]" {
		t.Fatal(opened)
	}
}
func TestSafeLink(t *testing.T) {
	for _, s := range []string{"https://go.dev", "http://example.com", "mailto:a@b.com", "#heading", "/settings", "docs/start"} {
		if !SafeLink(s) {
			t.Errorf("rejected %q", s)
		}
	}
	for _, s := range []string{"javascript:alert(1)", "JaVaScRiPt:alert(1)", "data:text/html,hi", "file:///etc/passwd", "https:\\evil.com", "//evil.com", "\nhttps://go.dev", "https://", ""} {
		if SafeLink(s) {
			t.Errorf("accepted %q", s)
		}
	}
}
func TestSubscriptionReleasedAndSessionChanged(t *testing.T) {
	s := new(Session)
	show := true
	tt := ui.NewTester(func(c *ui.Context) {
		if show {
			Stream(c, s, Props{Reveal: instant})
		}
	}, 600, 300)
	if len(s.listeners) != 1 {
		t.Fatal("not subscribed")
	}
	old := s
	s = new(Session)
	tt.Frame()
	if len(old.listeners) != 0 || len(s.listeners) != 1 {
		t.Fatal("old subscription retained")
	}
	show = false
	tt.Frame()
	tt.Frame()
	if len(s.listeners) != 0 {
		t.Fatal("subscription leaked")
	}
}
func TestVirtualizedMessages(t *testing.T) {
	messages := make([]Message, 1000)
	for i := range messages {
		messages[i] = Message{ID: fmt.Sprint(i), Source: fmt.Sprintf("## Reply %d\n\nSome **content**.", i)}
	}
	state := new(ui.ListState)
	builds := 0
	p := Props{Block: func(c *ui.Context, d *Document, i int) bool { builds++; return false }}
	tt := ui.NewTester(func(c *ui.Context) { Messages(c, state, messages, p).Fill() }, 800, 500)
	if !tt.HasText("Reply 0") {
		t.Fatal(tt.Texts())
	}
	if builds >= 1000 {
		t.Fatalf("built %d blocks for 1000 rows", builds)
	}
	state.ScrollTo(999, ui.End)
	tt.Frame()
	if !tt.HasText("Reply 999") {
		t.Fatal("cannot scroll to last row")
	}
}
func BenchmarkNativeSteadyFrame(b *testing.B) {
	source := strings.Repeat(corpus, 5)
	tt := ui.NewTester(func(c *ui.Context) { ui.Scroll(c).Fill().Children(func() { View(c, source, Props{}) }) }, 800, 700)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tt.Frame()
	}
}

func TestStreamingFencePrefixDoesNotFlashThenRestoresAtEOF(t *testing.T) {
	s := new(Session)
	s.Append("```go\nvalue := 42\n")
	tt := ui.NewTester(func(c *ui.Context) { Stream(c, s, Props{Reveal: instant}) }, 800, 400)
	s.Append("`")
	tt.Frame()
	if tt.HasText("`") {
		t.Fatal("partial closing fence flashed as code")
	}
	s.Finish()
	tt.Frame()
	if !tt.HasText("`") {
		t.Fatal("literal incomplete fence not restored at EOF", tt.Texts())
	}
}
