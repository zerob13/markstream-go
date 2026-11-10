package markstream

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type CompareOptions struct {
	FileOptions
	Inline           editor.DiffInline
	Indicators       editor.DiffIndicators
	TabSize          int
	FontFamily       string
	NoBackground     bool
	ExpansionLines   int
	LineSelection    bool
	OnLineSelection  func(editor.DiffLineRange)
	OnGutterUtility  func(editor.DiffLineRange)
	Annotations      []editor.DiffAnnotation
	RenderAnnotation func(*ui.Context, editor.DiffAnnotation)
	Controller       func(*editor.DiffState)
}
type compareState struct {
	diff        editor.DiffState
	mode        DiffMode
	fold        bool
	initialized bool
	old, new    string
}

// Compare is the full native FileDiff surface for original/updated files:
// virtualized syntax highlighting, word/character diffs, split/unified layout,
// line selections, gutter actions, annotations and context expansion.
func Compare(c *ui.Context, old, new FileData, o CompareOptions) ui.Element {
	root := ui.Column(c).AlignSelf(ui.Stretch).Gap(6)
	state := *ui.Local(root, "compare", func() *compareState { return &compareState{} })
	if !state.initialized {
		state.mode = o.DiffMode
		state.fold = o.HideUnchanged
		state.diff.Old = editor.NewBuffer(old.Content)
		state.diff.New = editor.NewBuffer(new.Content)
		state.initialized = true
	}
	if old.Content != state.old {
		state.diff.Old.Set(old.Content)
		state.old = old.Content
	}
	if new.Content != state.new {
		state.diff.New.Set(new.Content)
		state.new = new.Content
	}
	state.diff.Old.SetLanguage(editor.LanguageNamed(old.Language))
	state.diff.New.SetLanguage(editor.LanguageNamed(new.Language))
	size, line, maxHeight := fileMetrics(o.FileOptions)
	d := &state.diff
	d.Font = ui.Font{Family: o.FontFamily, Size: size}
	if d.Font.Family == "" {
		d.Font.Family = "monospace"
	}
	d.LineHeight = line / size
	d.TabSize = o.TabSize
	d.NoBackground = o.NoBackground
	d.NoLineNumbers = o.HideLineNumbers
	d.Wrap = o.Wrap
	d.ContextLines = o.ContextLines
	d.ExpansionLines = o.ExpansionLines
	d.ExpandUnchanged = !state.fold
	d.Inline = o.Inline
	d.Indicators = o.Indicators
	d.LineSelection = o.LineSelection || o.OnLineSelection != nil
	d.OnLineSelection = o.OnLineSelection
	d.OnGutterButton = o.OnGutterUtility
	d.Annotations = o.Annotations
	d.RenderAnnotation = o.RenderAnnotation
	if state.mode == Split {
		d.Layout = editor.DiffSplit
	} else {
		d.Layout = editor.DiffUnified
	}
	root.Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, new.Name).Bold()
			ui.Spacer(c)
			label := "Split"
			if state.mode == Split {
				label = "Unified"
			}
			if ui.Button(c, label).Clicked() {
				if state.mode == Split {
					state.mode = Unified
				} else {
					state.mode = Split
				}
				c.Invalidate()
			}
			label = "Hide unchanged"
			if state.fold {
				label = "Show unchanged"
			}
			if ui.Button(c, label).Clicked() {
				state.fold = !state.fold
				c.Invalidate()
			}
			if ui.Button(c, "Copy updated").Clicked() {
				c.Services().WriteClipboard(new.Content)
			}
		})
		editor.DiffView(c, d).Height(maxHeight).AlignSelf(ui.Stretch)
	})
	if o.Controller != nil {
		o.Controller(d)
	}
	return root
}
