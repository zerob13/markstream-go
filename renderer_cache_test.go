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

func TestIndependentNodeIdentitySurvivesEditsAndPrepend(t *testing.T) {
	parser := NewParser(0)
	nodes := parser.Set("# First\n\nSecond paragraph.\n\n# Last", true)
	lastID := nodes[len(nodes)-1].ID
	edited := parser.Set("# First edited\n\nSecond paragraph.\n\n# Last", true)
	if edited[len(edited)-1].ID != lastID {
		t.Fatal("offset-only edit remounted the last block")
	}
	prepended := parser.Set("Intro.\n\n# First edited\n\nSecond paragraph.\n\n# Last", true)
	if prepended[len(prepended)-1].ID != lastID {
		t.Fatal("prepend lost the last block's stable identity")
	}
}

func TestIndependentParagraphPrependDoesNotStealExistingIdentity(t *testing.T) {
	parser := NewParser(0)
	before := parser.Set("First.\n\nSecond.\n\nThird.", true)
	ids := []string{before[0].ID, before[1].ID, before[2].ID}
	after := parser.Set("Inserted.\n\nFirst.\n\nSecond.\n\nThird.", true)
	for i, id := range ids {
		if after[i+1].ID != id {
			t.Fatalf("paragraph %d lost identity: %s -> %s", i, id, after[i+1].ID)
		}
	}
}

func TestRepeatedParagraphEditsKeepUniqueIDs(t *testing.T) {
	p := NewParser(0)
	p.Set("A.\n\nB.", true)
	nodes := p.Set("B.\n\nB.", true)
	if nodes[0].ID == nodes[1].ID {
		t.Fatal("duplicate nodes share native control identity")
	}
}
