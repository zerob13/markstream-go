package markstream

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
)

func TestIndependentNodeCacheTracksLateNestedReferences(t *testing.T) {
	doc := NewDocument(0)
	doc.SetStreaming(true)
	doc.Append("- [Go][ref]\n\n")
	builder := nodeBuilder{}
	before := builder.build(doc)
	first := before[0]
	if got := builder.build(doc)[0]; got != first {
		t.Fatal("unchanged completed node not reused")
	}
	doc.Append("[ref]: https://go.dev\n")
	after := builder.build(doc)
	var found bool
	var walk func([]*Node)
	walk = func(nodes []*Node) {
		for _, n := range nodes {
			if n.Type == "link" && n.URL == "https://go.dev" {
				found = true
			}
			walk(n.Children)
		}
	}
	walk(after)
	if !found {
		t.Fatal("nested cached reference stayed unresolved")
	}
}
func TestIndependentPreparsedNodesAndRegistry(t *testing.T) {
	nodes := []*Node{{Type: "heading", Level: 1, Children: []*Node{{Type: "text", Content: "Own tree"}}}, {Type: "custom", Content: "Own component"}}
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, "", Props{Nodes: nodes, NodeRenderers: map[string]func(*ui.Context, *Node){"custom": func(c *ui.Context, n *Node) { ui.Text(c, n.Content) }}})
	}, 700, 400)
	if !tt.HasText("Own tree") || !tt.HasText("Own component") {
		t.Fatal(tt.Texts())
	}
}
func TestIndependentRendererKeepsRichMarkdownInsideDefinitions(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) { View(c, "Term\n: A ==highlight== and **definition**", Props{}) }, 700, 400)
	text := strings.Join(tt.Texts(), " ")
	if strings.Contains(text, "==") || !strings.Contains(text, "highlight") {
		t.Fatal(text)
	}
}
