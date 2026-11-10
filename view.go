package markstream

import (
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	md "github.com/egoist/mygo/ui/markdown"
)

type Theme = md.Theme
type CodeBlock = md.CodeBlockView
type InlineObject = md.InlineObject
type Reveal = md.Reveal
type RevealUnit = md.RevealUnit

const (
	ByWord      = md.ByWord
	ByCharacter = md.ByCharacter
	BySentence  = md.BySentence
	ByLine      = md.ByLine
	ByParagraph = md.ByParagraph
	AtOnce      = md.AtOnce
)

// Variant selects typography; colors follow the window's light/dark theme.
type Variant uint8

const (
	DocumentStyle Variant = iota
	ChatStyle
)

// Props configures a native Markdown view. Keep callbacks and Theme stable where
// possible. Custom renderers run on the UI thread and must not block on I/O.
type Props struct {
	Streaming  bool
	Variant    Variant
	Theme      *Theme
	Reveal     *Reveal
	Extensions Extensions
	OnLink     func(string)
	LoadImage  func(string) (ui.ImageSource, error)
	WrapCode   bool
	// PlainCode omits the enhanced language/copy header. Body remains selectable.
	PlainCode bool
	CodeBlock func(*ui.Context, *CodeBlock)
	// Code provides per-language renderers, e.g. mermaid, d2, or infographic.
	// While a fence is open, the default code surface is retained. Finalization
	// also releases an unclosed fence. RenderOpenCode opts into incomplete input.
	Code           map[string]func(*ui.Context, *CodeBlock)
	RenderOpenCode bool
	Math           func(*ui.Context, string)
	InlineMath     func(string, float32) (InlineObject, bool)
	HTML           func(*ui.Context, string)
	Block          func(*ui.Context, *Document, int) bool
	Anchor         string
	AnchorVersion  uint64
	ShareSelection bool
}

type viewState struct {
	props       Props
	options     md.Options
	fallback    Theme
	reveal      Reveal
	session     *Session
	unsubscribe func()
}

func (v *viewState) Close() error {
	if v.unsubscribe != nil {
		v.unsubscribe()
		v.unsubscribe = nil
	}
	return nil
}
func newView() *viewState {
	v := &viewState{reveal: Reveal{Unit: ByCharacter, CatchUp: 120 * time.Millisecond, Fade: 180 * time.Millisecond}}
	v.options.CodeBlock = func(c *ui.Context, b *CodeBlock) {
		p := &v.props
		if f := p.Code[strings.ToLower(b.Language)]; f != nil && (!b.Open || !p.Streaming || p.RenderOpenCode) {
			f(c, b)
			return
		}
		if p.CodeBlock != nil {
			p.CodeBlock(c, b)
			return
		}
		if p.PlainCode {
			b.Body(c)
			return
		}
		md.DefaultCodeBlock(c, b)
	}
	return v
}
func (v *viewState) configure(p Props) {
	v.props = p
	o := &v.options
	o.Theme = p.Theme
	if o.Theme == nil {
		v.fallback = Theme{FontSize: 16, LineHeight: 1.7, CodeFont: "monospace"}
		if p.Variant == ChatStyle {
			v.fallback.FontSize = 14
			v.fallback.LineHeight = 1.55
			v.fallback.BlockSpacing = 0.55
		}
		o.Theme = &v.fallback
	}
	o.Extensions = p.Extensions
	if o.Extensions == 0 {
		o.Extensions = DefaultExtensions
	}
	o.Reveal = p.Reveal
	if o.Reveal == nil {
		o.Reveal = &v.reveal
	}
	// Leave default handling to MyGo when no callback is provided.
	if p.OnLink == nil {
		o.OnLink = nil
	} else {
		o.OnLink = v.openLink
	}
	o.LoadImage = p.LoadImage
	o.WrapCode = p.WrapCode
	o.Math = p.Math
	o.InlineMath = p.InlineMath
	o.HTML = p.HTML
	o.Block = p.Block
	o.Anchor = p.Anchor
	o.AnchorVersion = p.AnchorVersion
	o.ShareSelection = p.ShareSelection
}
func (v *viewState) openLink(dest string) {
	if SafeLink(dest) {
		v.props.OnLink(dest)
	}
}

// SafeLink accepts web/mail links, anchors and relative application routes. It
// rejects executable/local schemes, control characters and backslash ambiguity.
// The same policy protects custom OnLink callbacks; HTML is never executed.
func SafeLink(dest string) bool {
	if strings.ContainsAny(dest, "\\") || strings.IndexFunc(dest, unicode.IsControl) >= 0 {
		return false
	}
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return false
	}
	u, err := url.Parse(dest)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "":
		return u.Host == ""
	case "http", "https":
		return u.Host != ""
	case "mailto":
		return u.Opaque != ""
	}
	return false
}

// View renders Markdown using native MyGo controls, without a browser. Keep it
// at the same keyed position while Streaming changes to false to drain reveal
// animation and preserve selection and completed-block caches.
func View(c *ui.Context, source string, p Props) ui.Element {
	root := ui.Column(c).AlignSelf(ui.Stretch)
	v := ui.Local(root, "markstream", newView)
	(*v).configure(p)
	return (*v).build(c, root, source, p.Streaming, nil)
}
func (v *viewState) build(c *ui.Context, root ui.Element, source string, streaming bool, key any) ui.Element {
	var result ui.Element
	root.Children(func() {
		if key != nil {
			c.Key(key)
		}
		if streaming {
			result = md.Stream(c, source, &v.options)
		} else {
			result = md.View(c, source, &v.options)
		}
	})
	return result
}

// Stream renders a thread-safe session and subscribes to its wake-ups. The
// subscription is removed when MyGo drops the view. Finalized/canceled/failed
// sessions render strictly, while any queued reveal animation finishes.
func Stream(c *ui.Context, s *Session, p Props) ui.Element {
	root := ui.Column(c).AlignSelf(ui.Stretch)
	v := ui.Local(root, "markstream", newView)
	if (*v).session != s {
		(*v).Close()
		(*v).session = s
		if s != nil {
			(*v).unsubscribe = s.Subscribe(c.Services().Invalidate)
		}
	}
	if s == nil {
		p.Streaming = false
		(*v).configure(p)
		return (*v).build(c, root, "", false, nil)
	}
	snapshot := s.Snapshot()
	p.Streaming = !snapshot.Done()
	(*v).configure(p)
	return (*v).build(c, root, snapshot.Source, p.Streaming, struct {
		Session    *Session
		Generation uint64
	}{s, snapshot.Generation})
}

// Revealed reports that the view has displayed and faded in all received text.
// A stream is complete only when its Snapshot.Done() AND Revealed are true.
func Revealed(el ui.Element) bool { return md.Revealed(el) }

// Message is one stable row of a conversation. IDs must be unique and nonempty.
// Session, if nonnil, takes precedence over Source.
type Message struct {
	ID      string
	Source  string
	Session *Session
}

// Messages builds only visible messages (plus native overscan), with variable
// row heights and stable anchors. Set state.FollowEnd=true for chat following;
// scrolling away pauses following. Keep state for the lifetime of the list.
// Virtualization is per message, not per block of one large document.
func Messages(c *ui.Context, state *ui.ListState, messages []Message, p Props) ui.Element {
	state.Key = func(i int) any { return messages[i].ID }
	return ui.List(c, state, len(messages), func(i int) {
		m := messages[i]
		ui.Column(c).AlignSelf(ui.Stretch).Padding(12, 16).Children(func() {
			if m.Session != nil {
				Stream(c, m.Session, p)
			} else {
				View(c, m.Source, p)
			}
		})
	})
}
