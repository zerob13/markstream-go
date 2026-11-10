package markstream

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/egoist/mygo/ui"
	"sort"
)

type HeightEntry struct {
	Index     int
	Height    float32
	Signature string
}
type VirtualSnapshot struct {
	ContentHash    string
	Width          float32
	Anchor         int
	Offset         float32
	Heights        []HeightEntry
	MeasurementKey string
}
type VirtualMetrics struct {
	NodeCount, RenderedCount, First, Last, MeasuredCount, EstimatedCount int
	Width, TotalHeight                                                   float32
	Final, Stable                                                        bool
	Phase                                                                string
}

// VirtualState owns Markstream's block-level virtual document viewport. Its
// cache and anchors are independent from the low-level syntax parser.
type VirtualState struct {
	List            ui.ListState
	Scroll          ui.ScrollState
	MeasurementKey  string
	heights         map[int]HeightEntry
	signature       string
	width           float32
	anchor          int
	offset          float32
	restore         *VirtualSnapshot
	restorePhase    int
	metrics         VirtualMetrics
	bounds          ui.Rect
	lastMeasurement string
}

func (v *VirtualState) Capture() VirtualSnapshot {
	out := VirtualSnapshot{v.signature, v.width, v.anchor, v.offset, nil, v.MeasurementKey}
	for _, h := range v.heights {
		out.Heights = append(out.Heights, h)
	}
	sort.Slice(out.Heights, func(i, j int) bool { return out.Heights[i].Index < out.Heights[j].Index })
	return out
}
func (v *VirtualState) Restore(s VirtualSnapshot)          { v.restore = &s; v.restorePhase = 0 }
func (v *VirtualState) Metrics() VirtualMetrics            { return v.metrics }
func (v *VirtualState) ScrollTo(index int, align ui.Align) { v.List.ScrollTo(index, align) }
func (v *VirtualState) ScrollToEnd()                       { v.List.ScrollToEnd() }
func (v *VirtualState) Reset() {
	follow := v.List.FollowEnd
	*v = VirtualState{List: ui.ListState{FollowEnd: follow}}
}
func contentHash(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:12])
}
func (v *VirtualState) render(c *ui.Context, owner *viewState, nodes []*Node) ui.Element {
	v.signature = contentHash(owner.shownSource)
	if v.heights == nil || v.lastMeasurement != v.MeasurementKey {
		v.heights = map[int]HeightEntry{}
		v.lastMeasurement = v.MeasurementKey
	}
	if v.restore != nil && v.restorePhase == 0 {
		saved := v.restore
		if saved.ContentHash == v.signature && saved.MeasurementKey == v.MeasurementKey {
			v.List.ScrollTo(min(max(saved.Anchor, 0), max(len(nodes)-1, 0)), ui.Start)
			v.restorePhase = 1
			for _, h := range saved.Heights {
				if h.Index >= 0 && h.Index < len(nodes) && h.Signature == contentHash(nodes[h.Index].Raw) {
					v.heights[h.Index] = h
				}
			}
		} else {
			v.restore = nil
		}
	}
	first, last := v.List.Visible()
	v.metrics = VirtualMetrics{NodeCount: len(nodes), First: first, Last: last, Width: v.width, Final: !owner.receiving && !owner.revealing}
	live := map[int]bool{}
	v.List.Key = func(i int) any { return nodes[i].ID }
	list := ui.List(c, &v.List, len(nodes), func(i int) {
		live[i] = true
		v.metrics.RenderedCount = len(live)
		container := ui.Column(c).AlignSelf(ui.Stretch)
		rect := container.Bounds()
		key := contentHash(nodes[i].Raw)
		if cached, ok := v.heights[i]; ok && cached.Signature == key && v.restore != nil && v.restore.Width == v.width {
			container.MinHeight(cached.Height)
		}
		if rect.H > 0 {
			v.heights[i] = HeightEntry{i, rect.H, key}
			if i == first {
				v.anchor = i
				v.offset = v.bounds.Y - rect.Y
			}
		}
		container.Children(func() { owner.renderNode(c, nodes[i]) })
	}).TrackScroll(&v.Scroll).Fill()
	bounds := list.Bounds()
	if bounds.W > 0 && v.width != bounds.W {
		v.width = bounds.W
		clear(v.heights)
		if v.restore != nil && v.restore.Width != v.width {
			v.restore = nil
		}
	}
	v.bounds = bounds
	if v.restore != nil && v.restorePhase == 1 && first == v.restore.Anchor {
		v.Scroll.Y += v.restore.Offset
		v.restorePhase = 2
		c.Invalidate()
	} else if v.restorePhase == 2 {
		v.restore = nil
		v.restorePhase = 0
	}
	measured := float32(0)
	for i, h := range v.heights {
		if i >= len(nodes) || h.Signature != contentHash(nodes[i].Raw) {
			delete(v.heights, i)
			continue
		}
		measured += h.Height
	}
	v.metrics.MeasuredCount = len(v.heights)
	v.metrics.EstimatedCount = max(0, len(nodes)-len(v.heights))
	v.metrics.TotalHeight = measured
	if len(v.heights) > 0 {
		v.metrics.TotalHeight += measured / float32(len(v.heights)) * float32(v.metrics.EstimatedCount)
	}
	v.metrics.Width = v.width
	v.metrics.Stable = v.metrics.Final && owner.jobs.Load() == 0 && v.restore == nil
	v.metrics.Phase = "streaming"
	if v.metrics.Stable {
		v.metrics.Phase = "final"
	}
	return list
}

func layoutKey(v *viewState) string {
	return fmt.Sprintf("%s:%g:%g:%s:%t:%d:%d", v.props.MeasurementKey, v.fallback.FontSize, v.fallback.LineHeight, v.fallback.Font, v.dark, v.richRevision.Load(), v.props.ParseRevision)
}
