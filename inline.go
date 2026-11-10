package markstream

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

type inlineStyle struct {
	size                                  float32
	weight                                int
	color                                 ui.Color
	italic, strike, underline, code, mark bool
	shift                                 float32
}
type inlineEvent struct {
	length int
	tag    string
	close  bool
}
type inlineWriter struct {
	view   *viewState
	root   ui.Element
	source string
	events map[int]inlineEvent
	tags   []string
}

func (v *viewState) paragraph(c *ui.Context, n *Node, style inlineStyle) ui.Element {
	source := v.shownSource
	if n.LocalSource != "" {
		source = n.LocalSource
	}
	el := ui.RichText(c).AlignSelf(ui.Stretch)
	w := inlineWriter{view: v, root: el, source: source, events: extensionEvents(n.Children, source, v.receiving || v.revealing)}
	el.Children(func() { w.nodes(c, n.Children, style) })
	v.note(el, n)
	return el
}
func (w *inlineWriter) styled(s inlineStyle) inlineStyle {
	for _, tag := range w.tags {
		switch tag {
		case "b", "strong":
			s.weight = w.view.fallback.StrongWeight
		case "i", "em":
			s.italic = true
		case "s", "del", "strike":
			s.strike = true
		case "u", "ins":
			s.underline = true
		case "code", "kbd", "samp":
			s.code = true
		case "mark":
			s.mark = true
		case "sub":
			s.size = w.view.fallback.FontSize * .75
			s.shift = -w.view.fallback.FontSize * .2
		case "sup":
			s.size = w.view.fallback.FontSize * .75
			s.shift = w.view.fallback.FontSize * .35
		case "small":
			s.size = w.view.fallback.FontSize * .8
		}
	}
	return s
}
func (w *inlineWriter) text(c *ui.Context, text string, style inlineStyle) {
	style = w.styled(style)
	size := style.size
	if size <= 0 {
		size = w.view.fallback.FontSize
	}
	color := style.color
	if color.A == 0 {
		color = w.view.fallback.Text
	}
	span := ui.Span{Text: text, Size: size, Weight: style.weight, Color: color, Italic: style.italic, Strikethrough: style.strike, Underline: style.underline, BaselineShift: style.shift}
	if style.code {
		span.Font = w.view.fallback.CodeFont
		span.Size *= w.view.fallback.CodeSize
		span.Background = w.view.fallback.CodeBackground
		span.PaddingX = 3
		span.Radius = 3
	}
	if style.mark {
		span.Background = w.view.fallback.Mark
	}
	ui.RichText(c, span)
}
func (w *inlineWriter) nodes(c *ui.Context, nodes []*Node, style inlineStyle) {
	for _, n := range nodes {
		switch n.Type {
		case "text":
			w.extended(c, n, style)
		case "soft_break", "line_break":
			w.text(c, n.Content, style)
		case "emphasis":
			next := style
			next.italic = true
			w.nodes(c, n.Children, next)
		case "strong":
			next := style
			next.weight = w.view.fallback.StrongWeight
			w.nodes(c, n.Children, next)
		case "strikethrough":
			next := style
			next.strike = true
			w.nodes(c, n.Children, next)
		case "code":
			next := style
			next.code = true
			w.text(c, n.Content, next)
		case "link":
			next := style
			next.color = w.view.fallback.Link
			label := plainNodes(n.Children)
			el := ui.RichText(c).Role(ui.RoleLink).Label(label)
			el.Children(func() { w.nodes(c, n.Children, next) })
			if el.Clicked() && !n.Loading && SafeLink(n.URL) {
				if w.view.props.OnLink != nil {
					w.view.props.OnLink(n.URL)
				} else if strings.HasPrefix(n.URL, "#") {
					w.view.anchor = strings.TrimPrefix(n.URL, "#")
					w.view.scrollAnchor(c)
				} else {
					c.Services().OpenURL(n.URL)
				}
			}
		case "image":
			w.view.image(c, n, true)
		case "math":
			size := style.size
			if size <= 0 {
				size = w.view.fallback.FontSize
			}
			if w.view.props.InlineMath != nil {
				if object, ok := w.view.props.InlineMath(n.Content, size); ok {
					w.object(c, object, n.Content)
					continue
				}
			}
			next := style
			next.code = true
			w.text(c, n.Content, next)
		case "html":
			if htmlTag(n.Content) == "br" {
				w.text(c, "\n", style)
			} else if !w.tag(n.Content) {
				w.text(c, n.Content, style)
			}
		case "footnote_reference":
			label := n.Content
			number := w.view.notes[label]
			text := fmt.Sprintf("[%d]", number)
			if number == 0 {
				text = "[" + label + "]"
			}
			next := style
			next.color = w.view.fallback.Link
			next.size = w.view.fallback.FontSize * .75
			next.shift = w.view.fallback.FontSize * .35
			ref := ui.RichText(c).Role(ui.RoleLink).Label(text)
			ref.Children(func() { w.text(c, text, next) })
			if ref.Clicked() {
				w.view.anchor = "footnote:" + label
				w.view.scrollAnchor(c)
			}
		default:
			if len(n.Children) > 0 {
				w.nodes(c, n.Children, style)
			} else {
				w.text(c, n.Content, style)
			}
		}
	}
}
func (w *inlineWriter) object(c *ui.Context, object InlineObject, label string) {
	ui.RichText(c, ui.Span{Text: "\uFFFC", ObjectWidth: object.Width, ObjectHeight: object.Height, ObjectDescent: object.Depth}).Label(label).DrawObjects(func(p *ui.Painter, _ int, r ui.Rect) {
		if object.Draw != nil {
			object.Draw(p, r)
		}
	})
}
func (w *inlineWriter) tag(source string) bool {
	tag := htmlTag(source)
	switch tag {
	case "b", "strong", "i", "em", "s", "del", "strike", "u", "ins", "code", "kbd", "samp", "mark", "sub", "sup", "small":
	default:
		return false
	}
	if strings.HasPrefix(source, "</") {
		for i := len(w.tags) - 1; i >= 0; i-- {
			if w.tags[i] == tag {
				w.tags = append(w.tags[:i], w.tags[i+1:]...)
				break
			}
		}
	} else {
		w.tags = append(w.tags, tag)
	}
	return true
}
func (w *inlineWriter) extended(c *ui.Context, n *Node, style inlineStyle) {
	if n.End-n.Start != len(n.Content) || n.Start < 0 || n.End > len(w.source) || w.source[n.Start:n.End] != n.Content {
		w.text(c, n.Content, style)
		return
	}
	start := 0
	flush := func(end int) {
		if end > start {
			w.text(c, n.Content[start:end], style)
		}
	}
	for i := 0; i < len(n.Content); {
		if event, ok := w.events[n.Start+i]; ok {
			flush(i)
			tag := "<" + event.tag + ">"
			if event.close {
				tag = "</" + event.tag + ">"
			}
			w.tag(tag)
			i += event.length
			start = i
			continue
		}
		if n.Content[i] == ':' && w.view.props.EmojiAliases != nil {
			if count := strings.IndexByte(n.Content[i+1:], ':'); count >= 1 && count <= 48 {
				end := i + count + 2
				if value := w.view.props.EmojiAliases[n.Content[i:end]]; value != "" {
					flush(i)
					w.text(c, value, style)
					i = end
					start = i
					continue
				}
			}
		}
		_, size := utf8.DecodeRuneInString(n.Content[i:])
		i += size
	}
	flush(len(n.Content))
}
func extensionEvents(nodes []*Node, source string, streaming bool) map[int]inlineEvent {
	events := map[int]inlineEvent{}
	type opener struct{ at, end int }
	open := map[string]opener{}
	tags := map[string]string{"==": "mark", "++": "ins", "~": "sub", "^": "sup"}
	var walk func([]*Node)
	walk = func(nodes []*Node) {
		for _, n := range nodes {
			if n.Type != "text" {
				if n.Type != "code" && n.Type != "math" {
					walk(n.Children)
				}
				continue
			}
			if n.End-n.Start != len(n.Content) || n.End > len(source) || source[n.Start:n.End] != n.Content {
				continue
			}
			for i := 0; i < len(n.Content); {
				marker := ""
				for _, m := range []string{"==", "++", "~", "^"} {
					if strings.HasPrefix(n.Content[i:], m) {
						marker = m
						break
					}
				}
				if marker == "" {
					_, size := utf8.DecodeRuneInString(n.Content[i:])
					i += size
					continue
				}
				at := n.Start + i
				end := at + len(marker)
				if marker == "~" && (i+1 < len(n.Content) && n.Content[i+1] == '~' || i > 0 && n.Content[i-1] == '~') {
					i++
					continue
				}
				prev, next := rune(' '), rune(' ')
				if at > 0 {
					prev, _ = utf8.DecodeLastRuneInString(source[:at])
				}
				if end < len(source) {
					next, _ = utf8.DecodeRuneInString(source[end:])
				}
				if start, ok := open[marker]; ok && !unicode.IsSpace(prev) && at > start.end {
					body := source[start.end:at]
					if (marker != "~" && marker != "^" || strings.IndexFunc(body, unicode.IsSpace) < 0) && !strings.Contains(body, "\n\n") {
						events[start.at] = inlineEvent{len(marker), tags[marker], false}
						events[at] = inlineEvent{len(marker), tags[marker], true}
						delete(open, marker)
						i += len(marker)
						continue
					}
				}
				if !unicode.IsSpace(next) && end < len(source) {
					open[marker] = opener{at, end}
				}
				i += len(marker)
			}
		}
	}
	walk(nodes)
	if streaming {
		for marker, entry := range open {
			if entry.end < len(source) {
				events[entry.at] = inlineEvent{len(marker), tags[marker], false}
			}
		}
	}
	return events
}
func (v *viewState) scrollAnchor(c *ui.Context) {
	if v.virtual != nil {
		for index, n := range v.renderNodes() {
			if n.Type == "heading" && slug(plainNodes(n.Children)) == v.anchor || n.Type == "footnotes" && strings.HasPrefix(v.anchor, "footnote:") {
				v.virtual.ScrollTo(index, ui.Start)
				break
			}
		}
	}
	c.Invalidate()
}
