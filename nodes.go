package markstream

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/html"
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// Node is Markstream's renderer-independent tree. MyGo's low-level CommonMark
// parser is only a tokenizer dependency; no MyGo Markdown widget is used.
type Node struct {
	ID, Type, Raw, Content, Language, Info, URL, Title, Label, LocalSource string
	Attributes                                                             map[string]string
	Start, End, Index, Level, Number                                       int
	Loading, Ordered, Header                                               bool
	Task                                                                   int
	Align                                                                  []int
	Children                                                               []*Node
}
type nodeCache struct {
	kind parser.Kind

	raw      string
	revision uint32
	node     *Node
}
type nodeBuilder struct {
	incoming map[string]int

	serial   uint64
	used     map[string]bool
	previous map[string][]*Node

	cache map[int]nodeCache
	doc   *Document
}

func (b *nodeBuilder) build(d *Document) []*Node {
	b.doc = d
	b.used = map[string]bool{}
	b.incoming = map[string]int{}
	src := d.Source()
	for i := 1; i < d.Len(); i++ {
		block := d.Block(i)
		start, end := max(0, block.Start), min(len(src), block.End)
		if end >= start {
			key := fmt.Sprintf("%d:%s", block.Kind, contentHash(src[start:end]))
			b.incoming[key]++
		}
	}
	b.previous = map[string][]*Node{}
	for i, entry := range b.cache {
		if i >= d.Len() {
			delete(b.cache, i)
			continue
		}
		key := fmt.Sprintf("%d:%s", entry.kind, contentHash(entry.raw))
		b.previous[key] = append(b.previous[key], entry.node)
	}
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
	if previous, ok := b.cache[i]; ok && !b.used[previous.node.ID] && previous.raw == raw && previous.revision == revision && previous.node.Start == start && previous.node.End == end && previous.node.Loading == (block.Open && d.Streaming()) {
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
			b.used[previous.node.ID] = true
			return previous.node
		}
	}
	if leaf {
		children = b.inlines(in, 0, len(in))
	}
	raw = strings.Clone(raw)
	id := ""
	key := fmt.Sprintf("%d:%s", block.Kind, contentHash(raw))
	for _, old := range b.previous[key] {
		if !b.used[old.ID] && old.Raw == raw {
			id = old.ID
			break
		}
	}
	if id == "" {
		if old, ok := b.cache[i]; ok && !b.used[old.node.ID] && old.kind == block.Kind {
			oldKey := fmt.Sprintf("%d:%s", old.kind, contentHash(old.raw))
			if b.incoming[oldKey] == 0 {
				id = old.node.ID
			}
		}
	}
	if id == "" {
		b.serial++
		id = fmt.Sprintf("node:%d", b.serial)
	}
	b.used[id] = true
	n := &Node{ID: id, Type: strings.ReplaceAll(strings.ToLower(block.Kind.String()), " ", "_"), Raw: raw, Start: start, End: end, Index: i, Level: block.Level, Number: block.Number, Ordered: block.Ordered, Loading: block.Open && d.Streaming(), Header: block.Header, Task: int(block.Task), Info: block.Info(), Language: block.Language(), Label: block.Label(), Children: children}
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
		if len(n.Children) == 1 && n.Children[0].Type == "html_inline" && !inlineHTMLTag(n.Children[0].Label) {
			child := n.Children[0]
			n.Type = "html_block"
			n.Content = n.Raw
			n.Attributes = child.Attributes
		}
		if defs := definitionNodes(d.Lines(i), n); defs != nil {
			n.Type = "definition_list"
			n.Children = defs
		}
	}
	if n.Type == "html_block" {
		n.Attributes = htmlAttributes(n.Content)
	}
	b.cache[i] = nodeCache{kind: block.Kind, raw: raw, revision: revision, node: n}
	return n
}

func inlineHTMLTag(tag string) bool {
	switch tag {
	case "span", "b", "strong", "i", "em", "s", "del", "strike", "u", "ins", "code", "kbd", "samp", "mark", "sub", "sup", "small", "a":
		return true
	}
	return false
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
	return groupInlineHTML(nodes)
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
		attrs := map[string]string{}
		if at := strings.IndexByte(header, '{'); at >= 0 {
			var values map[string]any
			if json.Unmarshal([]byte(header[at:]), &values) == nil {
				for key, value := range values {
					attrs[strings.TrimPrefix(key, "data-")] = fmt.Sprint(value)
				}
			} else {
				attrs["attrs"] = header[at:]
			}
		}
		if args := strings.TrimSpace(strings.TrimPrefix(header, kind)); args != "" {
			attrs["args"] = args
		}
		container := &Node{Attributes: attrs, ID: n.ID + ":container", Type: "container", Label: kind, Title: title, Index: n.Index, Start: n.Start, End: n.End, Raw: n.Raw, Loading: n.Loading}
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
		if n.Attributes != nil {
			copy.Attributes = map[string]string{}
			for k, v := range n.Attributes {
				copy.Attributes[k] = v
			}
		}
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

func htmlAttributes(source string) map[string]string {
	z := html.NewTokenizer(strings.NewReader(source))
	kind := z.Next()
	if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
		return nil
	}
	attrs := map[string]string{}
	for _, attr := range z.Token().Attr {
		attrs[attr.Key] = attr.Val
	}
	return attrs
}
func groupInlineHTML(nodes []*Node) []*Node {
	var out []*Node
	for i := 0; i < len(nodes); i++ {
		n := nodes[i]
		if n.Type != "html" || strings.HasPrefix(n.Content, "</") || strings.HasSuffix(n.Content, "/>") {
			out = append(out, n)
			continue
		}
		tag := htmlTag(n.Content)
		end := i + 1
		depth := 1
		for ; end < len(nodes); end++ {
			item := nodes[end]
			if item.Type == "html" && htmlTag(item.Content) == tag {
				if strings.HasPrefix(item.Content, "</") {
					depth--
				} else {
					depth++
				}
				if depth == 0 {
					break
				}
			}
		}
		if end == len(nodes) {
			out = append(out, n)
			continue
		}
		group := *n
		group.Type = "html_inline"
		group.Label = tag
		group.Attributes = htmlAttributes(n.Content)
		group.Children = groupInlineHTML(nodes[i+1 : end])
		group.End = nodes[end].End
		var raw strings.Builder
		for _, part := range nodes[i : end+1] {
			if part.Type == "html" {
				raw.WriteString(part.Content)
			} else {
				raw.WriteString(plainNodes([]*Node{part}))
			}
		}
		group.Content = raw.String()
		out = append(out, &group)
		i = end
	}
	return out
}
