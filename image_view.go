package markstream

import (
	"github.com/egoist/mygo/ui"
	"sync"
)

type imageState struct {
	mu           sync.Mutex
	source       ui.ImageSource
	err          error
	done, closed bool
}

func (s *imageState) Close() error { s.mu.Lock(); s.closed = true; s.mu.Unlock(); return nil }
func (v *viewState) loadImage(c *ui.Context, n *Node) (ui.ImageSource, error) {
	if v.props.LoadImage == nil {
		return nil, nil
	}
	if v.images == nil {
		v.images = map[string]*imageState{}
	}
	entry := v.images[n.URL]
	if entry == nil {
		if len(v.images) >= 128 {
			for key, old := range v.images {
				old.Close()
				delete(v.images, key)
				break
			}
		}
		entry = &imageState{}
		v.images[n.URL] = entry
		load := v.props.LoadImage
		services := c.Services()
		v.jobs.Add(1)
		go func() {
			source, err := load(n.URL)
			entry.mu.Lock()
			if !entry.closed {
				entry.source = source
				entry.err = err
				entry.done = true
			}
			entry.mu.Unlock()
			v.jobs.Add(-1)
			v.richRevision.Add(1)
			services.Invalidate()
		}()
	}
	entry.mu.Lock()
	source, err := entry.source, entry.err
	entry.mu.Unlock()
	return source, err
}
func imageSize(source ui.ImageSource) (float32, float32) {
	width, height := float32(100), float32(80)
	switch image := source.(type) {
	case *ui.Bitmap:
		w, h := image.Size()
		width, height = float32(w), float32(h)
	case *ui.SVG:
		width, height = image.Size()
	}
	return width, height
}
func (v *viewState) image(c *ui.Context, n *Node, inline bool) {
	source, err := v.loadImage(c, n)
	if source == nil {
		label := n.Label
		if label == "" {
			label = v.label("Image")
		}
		el := ui.Text(c, label).Role(ui.RoleImage).Label(label)
		if err != nil {
			el.Tooltip(err.Error())
		}
		return
	}
	width, height := imageSize(source)
	root := ui.Column(c).AlignSelf(ui.Stretch)
	open := ui.Local(root, "image-preview", func() bool { return false })
	zoom := ui.Local(root, "image-zoom", func() float32 { return 1 })
	root.Children(func() {
		img := ui.Image(c, source).MaxWidthPercent(100).AspectRatio(width / height).Label(n.Label)
		if n.Title != "" {
			img.Tooltip(n.Title)
		}
		if img.Clicked() {
			*open = true
		}
		ui.Modal(c, open, func() {
			w, h := c.Size()
			ui.Column(c).Width(min(1000, w-32)).Height(min(700, h-32)).Padding(16).Gap(8).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, n.Label)
					ui.Spacer(c)
					if ui.Button(c, "−").Clicked() {
						*zoom = max(.2, *zoom/1.2)
					}
					if ui.Button(c, "+").Clicked() {
						*zoom = min(8, *zoom*1.2)
					}
					if ui.Button(c, v.label("Close image")).Clicked() {
						*open = false
					}
				})
				ui.ScrollBoth(c).Fill().Children(func() { ui.Image(c, source).Size(width**zoom, height**zoom).Label(n.Label) })
			})
		})
	})
}
