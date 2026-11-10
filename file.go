package markstream

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type FileData struct {
	Name, Language, Content string
	Streaming               bool
}
type LineEvent struct {
	Number, Old, New int
	Text             string
	Kind             DiffKind
}
type FileOptions struct {
	Streaming bool
	Language  string
	Compare   *CompareOptions

	FontSize, LineHeight, MaxHeight float32
	HideLineNumbers                 bool
	Wrap                            bool
	DiffMode                        DiffMode
	HideUnchanged                   bool
	ContextLines                    int
	OnLineClick                     func(LineEvent)
	// Annotation builds an optional native annotation following a file line.
	Annotation func(*ui.Context, LineEvent)
}
type fileState struct {
	source, language string
	lines            []string
	tokens           [][]editor.Token
	ends             []uint64
	list             ui.ListState
	selected         int
	expanded         map[string]bool
	mode             DiffMode
	fold             bool
	configured       bool
}

func fileMetrics(o FileOptions) (float32, float32, float32) {
	size := o.FontSize
	if size <= 0 {
		size = 13
	}
	line := o.LineHeight
	if line <= 0 {
		line = size * 1.5
	}
	height := o.MaxHeight
	if height <= 0 {
		height = 360
	}
	return size, line, height
}
func (s *fileState) update(data FileData) {
	if data.Content == s.source && data.Language == s.language && s.lines != nil {
		return
	}
	lines := strings.Split(strings.TrimSuffix(data.Content, "\n"), "\n")
	first := 0
	if s.language == data.Language {
		for first < len(lines) && first < len(s.lines) && lines[first] == s.lines[first] {
			first++
		}
	}
	s.tokens = s.tokens[:min(first, len(s.tokens))]
	s.ends = s.ends[:min(first, len(s.ends))]
	var tokenizer editor.Tokenizer
	if language := editor.LanguageNamed(data.Language); language != nil {
		tokenizer = language.Tokenizer
	}
	state := uint64(0)
	if first > 0 {
		state = s.ends[first-1]
	}
	for i := first; i < len(lines); i++ {
		var tokens []editor.Token
		if tokenizer != nil {
			tokens, state = tokenizer.Tokenize(lines[i], state, nil)
		}
		s.tokens = append(s.tokens, tokens)
		s.ends = append(s.ends, state)
	}
	s.source = data.Content
	s.language = data.Language
	s.lines = lines
}
func fileSpans(line string, tokens []editor.Token, theme *editor.Theme) []ui.Span {
	spans := make([]ui.Span, 0, len(tokens)+1)
	at := 0
	for _, token := range tokens {
		start, end := at, min(len(line), token.End)
		if start > end || start > len(line) {
			continue
		}
		if start > at {
			spans = append(spans, ui.Span{Text: line[at:start]})
		}
		style := theme.Tokens[token.Kind]
		spans = append(spans, ui.Span{Text: line[start:end], Color: style.Color, Weight: func() int {
			if style.Bold {
				return 700
			}
			return 0
		}(), Italic: style.Italic})
		at = end
	}
	if at < len(line) {
		spans = append(spans, ui.Span{Text: line[at:]})
	}
	if len(spans) == 0 {
		spans = append(spans, ui.Span{Text: line})
	}
	return spans
}

// File is a native, line-virtualized highlighted file, with a stable gutter,
// per-line interactions and annotations. Its copy action copies all source.
func File(c *ui.Context, data FileData, o FileOptions) ui.Element {
	root := ui.Column(c).AlignSelf(ui.Stretch).Gap(5)
	root.Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, data.Name).Bold()
			ui.Spacer(c)
			if ui.Button(c, "Copy file").Clicked() {
				c.Services().WriteClipboard(data.Content)
			}
		})
		fileBody(c, data, o, nil)
	})
	return root
}
func fileBody(c *ui.Context, data FileData, o FileOptions, note func(ui.Element, int)) ui.Element {
	box := ui.Column(c).AlignSelf(ui.Stretch)
	s := *ui.Local(box, "file", func() *fileState { return &fileState{} })
	s.update(data)
	size, lineHeight, maxHeight := fileMetrics(o)
	height := min(maxHeight, float32(len(s.lines))*lineHeight+8)
	gutter := max(4, len(strconv.Itoa(len(s.lines))))
	palette := editor.DefaultTheme(c.Theme())
	return box.Children(func() {
		ui.List(c, &s.list, len(s.lines), func(i int) {
			event := LineEvent{Number: i + 1, New: i + 1, Text: s.lines[i]}
			row := ui.Row(c).AlignSelf(ui.Stretch).MinHeight(lineHeight).Label(fmt.Sprintf("Line %d", i+1))
			if s.selected == i+1 {
				row.Background(c.Theme().Selection.Alpha(.3))
			}
			row.Children(func() {
				if !o.HideLineNumbers {
					ui.Text(c, fmt.Sprintf("%*d", gutter, i+1)).Font("monospace").FontSize(size).Width(float32(gutter)*size*.65+18).Padding(0, 8).TextColor(c.Theme().TextMuted).Unselectable()
				}
				el := ui.RichText(c, fileSpans(s.lines[i], s.tokens[i], palette)...).Font("monospace").FontSize(size).LineHeight(lineHeight / size)
				if !o.Wrap {
					el.NoWrap()
				}
				if note != nil {
					note(el, i)
				}
			})
			if row.Clicked() {
				s.selected = i + 1
				if o.OnLineClick != nil {
					o.OnLineClick(event)
				}
			}
			if o.Annotation != nil {
				o.Annotation(c, event)
			}
		}).Height(height).AlignSelf(ui.Stretch)
	})
}

// FileDiff displays unified patches with native unified/split switching and
// expandable unchanged regions. Its internal list only builds visible rows.
func FileDiff(c *ui.Context, source string, o FileOptions) ui.Element {
	root := ui.Column(c).AlignSelf(ui.Stretch).Gap(5)
	root.Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, "Changes").Bold()
			ui.Spacer(c)
			if ui.Button(c, "Copy diff").Clicked() {
				c.Services().WriteClipboard(source)
			}
		})
		diffBody(c, source, o)
	})
	return root
}
func patchBody(c *ui.Context, source string, o FileOptions) ui.Element {
	root := ui.Column(c).AlignSelf(ui.Stretch)
	s := *ui.Local(root, "diff", func() *fileState { return &fileState{expanded: map[string]bool{}} })
	if !s.configured {
		s.mode = o.DiffMode
		s.fold = o.HideUnchanged
		s.configured = true
	}
	rows := diffRows(ParseDiff(source), s.mode, s.fold, o.ContextLines, s.expanded)
	size, lineHeight, maxHeight := fileMetrics(o)
	root.Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			label := "Split"
			if s.mode == Split {
				label = "Unified"
			}
			if ui.Button(c, label).Clicked() {
				if s.mode == Split {
					s.mode = Unified
				} else {
					s.mode = Split
				}
				c.Invalidate()
			}
			label = "Hide unchanged"
			if s.fold {
				label = "Show unchanged"
			}
			if ui.Button(c, label).Clicked() {
				s.fold = !s.fold
				c.Invalidate()
			}
		})
		s.list.Key = func(i int) any { return rows[i].Key }
		ui.List(c, &s.list, len(rows), func(i int) {
			row := rows[i]
			if len(row.Hidden) > 0 {
				if ui.Button(c, row.Text).Clicked() {
					s.expanded[row.Key] = true
					c.Invalidate()
				}
				return
			}
			if row.Text != "" {
				ui.Text(c, row.Text).Font("monospace").FontSize(size).Padding(4, 8).TextColor(c.Theme().TextMuted).Background(c.Theme().SurfaceHover)
				return
			}
			ui.Row(c).AlignItems(ui.Stretch).MinHeight(lineHeight).Children(func() {
				if s.mode == Split {
					diffCell(c, row.Left, true, o)
					diffCell(c, row.Right, false, o)
				} else {
					line := row.Right
					if line == nil {
						line = row.Left
					}
					diffCell(c, line, false, o)
				}
			})
		}).Height(min(maxHeight, float32(max(len(rows), 1))*lineHeight+8)).AlignSelf(ui.Stretch)
	})
	return root
}
func diffCell(c *ui.Context, line *DiffLine, old bool, o FileOptions) {
	size, lineHeight, _ := fileMetrics(o)
	box := ui.Row(c).FillWidth().MinHeight(lineHeight)
	if line == nil {
		return
	}
	color := c.Theme().Text
	bg := ui.Color{}
	if line.Kind == DiffAdded {
		color = ui.Hex("#16803c")
		bg = ui.Hex("#dcfce7")
		if c.Theme().Dark {
			color = ui.Hex("#86efac")
			bg = ui.Hex("#163027")
		}
	}
	if line.Kind == DiffRemoved {
		color = ui.Hex("#be123c")
		bg = ui.Hex("#ffe4e6")
		if c.Theme().Dark {
			color = ui.Hex("#fda4af")
			bg = ui.Hex("#3d1c29")
		}
	}
	box.Background(bg).Label(fmt.Sprintf("Diff line %d %d", line.Old, line.New)).Children(func() {
		if !o.HideLineNumbers {
			number := line.New
			if old {
				number = line.Old
			}
			value := ""
			if number > 0 {
				value = strconv.Itoa(number)
			}
			ui.Text(c, fmt.Sprintf("%4s", value)).Font("monospace").FontSize(size).Width(size*4*.65+18).Padding(0, 8).TextColor(c.Theme().TextMuted).Unselectable()
		}
		prefix := " "
		if line.Kind == DiffAdded {
			prefix = "+"
		} else if line.Kind == DiffRemoved {
			prefix = "−"
		}
		ui.Text(c, prefix).Font("monospace").FontSize(size).TextColor(color).Unselectable()
		text := ui.Text(c, line.Text).Font("monospace").FontSize(size).LineHeight(lineHeight / size).TextColor(color)
		if !o.Wrap {
			text.NoWrap()
		}
	})
	event := LineEvent{Number: line.New, Old: line.Old, New: line.New, Text: line.Text, Kind: line.Kind}
	if box.Clicked() && o.OnLineClick != nil {
		o.OnLineClick(event)
	}
	if o.Annotation != nil {
		o.Annotation(c, event)
	}
}
