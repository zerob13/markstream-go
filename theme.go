package markstream

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"time"
)

type HeadingStyle struct {
	Size        float32
	Weight      int
	Color       ui.Color
	Top, Bottom float32
	Rule        bool
}
type Theme struct {
	FontSize                                        float32
	Font, CodeFont                                  string
	LineHeight, CodeLineHeight                      float32
	Text, Muted, Link, Strong                       ui.Color
	StrongWeight, LinkWeight                        int
	Code, CodeBackground                            ui.Color
	CodeSize, BlockSpacing, ItemSpacing, ListIndent float32
	Headings                                        [6]HeadingStyle
	QuoteBar, QuoteText                             ui.Color
	QuoteBarWidth                                   float32
	Alerts                                          [5]ui.Color
	Rule                                            ui.Color
	CodeBlockBackground, CodeBlockBorder            ui.Color
	CodeBlockRadius, CodeBlockPadding               float32
	CodeColors                                      *editor.Theme
	TableBorder, TableHeader, TableStripe           ui.Color
	TableCellPadding                                [2]float32
	TableMaxColumn                                  float32
	Mark, Checkbox                                  ui.Color
}

func resolveTheme(c *ui.Context, p Props) Theme {
	uiTheme := c.Theme()
	size := float32(16)
	if p.Variant == ChatStyle {
		size = 14
	}
	t := Theme{FontSize: size, Font: uiTheme.Font, CodeFont: "monospace", LineHeight: 1.7, CodeLineHeight: 1.5, Text: uiTheme.Text, Muted: uiTheme.TextMuted, Link: uiTheme.Accent, Strong: uiTheme.Text, StrongWeight: 600, Code: uiTheme.Text, CodeBackground: uiTheme.SurfaceHover, CodeSize: .875, BlockSpacing: .75, ItemSpacing: .3, ListIndent: 1.5, QuoteBar: uiTheme.Border, QuoteText: uiTheme.TextMuted, QuoteBarWidth: 3, Rule: uiTheme.Border, TableBorder: uiTheme.Border, TableHeader: uiTheme.SurfaceHover, TableCellPadding: [2]float32{.6, .4}, TableMaxColumn: 24, CodeBlockBackground: uiTheme.Surface, CodeBlockBorder: uiTheme.Border, CodeBlockRadius: 8, CodeBlockPadding: .8, Mark: ui.Hex("#fef08a"), Checkbox: uiTheme.Accent}
	t.Alerts = [5]ui.Color{ui.Hex("#3b82f6"), ui.Hex("#22c55e"), ui.Hex("#a855f7"), ui.Hex("#f59e0b"), ui.Hex("#ef4444")}
	for i, s := range []float32{2, 1.65, 1.4, 1.2, 1.1, 1} {
		t.Headings[i] = HeadingStyle{Size: s, Weight: 700, Color: t.Text, Top: 1, Bottom: .5}
	}
	if p.Theme != nil {
		o := p.Theme
		if o.FontSize > 0 {
			t.FontSize = o.FontSize
		}
		if o.Font != "" {
			t.Font = o.Font
		}
		if o.CodeFont != "" {
			t.CodeFont = o.CodeFont
		}
		if o.LineHeight > 0 {
			t.LineHeight = o.LineHeight
		}
		if o.CodeLineHeight > 0 {
			t.CodeLineHeight = o.CodeLineHeight
		}
		for _, pair := range [][2]*ui.Color{{&t.Text, &o.Text}, {&t.Muted, &o.Muted}, {&t.Link, &o.Link}, {&t.Strong, &o.Strong}, {&t.Code, &o.Code}, {&t.CodeBackground, &o.CodeBackground}, {&t.QuoteBar, &o.QuoteBar}, {&t.QuoteText, &o.QuoteText}, {&t.Rule, &o.Rule}, {&t.TableBorder, &o.TableBorder}, {&t.TableHeader, &o.TableHeader}, {&t.TableStripe, &o.TableStripe}, {&t.Mark, &o.Mark}, {&t.Checkbox, &o.Checkbox}} {
			if pair[1].A > 0 {
				*pair[0] = *pair[1]
			}
		}
		if o.StrongWeight > 0 {
			t.StrongWeight = o.StrongWeight
		}
		if o.LinkWeight > 0 {
			t.LinkWeight = o.LinkWeight
		}
		if o.CodeSize > 0 {
			t.CodeSize = o.CodeSize
		}
		if o.BlockSpacing > 0 {
			t.BlockSpacing = o.BlockSpacing
		}
		if o.ItemSpacing > 0 {
			t.ItemSpacing = o.ItemSpacing
		}
		if o.ListIndent > 0 {
			t.ListIndent = o.ListIndent
		}
		if o.TableMaxColumn > 0 {
			t.TableMaxColumn = o.TableMaxColumn
		}
		if o.TableCellPadding[0] > 0 {
			t.TableCellPadding = o.TableCellPadding
		}
		for i, h := range o.Headings {
			if h.Size > 0 {
				t.Headings[i].Size = h.Size
			}
			if h.Weight > 0 {
				t.Headings[i].Weight = h.Weight
			}
			if h.Color.A > 0 {
				t.Headings[i].Color = h.Color
			}
		}
	}
	return t
}

type InlineObject struct {
	Width, Height, Depth float32
	Draw                 func(*ui.Painter, ui.Rect)
}
type RevealUnit uint8

const (
	ByWord RevealUnit = iota
	ByCharacter
	BySentence
	ByLine
	ByParagraph
	AtOnce
)

type Reveal struct {
	Unit          RevealUnit
	CatchUp, Fade time.Duration
}
