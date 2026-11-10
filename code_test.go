package markstream

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestCodeCollapseAndCopy(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, "```go title=main.go\nfmt.Println(42)\n```", Props{CodeOptions: &CodeOptions{Collapsible: true, DefaultCollapsed: true, Filename: FenceFilename, MaxHeight: 180}})
	}, 700, 400)
	if !tt.HasText("main.go") || !tt.HasText("Code collapsed") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("Copy"); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != "fmt.Println(42)\n" {
		t.Fatal(tt.Clipboard())
	}
	if err := tt.Click("Expand"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("fmt.Println(42)") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("Collapse"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Code collapsed") {
		t.Fatal(tt.Texts())
	}
}
func TestFenceFilename(t *testing.T) {
	for _, tc := range []struct{ info, want string }{{"go title=main.go", "main.go"}, {`go filename="hello world.go"`, "hello world.go"}, {"go title='hi.go'", "hi.go"}, {"go", ""}, {"go nottitle=secret", ""}, {"go filename= ", ""}, {`go title="unclosed`, ""}} {
		if got := FenceFilename(&CodeBlock{Info: tc.info}); got != tc.want {
			t.Errorf("%q => %q, want %q", tc.info, got, tc.want)
		}
	}
}
