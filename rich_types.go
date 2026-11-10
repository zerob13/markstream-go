package markstream

import "context"

type RichRequest struct {
	Kind     string  `json:"kind"`
	Source   string  `json:"source"`
	Dark     bool    `json:"dark"`
	Width    int     `json:"width"`
	FontSize float32 `json:"fontSize"`
}
type RichLink struct {
	URL                 string `json:"url"`
	Label               string `json:"label"`
	X, Y, Width, Height float32
}
type RichResult struct {
	PNG    []byte     `json:"-"`
	SVG    string     `json:"svg"`
	Plain  string     `json:"plain"`
	Width  float32    `json:"width"`
	Height float32    `json:"height"`
	Depth  float32    `json:"depth"`
	Links  []RichLink `json:"links"`
}

// RichRenderer computes a graphic away from MyGo's UI thread. The rich package
// provides the bundled Mermaid/KaTeX/D2/Infographic/HTML implementation.
type RichRenderer interface {
	Render(context.Context, RichRequest) (RichResult, error)
}
type Export struct {
	Kind, Format, Source string
	Data                 []byte
}
type RichOptions struct {
	onWork   func(int)
	OnRender func(RichRenderState)

	Renderer  RichRenderer
	Width     int
	MaxHeight float32
	// Progressive renders valid diagram prefixes while a fence is still open.
	Progressive bool
	// OnExport receives the real SVG or PNG. A nil handler opens the native Save dialog.
	OnExport func(Export)
}

// RichRenderState is delivered on the UI thread after a render completes.
type RichRenderState struct {
	Kind, Source  string
	Width, Height float32
	Err           error
}

type RenderStatus struct {
	Receiving, Revealing, Settled, Final bool
	Pending                              int
	Metrics                              VirtualMetrics
}
