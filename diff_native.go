package markstream

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"path/filepath"
	"strings"
)

type nativePatchView struct {
	source      string
	files       []FilePatch
	diffs       []*editor.DiffState
	initialized bool
	list        ui.ListState
}

func nativePatch(file FilePatch) editor.FilePatch {
	out := editor.FilePatch{OldName: file.OldName, NewName: file.NewName, Binary: file.Binary}
	var hunk *editor.PatchHunk
	for _, line := range file.Lines {
		if line.Kind == DiffHunk {
			out.Hunks = append(out.Hunks, editor.PatchHunk{})
			hunk = &out.Hunks[len(out.Hunks)-1]
			if match := hunkHeader.FindStringSubmatch(line.Text); match != nil {
				fmtScan(match[1], &hunk.OldStart)
				fmtScan(match[3], &hunk.NewStart)
			}
			continue
		}
		if line.Kind != DiffAdded && line.Kind != DiffRemoved && line.Kind != DiffContext {
			continue
		}
		if hunk == nil {
			out.Hunks = append(out.Hunks, editor.PatchHunk{OldStart: max(1, line.Old), NewStart: max(1, line.New)})
			hunk = &out.Hunks[len(out.Hunks)-1]
		}
		prefix := " "
		switch line.Kind {
		case DiffAdded:
			prefix = "+"
			hunk.NewLines++
		case DiffRemoved:
			prefix = "-"
			hunk.OldLines++
		default:
			hunk.OldLines++
			hunk.NewLines++
		}
		hunk.Lines = append(hunk.Lines, prefix+line.Text)
	}
	return out
}
func fmtScan(text string, to *int) {
	value := 0
	for _, digit := range text {
		value = value*10 + int(digit-'0')
	}
	*to = value
}
func diffBody(c *ui.Context, source string, o FileOptions) ui.Element {
	if o.Streaming {
		return patchBody(c, source, o)
	}
	root := ui.Column(c).AlignSelf(ui.Stretch)
	state := *ui.Local(root, "native-patch", func() *nativePatchView { return &nativePatchView{} })
	// The light streaming surface and final editor share toolbar state and
	// viewport estimation, so finalization preserves the user's layout/height.
	controls := *ui.Local(root, "diff", func() *fileState { return &fileState{expanded: map[string]bool{}} })
	if !controls.configured {
		controls.mode = o.DiffMode
		controls.fold = o.HideUnchanged
		controls.configured = true
	}
	if source != state.source || !state.initialized {
		state.files = ParseDiff(source)
		state.diffs = make([]*editor.DiffState, len(state.files))
		for i, file := range state.files {
			d := &editor.DiffState{}
			d.SetPatch(nativePatch(file))
			state.diffs[i] = d
		}
		state.source = source
		state.initialized = true
	}
	size, line, maxHeight := fileMetrics(o)
	rows := diffRows(state.files, controls.mode, controls.fold, o.ContextLines, controls.expanded)
	height := min(maxHeight, float32(max(len(rows), 1))*line+8)
	root.Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			label := "Split"
			if controls.mode == Split {
				label = "Unified"
			}
			if ui.Button(c, tr(o.Labels, label)).Clicked() {
				if controls.mode == Split {
					controls.mode = Unified
				} else {
					controls.mode = Split
				}
				c.Invalidate()
			}
			label = "Hide unchanged"
			if controls.fold {
				label = "Show unchanged"
			}
			if ui.Button(c, tr(o.Labels, label)).Clicked() {
				controls.fold = !controls.fold
				c.Invalidate()
			}
		})
		ui.List(c, &state.list, len(state.files), func(i int) {
			file := state.files[i]
			d := state.diffs[i]
			name := file.NewName
			if name == "" {
				name = file.OldName
			}
			ui.Text(c, name).Font("monospace").FontSize(size).TextColor(c.Theme().TextMuted)
			language := o.Language
			if language == "" {
				language = strings.TrimPrefix(filepath.Ext(name), ".")
			}
			d.Old.SetLanguage(editor.LanguageNamed(language))
			d.New.SetLanguage(editor.LanguageNamed(language))
			d.Font = ui.Font{Family: "monospace", Size: size}
			d.LineHeight = line / size
			d.NoLineNumbers = o.HideLineNumbers
			d.Wrap = o.Wrap
			d.ContextLines = o.ContextLines
			d.ExpandUnchanged = !controls.fold
			if controls.mode == Split {
				d.Layout = editor.DiffSplit
			} else {
				d.Layout = editor.DiffUnified
			}
			if options := o.Compare; options != nil {
				d.Inline = options.Inline
				d.Indicators = options.Indicators
				d.TabSize = options.TabSize
				d.NoBackground = options.NoBackground
				d.ExpansionLines = options.ExpansionLines
				d.LineSelection = options.LineSelection || options.OnLineSelection != nil
				d.OnLineSelection = options.OnLineSelection
				d.OnGutterButton = options.OnGutterUtility
				d.Annotations = options.Annotations
				d.RenderAnnotation = options.RenderAnnotation
				if options.Controller != nil {
					options.Controller(d)
				}
			}
			if file.Binary {
				ui.Text(c, "Binary file changed")
			} else {
				editor.DiffView(c, d).Height(max(30, height-line)).AlignSelf(ui.Stretch)
			}
		}).Height(height).AlignSelf(ui.Stretch)
	})
	return root
}
