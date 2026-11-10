package rich

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	ms "github.com/zerob13/markstream-go"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	browser := os.Getenv("MARKSTREAM_CHROME_PATH")
	if browser == "" {
		for _, name := range []string{"chromium", "google-chrome"} {
			if p, e := exec.LookPath(name); e == nil {
				browser = p
				break
			}
		}
	}
	if browser == "" {
		if os.Getenv("MARKSTREAM_REQUIRE_CHROME") == "1" {
			t.Fatal("Chromium is required by this test job; set MARKSTREAM_CHROME_PATH")
		}
		t.Skip("Chromium not installed")
	}
	e := New(Options{BrowserPath: browser, NoSandbox: os.Getenv("MARKSTREAM_CHROME_NO_SANDBOX") == "1"})
	t.Cleanup(func() { e.Close() })
	return e
}
func TestBuiltInRichEngines(t *testing.T) {
	e := testEngine(t)
	cases := []struct{ kind, source, label string }{
		{"math", `\frac{1}{2}+\sqrt{x^2+y^2}`, ""},
		{"inline-math", `x_i^2+\alpha`, ""},
		{"mermaid", "flowchart TD\nA[Start] --> B{Ready?}\nB --> C[Done]", "Start"},
		{"mermaid", "sequenceDiagram\nAlice->>Bob: Hello\nBob-->>Alice: Hi", "Alice"},
		{"d2", "x: Hello\ny: World\nx -> y", "Hello"},
		{"infographic", "infographic list-row-simple-horizontal-arrow\ndata\n  items\n    - label Start\n    - label Finish", "Start"},
		{"html", "<h2>Hello</h2><p><strong>Native</strong> <a href=\"https://go.dev\">Go</a></p><script>window.unsafe=true</script>", "Hello"},
	}
	for _, tc := range cases {
		t.Run(tc.kind+tc.label, func(t *testing.T) {
			out, err := e.Render(context.Background(), ms.RichRequest{Kind: tc.kind, Source: tc.source, Width: 800, FontSize: 20})
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(out.PNG))
			if err != nil || img.Bounds().Dx() < 8 || img.Bounds().Dy() < 8 {
				t.Fatalf("invalid raster: %v", err)
			}
			first := img.At(0, 0)
			rr, gg, bb, aa := first.RGBA()
			different := 0
			for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
				for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
					r, g, b, a := img.At(x, y).RGBA()
					if r != rr || g != gg || b != bb || a != aa {
						different++
					}
				}
			}
			if different < 32 {
				t.Fatal("rendered PNG is blank")
			}
			if dir := os.Getenv("MARKSTREAM_RENDER_ARTIFACTS"); dir != "" {
				os.MkdirAll(dir, 0755)
				os.WriteFile(filepath.Join(dir, tc.kind+"-"+tc.label+".png"), out.PNG, 0644)
			}
			if tc.label != "" && !strings.Contains(out.Plain, tc.label) {
				t.Fatalf("missing label %q in %q", tc.label, out.Plain)
			}
			if tc.kind == "mermaid" || tc.kind == "d2" || tc.kind == "infographic" {
				if !strings.Contains(out.SVG, "<svg") {
					t.Fatal("missing actual SVG")
				}
			}
		})
	}
}
func TestEngineInvalidInputAndClose(t *testing.T) {
	e := testEngine(t)
	_, err := e.Render(context.Background(), ms.RichRequest{Kind: "math", Source: `\notARealCommand{x}`})
	if err == nil {
		t.Fatal("invalid TeX passed")
	}
	if err = e.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = e.Render(context.Background(), ms.RichRequest{Kind: "math", Source: "x"}); err == nil {
		t.Fatal("closed renderer used")
	}
}
