package markstream

// Parser owns the incremental low-level syntax document and Markstream node
// cache. It is single-goroutine state; returned nodes are immutable snapshots
// and may be handed to the UI through Props.Nodes without using a Markdown widget.
type Parser struct {
	document *Document
	builder  nodeBuilder
}

func NewParser(extensions Extensions) *Parser { return &Parser{document: NewDocument(extensions)} }
func (p *Parser) Set(source string, final bool) []*Node {
	p.document.SetStreaming(!final)
	p.document.Set(source)
	return p.snapshot()
}
func (p *Parser) Append(chunk string, final bool) []*Node {
	p.document.SetStreaming(!final)
	p.document.Append(chunk)
	return p.snapshot()
}
func (p *Parser) snapshot() []*Node {
	nodes := p.builder.build(p.document)
	for i := range p.document.Children(0) {
		if p.document.Block(i).Kind.String() == "footnote definition" {
			nodes = append(nodes, p.builder.block(i))
		}
	}
	return nodes
}
func ParseNodes(source string) []*Node { return NewParser(0).Set(source, true) }

func (p *Parser) Source() string { return p.document.Source() }
