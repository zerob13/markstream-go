package rich

import (
	"bytes"
	"context"
	"encoding/xml"
	"image/png"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/egoist/mygo/ui"
	ms "github.com/zerob13/markstream-go"
)

func TestIndependentNativeRichViewCompletionAndControls(t *testing.T) {
	engine := testEngine(t)
	chart := "flowchart LR\nA[Start]-->B[Done]\n"
	source := "Inline $x_i^2$\n\n```mermaid\n" + chart + "```\n"
	finals := 0
	pendingSeen := false
	var exports []ms.Export
	var final ms.RenderStatus
	p := ms.Props{Rich: &ms.RichOptions{Renderer: engine, Width: 600, OnExport: func(e ms.Export) { exports = append(exports, e) }}, OnUpdate: func(s ms.RenderStatus) {
		if s.Pending > 0 {
			pendingSeen = true
		}
		final = s
	}, OnFinal: func(ms.RenderStatus) { finals++ }}
	tt := ui.NewTester(func(c *ui.Context) { ms.View(c, source, p) }, 1000, 800)
	until := time.Now().Add(20 * time.Second)
	for time.Now().Before(until) && !final.Final {
		tt.Advance(20 * time.Millisecond)
		tt.Frame()
		time.Sleep(5 * time.Millisecond)
	}
	if !final.Final || !pendingSeen || finals != 1 {
		t.Fatalf("premature/missing completion: %+v finals=%d pendingSeen=%v", final, finals, pendingSeen)
	}
	if err := tt.Click("SVG"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("PNG"); err != nil {
		t.Fatal(err)
	}
	if len(exports) != 2 || exports[0].Source != chart {
		t.Fatalf("exports=%+v", exports)
	}
	var root struct{ XMLName xml.Name }
	if err := xml.Unmarshal(exports[0].Data, &root); err != nil || root.XMLName.Local != "svg" {
		t.Fatalf("invalid SVG: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(exports[1].Data)); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Source"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("flowchart LR") {
		t.Fatal("source tab did not expose actual source")
	}
	if err := tt.Click("Preview"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Fullscreen"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Close preview") {
		t.Fatal("native fullscreen modal did not open")
	}
	if err := tt.Click("Close preview"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if finals != 1 {
		t.Fatalf("completion repeated without a new render: %d", finals)
	}
}
func TestWorkerSanitizesHTMLAndRejectsCanceledWork(t *testing.T) {
	engine := testEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := engine.Render(ctx, ms.RichRequest{Kind: "math", Source: "x"}); err == nil {
		t.Fatal("canceled request ran")
	}
	out, err := engine.Render(context.Background(), ms.RichRequest{Kind: "html", Source: `<h2>Safe</h2><script>window.markstreamInjected=true</script><img src="x" onerror="window.markstreamInjected=true"><a href="javascript:alert(1)">Unsafe</a><iframe srcdoc="<script>window.markstreamInjected=true</script>"></iframe>`})
	if err != nil {
		t.Fatal(err)
	}
	var injected bool
	if err = chromedp.Run(engine.tab, chromedp.Evaluate("window.markstreamInjected === true", &injected)); err != nil {
		t.Fatal(err)
	}
	if injected {
		t.Fatal("untrusted HTML executed in renderer")
	}
	for _, link := range out.Links {
		if !ms.SafeLink(link.URL) {
			t.Fatalf("unsafe link survived: %q", link.URL)
		}
	}
}
func TestMermaidDiagramFamilies(t *testing.T) {
	e := testEngine(t)
	fixtures := map[string]string{
		"class":    "classDiagram\nAnimal <|-- Dog\nAnimal : +name string\nDog : +bark()",
		"state":    "stateDiagram-v2\n[*] --> Ready\nReady --> Done\nDone --> [*]",
		"er":       "erDiagram\nCUSTOMER ||--o{ ORDER : places",
		"gantt":    "gantt\n dateFormat YYYY-MM-DD\n section Build\n Parse :a, 2026-01-01, 3d\n Render :after a, 2d",
		"pie":      "pie title Shares\n \"A\" : 3\n \"B\" : 7",
		"git":      "gitGraph\ncommit\nbranch feature\ncheckout feature\ncommit\ncheckout main\nmerge feature",
		"mindmap":  "mindmap\n  root((Markstream))\n    Parser\n    Renderer",
		"timeline": "timeline\n title Releases\n 2025 : Parser\n 2026 : Renderer",
		"xy":       "xychart-beta\n x-axis [A, B]\n y-axis \"Value\" 0 --> 10\n bar [3,7]",
		"sankey":   "sankey-beta\n\nA,B,10\nB,C,10",
		"block":    "block-beta\n columns 3\n A B C",
		"journey":  "journey\n title Journey\n section Use\n Parse: 5: User\n Render: 5: User",
	}
	for name, source := range fixtures {
		t.Run(name, func(t *testing.T) {
			out, err := e.Render(context.Background(), ms.RichRequest{Kind: "mermaid", Source: source, Width: 700})
			if err != nil {
				t.Fatal(err)
			}
			if len(out.PNG) < 200 || out.SVG == "" {
				t.Fatal("missing real diagram output")
			}
		})
	}
}

func TestEngineConfigurationAndImmutableCache(t *testing.T) {
	engine := testEngine(t)
	request := ms.RichRequest{Kind: "math", Source: `\RR^2`, Config: map[string]any{"macros": map[string]string{`\RR`: `\mathbb{R}`}}}
	out, err := engine.Render(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	original := append([]byte(nil), out.PNG...)
	out.PNG[0] = 0
	cached, err := engine.Render(context.Background(), request)
	if err != nil || !bytes.Equal(original, cached.PNG) {
		t.Fatal("caller corrupted cached graphic", err)
	}
	if _, err := engine.Render(context.Background(), ms.RichRequest{Kind: "math", Source: "x", Config: map[string]any{"invalid": func() {}}}); err == nil {
		t.Fatal("invalid config not rejected")
	}
}

func TestProgressiveDiagramKeepsLastValidPreviewAndRecovers(t *testing.T) {
	engine := testEngine(t)
	valid := "flowchart LR\nA[First]-->B[Valid]\n"
	source := "```mermaid\n" + valid
	streaming := true
	completed := 0
	failed := false
	var exports []ms.Export
	p := ms.Props{Reveal: &ms.Reveal{Unit: ms.AtOnce}, Rich: &ms.RichOptions{Renderer: engine, Width: 600, Progressive: true, OnExport: func(e ms.Export) { exports = append(exports, e) }, OnRender: func(s ms.RichRenderState) { completed++; failed = s.Err != nil }}}
	tt := ui.NewTester(func(c *ui.Context) { p.Streaming = streaming; ms.View(c, source, p) }, 900, 700)
	waitRender := func(previous int) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for completed <= previous && time.Now().Before(deadline) {
			tt.Frame()
			time.Sleep(5 * time.Millisecond)
		}
		if completed <= previous {
			t.Fatal("diagram update did not complete")
		}
	}
	waitRender(0)
	if failed {
		t.Fatal("valid preview failed")
	}
	previous := completed
	source += "B --> C["
	tt.Frame()
	waitRender(previous)
	if !failed {
		t.Fatal("invalid prefix was not tested")
	}
	if err := tt.Click("SVG"); err != nil {
		t.Fatal("last valid preview disappeared", err)
	}
	if len(exports) != 1 || exports[0].Source != valid {
		t.Fatal("export did not match retained preview", exports)
	}
	previous = completed
	source += "Recovered]\n```\n"
	streaming = false
	tt.Frame()
	waitRender(previous)
	if failed {
		t.Fatal("final valid diagram did not recover")
	}
	if err := tt.Click("SVG"); err != nil {
		t.Fatal(err)
	}
	if len(exports) != 2 || !bytes.Contains(exports[1].Data, []byte("Recovered")) {
		t.Fatal("recovered output not exported")
	}
}
