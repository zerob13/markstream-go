package markstream

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// Node is Markstream's renderer-independent tree. MyGo's low-level CommonMark
// parser is only a tokenizer dependency; no MyGo Markdown widget is used.
type Node struct {
	ID, Type, Raw, Content, Language, Info, URL, Title, Label, LocalSource string
	Start, End, Index, Level, Number                                       int
	Loading, Ordered, Header                                               bool
	Task                                                                   int
	Align                                                                  []int
	Children                                                               []*Node
}
type nodeCache struct {
	raw      string
	revision uint32
	node     *Node
}
type nodeBuilder struct {
	cache map[int]nodeCache
	doc   *Document
}

func (b *nodeBuilder) build(d *Document) []*Node {
	b.doc = d
	if b.cache == nil {
		b.cache = map[int]nodeCache{}
	}
	var nodes []*Node
	for i := range d.Children(0) {
		if d.Block(i).Kind == parser.FootnoteDefinition {
			continue
		}
		nodes = append(nodes, b.block(i))
	}
	return projectContainers(nodes)
}
func (b *nodeBuilder) block(i int) *Node {
	d := b.doc
	block := d.Block(i)
	source := d.Source()
	start, end := max(0, block.Start), min(block.End, len(source))
	if end < start {
		end = start
	}
	raw := source[start:end]
	leaf := block.Kind == parser.Paragraph || block.Kind == parser.Heading || block.Kind == parser.TableCell
	var in []parser.Inline
	if leaf {
		in = d.Inlines(i)
	}
	revision := block.Revision()
	var children []*Node
	if !leaf && block.Kind != parser.CodeBlock && block.Kind != parser.HTMLBlock && block.Kind != parser.MathBlock {
		for child := range d.Children(i) {
			children = append(children, b.block(child))
		}
	}
	if previous, ok := b.cache[i]; ok && previous.raw == raw && previous.revision == revision && previous.node.Start == start && previous.node.End == end && previous.node.Loading == (block.Open && d.Streaming()) {
		same := leaf || len(children) == len(previous.node.Children)
		if !leaf && same {
			for index, child := range children {
				if child != previous.node.Children[index] {
					same = false
					break
				}
			}
		}
		if same {
			return previous.node
		}
	}
	if leaf {
		children = b.inlines(in, 0, len(in))
	}
	raw = strings.Clone(raw)
	n := &Node{ID: fmt.Sprintf("%d:%d", block.Start, block.Kind), Type: strings.ReplaceAll(strings.ToLower(block.Kind.String()), " ", "_"), Raw: raw, Start: start, End: end, Index: i, Level: block.Level, Number: block.Number, Ordered: block.Ordered, Loading: block.Open && d.Streaming(), Header: block.Header, Task: int(block.Task), Info: block.Info(), Language: block.Language(), Label: block.Label(), Children: children}
	if block.Alert != parser.NoAlert {
		n.Type = "admonition"
		n.Label = block.Alert.String()
	}
	for _, align := range block.Align {
		n.Align = append(n.Align, int(align))
	}
	switch block.Kind {
	case parser.CodeBlock, parser.HTMLBlock, parser.MathBlock:
		n.Content = d.Lines(i)
		if block.Kind == parser.CodeBlock && block.Fenced {
			n.Content = pendingFenceTail(n.Content, n.Raw, n.Loading)
		}
	case parser.Paragraph:
		if defs := definitionNodes(d.Lines(i), n); defs != nil {
			n.Type = "definition_list"
			n.Children = defs
		}
	}
	b.cache[i] = nodeCache{raw, revision, n}
	return n
}
func (b *nodeBuilder) inlines(in []parser.Inline, from, to int) []*Node {
	var nodes []*Node
	for i := from; i < to; i = in[i].Next {
		item := in[i]
		n := &Node{Type: strings.ReplaceAll(strings.ToLower(item.Kind.String()), " ", "_"), Content: strings.Clone(item.Text), URL: item.Dest, Title: item.Title, Start: item.Start, End: item.End, Loading: item.Pending}
		if item.Next > i+1 {
			n.Children = b.inlines(in, i+1, item.Next)
		}
		if item.Kind == parser.SoftBreak {
			n.Content = " "
		}
		if item.Kind == parser.LineBreak {
			n.Content = "\n"
		}
		if item.Kind == parser.Image {
			n.Label = plainNodes(n.Children)
		}
		nodes = append(nodes, n)
	}
	return nodes
}
func plainNodes(nodes []*Node) string {
	var out strings.Builder
	for _, n := range nodes {
		if len(n.Children) > 0 {
			out.WriteString(plainNodes(n.Children))
		} else {
			out.WriteString(n.Content)
		}
	}
	return out.String()
}
func definitionNodes(raw string, parent *Node) []*Node {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[1], ": ") {
		return nil
	}
	var nodes []*Node
	for i := 0; i < len(lines); {
		term := lines[i]
		i++
		item := &Node{Type: "definition_item", Content: term, Index: parent.Index}
		for i < len(lines) && strings.HasPrefix(lines[i], ": ") {
			doc := Parse(strings.TrimPrefix(lines[i], ": "))
			builder := nodeBuilder{}
			nested := builder.build(doc)
			localizeNodes(nested, doc.Source())
			item.Children = append(item.Children, nested...)
			i++
		}
		if len(item.Children) == 0 {
			return nil
		}
		nodes = append(nodes, item)
	}
	return nodes
}
func projectContainers(input []*Node) []*Node {
	var out []*Node
	for i := 0; i < len(input); i++ {
		n := input[i]
		if n.Type != "paragraph" || !strings.HasPrefix(strings.TrimSpace(n.Raw), ":::") {
			out = append(out, n)
			continue
		}
		lines := strings.Split(strings.TrimSpace(n.Raw), "\n")
		header := strings.TrimSpace(strings.TrimPrefix(lines[0], ":::"))
		if header == "" {
			out = append(out, n)
			continue
		}
		kind, title, _ := strings.Cut(header, " ")
		if title == "" {
			title = kind
		}
		container := &Node{ID: n.ID + ":container", Type: "container", Label: kind, Title: title, Index: n.Index, Start: n.Start, End: n.End, Raw: n.Raw, Loading: n.Loading}
		if len(lines) > 1 {
			body := lines[1:]
			if strings.TrimSpace(body[len(body)-1]) == ":::" {
				body = body[:len(body)-1]
			}
			doc := Parse(strings.Join(body, "\n"))
			builder := nodeBuilder{}
			container.Children = builder.build(doc)
			localizeNodes(container.Children, doc.Source())
		} else {
			start := i + 1
			depth := 0
			for i++; i < len(input); i++ {
				raw := strings.TrimSpace(input[i].Raw)
				if raw == ":::" {
					if depth == 0 {
						break
					}
					depth--
				} else if strings.HasPrefix(raw, ":::") {
					depth++
				}
			}
			end := min(i, len(input))
			container.Children = projectContainers(input[start:end])
			if i < len(input) {
				container.End = input[i].End
			}
		}
		out = append(out, container)
	}
	return out
}

func localizeNodes(nodes []*Node, source string) {
	for _, n := range nodes {
		n.LocalSource = source
		localizeNodes(n.Children, source)
	}
}

func cloneNodes(nodes []*Node) []*Node {
	out := make([]*Node, len(nodes))
	for i, n := range nodes {
		copy := *n
		copy.Children = cloneNodes(n.Children)
		out[i] = &copy
	}
	return out
}
func keyedNodes(nodes []*Node, path string) []*Node {
	out := make([]*Node, len(nodes))
	for i, n := range nodes {
		copy := *n
		if copy.ID == "" {
			copy.ID = fmt.Sprintf("%s.%d", path, i)
		}
		copy.Children = keyedNodes(n.Children, copy.ID)
		out[i] = &copy
	}
	return out
}
