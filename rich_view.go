package markstream

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/egoist/mygo"
	"image"
	_ "image/png"
	"os"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

type rasterState struct {
	mu                     sync.Mutex
	busy                   bool
	closed                 bool
	cancel                 context.CancelFunc
	request                RichRequest
	result                 RichResult
	err                    error
	version                uint64
	painted                uint64
	bitmap                 *ui.Bitmap
	showSource, fullscreen bool
	zoom                   float32
	scroll                 ui.ScrollState
	emitted                uint64
	pan                    bool
	panX, panY             float32
}

func (s *rasterState) Close() error {
	s.mu.Lock()
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	return nil
}
func requestKey(r RichRequest) [32]byte { data, _ := json.Marshal(r); return sha256.Sum256(data) }
func (s *rasterState) load(r RichRequest, renderer RichRenderer, wake func(), onWork func(int)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.busy || requestKey(r) == requestKey(s.request) {
		return
	}

	if s.request.Kind != r.Kind || s.request.Dark != r.Dark || !strings.HasPrefix(r.Source, s.request.Source) {
		s.result = RichResult{}
		s.bitmap = nil
	}
	s.request = r
	s.busy = true
	s.err = nil
	if onWork != nil {
		onWork(1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go func() {
		result, err := renderer.Render(ctx, r)
		s.mu.Lock()
		if !s.closed {
			if err == nil {
				s.result = result
			}
			s.err = err
			s.version++
			s.busy = false
		}
		s.mu.Unlock()
		if onWork != nil {
			onWork(-1)
		}
		wake()
	}()
}
func (s *rasterState) snapshot() (RichResult, *ui.Bitmap, error, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.painted != s.version {
		s.painted = s.version
		if len(s.result.PNG) > 0 {
			if img, _, err := image.Decode(bytes.NewReader(s.result.PNG)); err == nil {
				s.bitmap = ui.NewBitmap(img)
			}
		}
	}
	return s.result, s.bitmap, s.err, s.busy
}

// RichView displays an actual rendered graphic in native controls. Parsing and
// layout happen in the configured background renderer. Invalid progressive
// input keeps the most recent valid preview and exposes its source and error.
func RichView(c *ui.Context, r RichRequest, o RichOptions) ui.Element { return richView(c, r, o, nil) }
func richView(c *ui.Context, r RichRequest, o RichOptions, fallback func()) ui.Element {
	root := ui.Column(c).AlignSelf(ui.Stretch).Gap(6)
	state := *ui.Local(root, "rich", func() *rasterState { return &rasterState{zoom: 1} })
	if r.Width == 0 {
		r.Width = o.Width
	}
	if r.Width == 0 {
		r.Width = 900
	}
	if r.FontSize == 0 {
		r.FontSize = 16
	}
	r.Dark = c.Theme().Dark
	if o.Renderer != nil {
		state.load(r, o.Renderer, c.Services().Invalidate, o.onWork)
	}
	out, bitmap, err, busy := state.snapshot()
	if state.emitted != state.painted {
		state.emitted = state.painted
		if o.OnRender != nil {
			o.OnRender(RichRenderState{r.Kind, state.request.Source, out.Width, out.Height, err})
		}
	}
	root.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, r.Kind).FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Spacer(c)
			label := "Source"
			if state.showSource {
				label = "Preview"
			}
			if ui.Button(c, label).Clicked() {
				state.showSource = !state.showSource
			}
			if ui.Button(c, "−").Disabled(bitmap == nil).Clicked() {
				state.zoom = max(.2, state.zoom/1.2)
			}
			if ui.Button(c, "+").Disabled(bitmap == nil).Clicked() {
				state.zoom = min(8, state.zoom*1.2)
			}
			if ui.Button(c, "Fit").Disabled(bitmap == nil).Clicked() {
				state.zoom = 1
				state.scroll.X = 0
				state.scroll.Y = 0
			}
			if ui.Button(c, "Fullscreen").Disabled(bitmap == nil).Clicked() {
				state.fullscreen = true
			}
			if ui.Button(c, "SVG").Disabled(out.SVG == "").Clicked() {
				exportRich(o, Export{r.Kind, "svg", r.Source, []byte(out.SVG)})
			}
			if ui.Button(c, "PNG").Disabled(len(out.PNG) == 0).Clicked() {
				exportRich(o, Export{r.Kind, "png", r.Source, out.PNG})
			}
		})
		if state.showSource || bitmap == nil {
			if fallback != nil {
				fallback()
			} else {
				ui.Text(c, r.Source).Font("monospace").Selectable()
			}
		} else {
			height := o.MaxHeight
			if height <= 0 {
				height = 500
			}
			ui.ScrollBoth(c).MaxHeight(height).TrackScroll(&state.scroll).AlignSelf(ui.Stretch).Children(func() {
				img := ui.Image(c, bitmap).Size(out.Width*state.zoom, out.Height*state.zoom).Label(out.Plain)
				x, y, _ := img.PointerPosition()
				if img.Pressed() {
					if state.pan {
						state.scroll.X -= x - state.panX
						state.scroll.Y -= y - state.panY
					}
					state.pan = true
					state.panX = x
					state.panY = y
				} else {
					state.pan = false
				}
			})
			for _, link := range out.Links {
				if SafeLink(link.URL) {
					ui.Link(c, link.Label, link.URL)
				}
			}
		}
		if busy {
			ui.Text(c, "Rendering…").FontSize(11).TextColor(c.Theme().TextMuted)
		}
		if err != nil {
			ui.Row(c).Gap(8).Children(func() {
				ui.Text(c, err.Error()).FontSize(11).TextColor(c.Theme().Danger)
				if ui.Button(c, "Retry").Clicked() {
					state.request = RichRequest{}

					c.Invalidate()
				}
			})
		}
		ui.Modal(c, &state.fullscreen, func() {
			ui.Column(c).Width(func() float32 { w, _ := c.Size(); return min(1000, w-32) }()).Height(func() float32 { _, h := c.Size(); return min(700, h-32) }()).Padding(16).Gap(10).Children(func() {
				ui.Row(c).Children(func() {
					ui.Text(c, r.Kind).Bold()
					ui.Spacer(c)
					if ui.Button(c, "Close preview").Clicked() {
						state.fullscreen = false
					}
				})
				if bitmap != nil {
					ui.ScrollBoth(c).Fill().Children(func() { ui.Image(c, bitmap).Size(out.Width*state.zoom, out.Height*state.zoom).Label(out.Plain) })
				}
			})
		})
	})
	return root
}
func (v *viewState) inlineMath(tex string, size float32) (InlineObject, bool) {
	r := RichRequest{Kind: "inline-math", Source: tex, Dark: v.dark, FontSize: size, Width: 1024}
	key := requestKey(r)
	if v.inline == nil {
		v.inline = map[[32]byte]*rasterState{}
	}
	state := v.inline[key]
	if state == nil {
		if len(v.inline) >= 256 {
			for k, s := range v.inline {
				s.Close()
				delete(v.inline, k)
				break
			}
		}
		state = &rasterState{}
		v.inline[key] = state
	}
	services := v.services
	state.load(r, v.props.Rich.Renderer, func() { v.richRevision.Add(1); services.Invalidate() }, v.props.Rich.onWork)
	result, bitmap, _, _ := state.snapshot()
	if bitmap == nil {
		return InlineObject{}, false
	}
	depth := result.Depth
	return InlineObject{Width: result.Width, Height: result.Height - depth, Depth: depth, Draw: func(p *ui.Painter, rect ui.Rect) { p.Image(bitmap, rect, ui.Contain) }}, true
}
func richLanguage(lang string) string {
	switch strings.ToLower(lang) {
	case "mermaid", "d2", "infographic", "html":
		return strings.ToLower(lang)
	case "tex", "latex", "math":
		return "math"
	}
	return ""
}
func (v *viewState) richRequest(kind, source string) RichRequest {
	return RichRequest{Kind: kind, Source: source, Width: v.props.Rich.Width, FontSize: v.fallback.FontSize, Dark: v.dark}
}
func (v *viewState) configureRich(c *ui.Context) {
	v.services = c.Services()
	v.dark = c.Theme().Dark
	if v.props.Rich == nil || v.props.Rich.Renderer == nil {
		return
	}
	v.richOptions = *v.props.Rich
	services := c.Services()
	v.richOptions.onWork = func(delta int) {
		v.jobs.Add(int64(delta))
		if delta < 0 {
			v.richRevision.Add(1)
			services.Invalidate()
		}
	}
	v.props.Rich = &v.richOptions

	if v.props.Math == nil {
		v.props.Math = func(c *ui.Context, tex string) { RichView(c, v.richRequest("math", tex), *v.props.Rich) }
	}
	if v.props.InlineMath == nil {
		v.props.InlineMath = v.inlineMath
	}
	if v.props.HTML == nil {
		v.props.HTML = func(c *ui.Context, source string) { RichView(c, v.richRequest("html", source), *v.props.Rich) }
	}
	if v.props.Virtual != nil {
		v.props.Virtual.MeasurementKey = fmt.Sprint(v.props.MeasurementKey, ":", v.richRevision.Load())
	}
}

func exportRich(o RichOptions, export Export) {
	if o.OnExport != nil {
		o.OnExport(export)
		return
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Title: "Export " + export.Kind, DefaultPath: export.Kind + "." + export.Format})
	if err == nil && path != "" {
		_ = os.WriteFile(path, export.Data, 0644)
	}
}
