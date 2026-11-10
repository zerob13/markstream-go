package markstream

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

func selectionData(text, markup, source string) transfer.Data {
	return transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte(text)), transfer.Bytes(transfer.HTML, []byte(markup)), transfer.Bytes(transfer.Markdown, []byte(source))))
}
func (v *viewState) renderNode(c *ui.Context, n *Node) ui.Element {
	c.Key(struct {
		Generation uint64
		ID         string
		Index      int
	}{v.contentGeneration, n.ID, n.Index})
	box := ui.Column(c).AlignSelf(ui.Stretch)
	if n.Type != "item" && n.Type != "table_row" {
		box.Margin(0, 0, v.fallback.FontSize*v.fallback.BlockSpacing, 0)
	}
	if n.Type == "heading" && slug(plainNodes(n.Children)) == v.anchor {
		box.ScrollIntoView()
		v.anchor = ""
	}
	box.Children(func() {
		if v.props.Block != nil && n.Index >= 0 && v.props.Block(c, v.doc, n.Index) {
			return
		}
		if renderer := v.props.NodeRenderers[n.Type]; renderer != nil {
			renderer(c, n)
			return
		}
		switch n.Type {
		case "paragraph", "table_cell":
			if onlyImages(n.Children) {
				for _, image := range n.Children {
					if image.Type == "image" {
						v.image(c, image, false)
					}
				}
			} else {
				v.paragraph(c, n, inlineStyle{})
			}
		case "heading":
			level := min(6, max(1, n.Level))
			h := v.fallback.Headings[level-1]
			v.paragraph(c, n, inlineStyle{size: v.fallback.FontSize * h.Size, weight: h.Weight, color: h.Color}).Role(ui.RoleHeading).Level(level)
		case "code_block":
			v.code(c, n)
		case "html_block":
			if renderer := v.props.CustomHTML[htmlTag(n.Content)]; renderer != nil {
				renderer(c, n)
			} else if v.props.HTML != nil {
				v.props.HTML(c, n.Content)
			} else {
				ui.Text(c, n.Content).Font(v.fallback.CodeFont).Selectable()
			}
		case "math_block":
			if v.props.Math != nil {
				v.props.Math(c, strings.TrimSpace(n.Content))
			} else {
				ui.Text(c, strings.TrimSpace(n.Content)).Font(v.fallback.CodeFont)
			}
		case "thematic_break":
			ui.Box(c).Height(1).AlignSelf(ui.Stretch).Background(v.fallback.Rule)
		case "quote", "admonition", "container":
			color := v.fallback.QuoteBar
			title := n.Label
			if n.Type == "container" {
				title = n.Title
			}
			key := strings.ToLower(n.Label)
			for index, name := range []string{"note", "tip", "important", "warning", "caution"} {
				if key == name {
					color = v.fallback.Alerts[index]
				}
			}
			ui.Column(c).BorderStartWidth(v.fallback.QuoteBarWidth).BorderColor(color).Padding(8, 14).AlignSelf(ui.Stretch).Children(func() {
				opened := ui.Local(box, "details", func() bool { return key != "details" })
				if key == "details" {
					if ui.Button(c, title).Clicked() {
						*opened = !*opened
					}
					if !*opened {
						return
					}
				} else if n.Type != "quote" && title != "" {
					ui.Text(c, title).Bold().TextColor(color)
				}
				for _, child := range n.Children {
					v.renderNode(c, child)
				}
			})
		case "list":
			for index, item := range n.Children {
				ui.Row(c).Gap(v.fallback.FontSize * .5).AlignItems(ui.Start).Children(func() {
					if item.Task > 0 {
						checked := item.Task == 2
						ui.Checkbox(c, &checked, "").Disabled(v.props.OnTaskToggle == nil)
						if v.props.OnTaskToggle != nil && checked != (item.Task == 2) {
							v.props.OnTaskToggle(item, checked)
						}
					} else {
						marker := "•"
						if n.Ordered {
							marker = fmt.Sprintf("%d.", max(n.Number, 1)+index)
						}
						ui.Text(c, marker).MinWidth(v.fallback.FontSize).Unselectable()
					}
					ui.Column(c).FillWidth().Children(func() {
						for _, child := range item.Children {
							v.renderNode(c, child)
						}
					})
				})
			}
		case "item", "footnote_definition":
			for _, child := range n.Children {
				v.renderNode(c, child)
			}
		case "table":
			v.table(c, n)
		case "definition_list":
			for _, item := range n.Children {
				ui.Text(c, item.Content).Bold()
				ui.Column(c).PaddingStart(v.fallback.FontSize * 1.5).Children(func() {
					for _, child := range item.Children {
						v.renderNode(c, child)
					}
				})
			}
		case "footnotes":
			ui.Box(c).Height(1).Background(v.fallback.Rule).AlignSelf(ui.Stretch)
			for _, footnote := range n.Children {
				ui.Row(c).Gap(8).Children(func() {
					label := fmt.Sprintf("[%d]", v.notes[footnote.Label])
					ui.Text(c, label).TextColor(v.fallback.Link)
					ui.Column(c).FillWidth().Children(func() {
						if v.anchor == "footnote:"+footnote.Label {
							box.ScrollIntoView()
							v.anchor = ""
						}
						for _, child := range footnote.Children {
							v.renderNode(c, child)
						}
					})
				})
			}
		default:
			if len(n.Children) > 0 {
				for _, child := range n.Children {
					v.renderNode(c, child)
				}
			} else if n.Content != "" {
				ui.Text(c, n.Content)
			}
		}
	})
	return box
}
func (v *viewState) table(c *ui.Context, n *Node) {
	columns := len(n.Align)
	for _, row := range n.Children {
		columns = max(columns, len(row.Children))
	}
	if columns == 0 {
		return
	}
	widths := make([]float32, columns)
	if n.Loading {
		copy(widths, v.tableWidths[n.ID])
	}
	for _, row := range n.Children {
		for column, cell := range row.Children {
			w, _ := c.MeasureText(0, ui.Span{Text: plainNodes(cell.Children), Size: v.fallback.FontSize})
			widths[column] = max(widths[column], min(w, v.fallback.FontSize*v.fallback.TableMaxColumn)+v.fallback.FontSize*1.2+1)
		}
	}
	v.tableWidths[n.ID] = widths
	ui.ScrollHorizontal(c).AlignSelf(ui.Stretch).Children(func() {
		ui.Column(c).Border(1, v.fallback.TableBorder).Role(ui.RoleTable).Children(func() {
			for rowIndex, row := range n.Children {
				el := ui.Row(c).Role(ui.RoleRow).AlignItems(ui.Stretch)
				if row.Header {
					el.Background(v.fallback.TableHeader)
				} else if rowIndex%2 == 1 {
					el.Background(v.fallback.TableStripe)
				}
				el.Children(func() {
					for column, cell := range row.Children {
						role := ui.RoleCell
						weight := 0
						if row.Header {
							role = ui.RoleColumnHeader
							weight = 600
						}
						cellBox := ui.Column(c).Width(widths[column]).Padding(v.fallback.FontSize*.4, v.fallback.FontSize*.6).BorderWidth(0, 1, 1, 0).BorderColor(v.fallback.TableBorder).Role(role).Label(plainNodes(cell.Children))
						cellBox.Children(func() {
							text := v.paragraph(c, cell, inlineStyle{weight: weight})
							if column < len(n.Align) {
								switch n.Align[column] {
								case 2:
									text.TextAlign(ui.Center)
								case 3:
									text.TextAlign(ui.End)
								}
							}
						})
					}
				})
			}
		})
	})
}
func (v *viewState) code(c *ui.Context, n *Node) {
	b := &CodeBlock{Language: n.Language, Info: n.Info, Open: n.Loading, Index: n.Index, node: n, owner: v}
	p := v.props
	if renderer := p.Code[strings.ToLower(n.Language)]; renderer != nil && (!b.Open || p.RenderOpenCode) {
		renderer(c, b)
		return
	}
	if p.CodeBlock != nil {
		p.CodeBlock(c, b)
		return
	}
	if p.Rich != nil && p.Rich.Renderer != nil {
		if kind := richLanguage(b.Language); kind != "" && (!b.Open || p.Rich.Progressive) {
			richView(c, v.richRequest(kind, b.Code()), *p.Rich, func() { b.Body(c) })
			return
		}
	}
	if p.PlainCode {
		b.Body(c)
		return
	}
	options := CodeOptions{}
	if p.CodeOptions != nil {
		options = *p.CodeOptions
	}
	RenderCode(c, b, options)
}

type CodeBlock struct {
	Language, Info string
	Open           bool
	Index          int
	node           *Node
	owner          *viewState
}

func (b *CodeBlock) Code() string {
	if b.node == nil {
		return ""
	}
	return b.node.Content
}
func (b *CodeBlock) Body(c *ui.Context) ui.Element {
	o := FileOptions{}
	if b.owner != nil {
		o.Wrap = b.owner.props.WrapCode
		o.HideLineNumbers = b.owner.props.PlainCode
	}
	return fileBody(c, FileData{Language: b.Language, Content: b.Code(), Streaming: b.Open}, o, b.NoteElement)
}
func (b *CodeBlock) NoteElement(el ui.Element, line int) {
	if b.owner != nil && b.node != nil {
		b.owner.note(el, b.node)
	}
}
func (b *CodeBlock) CopyButton(c *ui.Context) ui.Element {
	button := ui.Button(c, "Copy").Disabled(b.Open)
	copied := ui.Local(button, "copied", func() time.Time { return time.Time{} })
	if button.Clicked() {
		c.Services().WriteClipboard(b.Code())
		*copied = c.Now()
		c.After(2 * time.Second)
	}
	if !copied.IsZero() && c.Now().Sub(*copied) < 2*time.Second {
		ui.Text(c, "Copied").FontSize(11)
	}
	return button
}
func slug(text string) string {
	var out strings.Builder
	dash := false
	for _, r := range strings.ToLower(text) {
		if r == ' ' || r == '\n' || r == '\t' {
			dash = true
			continue
		}
		if dash && out.Len() > 0 {
			out.WriteByte('-')
		}
		dash = false
		if r != '#' && r != '*' && r != '`' {
			out.WriteRune(r)
		}
	}
	return out.String()
}
func htmlTag(source string) string {
	source = strings.TrimSpace(source)
	source = strings.TrimPrefix(source, "<")
	source = strings.TrimPrefix(source, "/")
	if end := strings.IndexAny(source, " >\t\r\n/"); end >= 0 {
		source = source[:end]
	}
	return strings.ToLower(source)
}
func onlyImages(nodes []*Node) bool {
	found := false
	for _, n := range nodes {
		if n.Type == "image" {
			found = true
		} else if n.Type != "soft_break" && n.Type != "line_break" && strings.TrimSpace(n.Content) != "" {
			return false
		}
	}
	return found
}
func (v *viewState) prepareFootnotes() {
	v.notes = map[string]int{}
	var walk func([]*Node)
	walk = func(nodes []*Node) {
		for _, n := range nodes {
			if n.Type == "footnote_reference" {
				if _, ok := v.notes[n.Content]; !ok {
					v.notes[n.Content] = len(v.notes) + 1
				}
			}
			walk(n.Children)
		}
	}
	walk(v.nodes)
}
func (v *viewState) renderNodes() []*Node {
	if len(v.notes) == 0 {
		return v.nodes
	}
	footer := &Node{ID: "footnotes", Type: "footnotes", Index: -1}
	defs := make([]*Node, len(v.notes))
	for label, number := range v.notes {
		if n := v.externalNotes[label]; n != nil {
			defs[number-1] = n
			continue
		}
		if i, ok := v.doc.Footnote(label); ok {
			node := v.builder.block(i)
			defs[number-1] = node
		}
	}
	for _, node := range defs {
		if node != nil {
			footer.Children = append(footer.Children, node)
		}
	}
	if len(footer.Children) == 0 {
		return v.nodes
	}
	nodes := append([]*Node(nil), v.nodes...)
	return append(nodes, footer)
}
