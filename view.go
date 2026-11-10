package markstream

import (
	"net/url"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/kyokomi/emoji/v2"
)

type Variant uint8

const (
	DocumentStyle Variant = iota
	ChatStyle
)

type Props struct {
	Pending int
	Locale  string
	Labels  Labels

	MaxLiveNodes          int
	DisableVirtualization bool

	Nodes          []*Node
	TransformNodes func([]*Node) []*Node

	NodeRenderers map[string]func(*ui.Context, *Node)
	CustomHTML    map[string]func(*ui.Context, *Node)
	OnTaskToggle  func(*Node, bool)

	OnUpdate  func(RenderStatus)
	OnSettled func(RenderStatus)
	OnFinal   func(RenderStatus)

	ParseRevision uint64

	ImageOptions  *ImageOptions
	DisableImages bool

	Emoji             bool
	EmojiAliases      map[string]string
	SourceTransform   func(string) string
	DocumentTransform func(*Document)
	Document          *Document

	Rich           *RichOptions
	MeasurementKey string

	Virtual    *VirtualState
	Streaming  bool
	Variant    Variant
	Theme      *Theme
	Reveal     *Reveal
	Extensions Extensions
	OnLink     func(string)
	LoadImage  func(string) (ui.ImageSource, error)
	WrapCode   bool
	// PlainCode omits the enhanced language/copy header. Body remains selectable.
	PlainCode   bool
	CodeOptions *CodeOptions
	CodeBlock   func(*ui.Context, *CodeBlock)
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
	externalNotes map[string]*Node

	autoVirtual *VirtualState
	shownSource string

	props                                Props
	fallback                             Theme
	services                             ui.Services
	dark                                 bool
	inline                               map[[32]byte]*rasterState
	richOptions                          RichOptions
	richRevision                         atomic.Uint64
	jobs                                 atomic.Int64
	session                              *Session
	unsubscribe                          func()
	generation                           uint64
	contentGeneration                    uint64
	doc                                  *Document
	builder                              nodeBuilder
	nodes                                []*Node
	reveal                               revealState
	receiving, revealing, fading         bool
	source, rawSource, transformedSource string
	transformReady                       bool
	parseRevision                        uint64
	lastParsedStreaming                  bool
	lastHookSource                       string
	settledFired, finalFired             bool
	anchor                               string
	anchorVersion                        uint64
	notes                                map[string]int
	sources                              map[uint64]sourceRegion
	images                               map[string]*imageState
	virtual                              *VirtualState
	tableWidths                          map[string][]float32
}
type sourceRegion struct {
	source     string
	start, end int
	node       *Node
	fragments  []selectionFragment
}
type ownerState struct{ view *viewState }

func newView() *viewState {
	return &viewState{notes: map[string]int{}, sources: map[uint64]sourceRegion{}, tableWidths: map[string][]float32{}}
}
func (v *viewState) Close() error {
	if v.unsubscribe != nil {
		v.unsubscribe()
		v.unsubscribe = nil
	}
	for _, entry := range v.inline {
		entry.Close()
	}
	v.inline = nil
	for _, entry := range v.images {
		entry.Close()
	}
	v.images = nil
	return nil
}
func (v *viewState) reset() {
	v.doc = nil
	v.builder = nodeBuilder{}
	v.reveal = revealState{}
	v.transformReady = false
	v.contentGeneration++
	v.settledFired = false
	v.finalFired = false
	v.tableWidths = map[string][]float32{}
}
func View(c *ui.Context, source string, p Props) ui.Element { return buildView(c, source, p, nil) }
func Stream(c *ui.Context, s *Session, p Props) ui.Element  { return buildView(c, "", p, s) }
func buildView(c *ui.Context, source string, p Props, session *Session) ui.Element {
	root := ui.Column(c).AlignSelf(ui.Stretch)
	v := *ui.Local(root, "markstream", newView)
	if v.session != session {
		if v.unsubscribe != nil {
			v.unsubscribe()
			v.unsubscribe = nil
		}
		v.session = session
		v.reset()
		if session != nil {
			v.unsubscribe = session.Subscribe(c.Services().Invalidate)
		}
	}
	if session != nil {
		snapshot := session.Snapshot()
		source = snapshot.Source
		p.Streaming = !snapshot.Done()
		if snapshot.Generation != v.generation {
			v.generation = snapshot.Generation
			v.reset()
		}
	}
	if p.Document != nil {
		source = p.Document.Source()
	}
	if p.Nodes != nil && source == "" {
		var serialized strings.Builder
		for _, n := range p.Nodes {
			if n.Raw != "" {
				serialized.WriteString(n.Raw)
			} else {
				serialized.WriteString(plainNodes([]*Node{n}))
			}
			serialized.WriteByte('\n')
		}
		source = serialized.String()
	}
	if source != v.source {

		v.source = source
		v.settledFired = false
		v.finalFired = false
	}
	v.props = p
	v.props.Labels = labelsFor(p.Locale, p.Labels)
	v.fallback = resolveTheme(c, p)
	v.services = c.Services()
	v.dark = c.Theme().Dark
	v.receiving = p.Streaming
	if p.Emoji && p.EmojiAliases == nil {
		v.props.EmojiAliases = emoji.CodeMap()
	}
	if p.SourceTransform != nil {
		if !v.transformReady || v.rawSource != source || v.parseRevision != p.ParseRevision {
			v.transformedSource = p.SourceTransform(source)
			v.rawSource = source
			v.transformReady = true
		}
		source = v.transformedSource
	}
	shown, pending, fading := v.reveal.step(c, source, p.Streaming, p.Reveal)
	v.revealing = pending
	v.fading = fading
	showStreaming := p.Streaming || pending
	changed := v.doc == nil || v.lastHookSource != shown || v.lastParsedStreaming != showStreaming || v.parseRevision != p.ParseRevision
	if p.Nodes != nil {
		if v.doc == nil {
			v.doc = NewDocument(p.Extensions)
		}
		v.revealing = false
		v.fading = false
		shown = source
	} else if p.Document != nil {
		if v.doc != p.Document {
			v.builder = nodeBuilder{}
		}
		v.doc = p.Document
		shown = source
		v.revealing = false
		v.fading = false
	} else {
		ext := p.Extensions
		if ext == 0 {
			ext = DefaultExtensions
		}
		if v.doc == nil || v.doc.Extensions() != ext {
			v.doc = NewDocument(ext)
			v.builder = nodeBuilder{}
		}
		v.doc.SetStreaming(showStreaming)
		old := v.doc.Source()
		if shown != old {
			if strings.HasPrefix(shown, old) {
				v.doc.Append(shown[len(old):])
			} else {
				v.doc.Set(shown)
			}
		}
	}
	if changed && p.DocumentTransform != nil {
		p.DocumentTransform(v.doc)
		v.builder.cache = nil
	}
	v.lastHookSource = shown
	v.lastParsedStreaming = showStreaming
	v.parseRevision = p.ParseRevision
	if changed || v.nodes == nil || p.Document != nil {
		v.nodes = v.builder.build(v.doc)
		if p.TransformNodes != nil {
			v.nodes = p.TransformNodes(cloneNodes(v.nodes))
		}
	}
	v.externalNotes = nil
	if p.Nodes != nil {
		v.externalNotes = map[string]*Node{}
		v.nodes = nil
		for _, n := range keyedNodes(p.Nodes, "node") {
			if n.Type == "footnote_definition" || n.Type == "footnote" {
				v.externalNotes[n.Label] = n
			} else {
				v.nodes = append(v.nodes, n)
			}
		}
	}
	limit := p.MaxLiveNodes
	if limit == 0 {
		limit = 220
	}
	if p.Virtual == nil && !p.DisableVirtualization && limit > 0 && len(v.nodes) > limit {
		if v.autoVirtual == nil {
			v.autoVirtual = &VirtualState{}
		}
		p.Virtual = v.autoVirtual
		v.props.Virtual = p.Virtual
		_, h := c.Size()
		root.Height(max(160, h-80))
	}
	v.shownSource = shown
	v.configureRich(c)
	if v.props.LoadImage == nil && !p.DisableImages {
		o := ImageOptions{}
		if p.ImageOptions != nil {
			o = *p.ImageOptions
		}
		var renderer RichRenderer
		if p.Rich != nil {
			renderer = p.Rich.Renderer
		}
		v.props.LoadImage = ImageLoader(o, renderer)
	}
	v.prepareFootnotes()
	if p.Anchor != "" && (p.Anchor != v.anchor || p.AnchorVersion != v.anchorVersion) {
		v.anchor = strings.TrimPrefix(p.Anchor, "#")
		v.anchorVersion = p.AnchorVersion
	}
	if p.Virtual != nil {
		if p.Virtual != v.autoVirtual {
			root.Fill()
		}
		p.Virtual.MeasurementKey = layoutKey(v)
	}
	root.FontSize(v.fallback.FontSize).LineHeight(v.fallback.LineHeight).TextColor(v.fallback.Text)
	if v.fallback.Font != "" {
		root.Font(v.fallback.Font)
	}
	if !p.ShareSelection {
		root.Selectable().CopyData(func(parts []ui.SelectedText) transfer.Data { return v.copySelection(parts) })
		root.ContextMenu(func(m *ui.Menu) {
			m.EditItems()
			m.Separator()
			parts := root.SelectedTexts()
			if m.Item(v.label("Copy Markdown")).Disabled(len(parts) == 0).Chosen() {
				c.Services().WriteClipboard(v.selectedMarkdown(parts))
			}
		})
		if root.Shortcut(ui.Cmd|ui.Shift, ui.KeyC) {
			c.Services().WriteClipboard(v.selectedMarkdown(root.SelectedTexts()))
		}
	}
	v.sources = map[uint64]sourceRegion{}
	root.Children(func() {
		if p.Virtual != nil {
			v.virtual = p.Virtual
			p.Virtual.render(c, v, v.renderNodes())
		} else {
			v.virtual = nil
			for _, n := range v.renderNodes() {
				v.renderNode(c, n)
			}
		}
	})
	ui.Local(root, "markstream-owner", func() ownerState { return ownerState{v} })
	v.progress(source, p.Streaming, root)
	return root
}
func Revealed(el ui.Element) bool {
	owner := ui.Local(el, "markstream-owner", func() ownerState { return ownerState{} })
	if owner.view == nil {
		return false
	}
	v := owner.view
	return !v.revealing && !v.fading && v.jobs.Load()+int64(max(v.props.Pending, 0)) == 0
}
func (v *viewState) progress(source string, receiving bool, el ui.Element) {
	status := RenderStatus{Receiving: receiving, Revealing: v.revealing || v.fading, Pending: int(v.jobs.Load()) + max(v.props.Pending, 0)}
	status.Settled = !status.Revealing && status.Pending == 0
	status.Final = status.Settled && !receiving
	if v.props.Virtual != nil {
		status.Metrics = v.props.Virtual.Metrics()
		if v.props.Virtual.restore != nil {
			status.Pending++
			status.Settled = false
			status.Final = false
		}
	}
	if !status.Settled {
		v.settledFired = false
	}
	if !status.Final {
		v.finalFired = false
	}
	if v.props.OnUpdate != nil {
		v.props.OnUpdate(status)
	}
	if status.Settled && !v.settledFired {
		v.settledFired = true
		if v.props.OnSettled != nil {
			v.props.OnSettled(status)
		}
	}
	if status.Final && !v.finalFired {
		v.finalFired = true
		if v.props.OnFinal != nil {
			v.props.OnFinal(status)
		}
	}
}
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

type Message struct {
	ID, Source string
	Session    *Session
}

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
func (v *viewState) note(el ui.Element, n *Node) {
	source := v.doc.Source()
	if n.LocalSource != "" {
		source = n.LocalSource
	}
	v.sources[el.ID()] = sourceRegion{source: source, start: n.Start, end: n.End, node: n}
}
