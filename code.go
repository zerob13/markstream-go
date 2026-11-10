package markstream

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// CodeOptions configures the native code surface. Zero values retain the built-in
// highlighted, selectable body and Copy action. Language renderers take priority.
type CodeOptions struct {
	ShowFontSize            bool
	HeaderLeft, HeaderRight func(*ui.Context, *CodeBlock)
	Labels                  Labels
	File                    *FileOptions

	Collapsible      bool
	DefaultCollapsed bool
	// MaxHeight bounds the body with a vertical scroll container (DIPs).
	MaxHeight    float32
	HideCopy     bool
	HideLanguage bool
	// Filename is optional metadata computed from a fence's Info string.
	Filename func(*CodeBlock) string
}

// RenderCode draws the same native code surface used by View when CodeOptions
// is set. Custom renderers can call it for fallback without losing copy behavior.
func RenderCode(c *ui.Context, b *CodeBlock, o CodeOptions) ui.Element {
	th := c.Theme()
	theme := resolveTheme(c, Props{})
	if b.owner != nil {
		theme = b.owner.fallback
	}
	root := ui.Column(c).AlignSelf(ui.Stretch).Background(theme.CodeBlockBackground).Border(1, theme.CodeBlockBorder).Radius(theme.CodeBlockRadius).Clip().Role(ui.RoleGroup).Label("Code, " + b.Language)
	collapsed := ui.Local(root, "collapsed", func() bool { return o.DefaultCollapsed })
	fontSize := ui.Local(root, "font-size", func() float32 {
		if o.File != nil && o.File.FontSize > 0 {
			return o.File.FontSize
		}
		return theme.FontSize * theme.CodeSize
	})
	return root.Children(func() {
		ui.Row(c).Padding(7, 12).Gap(8).AlignItems(ui.Center).Children(func() {
			if o.HeaderLeft != nil {
				o.HeaderLeft(c, b)
			}
			if !o.HideLanguage {
				label := b.Language
				if o.Filename != nil {
					if name := o.Filename(b); name != "" {
						label = name
					}
				}
				ui.Text(c, label).FontSize(12).TextColor(th.TextMuted).Unselectable()
			}
			ui.Spacer(c)
			if b.Open {
				ui.Text(c, tr(o.Labels, "Receiving")).FontSize(11).TextColor(th.TextMuted).Unselectable()
			}
			if o.Collapsible {
				label := "Collapse"
				if *collapsed {
					label = "Expand"
				}
				if ui.Button(c, tr(o.Labels, label)).Clicked() {
					*collapsed = !*collapsed
				}
			}
			if o.ShowFontSize {
				if ui.Button(c, "A−").Clicked() {
					*fontSize = max(9, *fontSize-1)
				}
				if ui.Button(c, "A+").Clicked() {
					*fontSize = min(32, *fontSize+1)
				}
			}
			if o.HeaderRight != nil {
				o.HeaderRight(c, b)
			}
			if !o.HideCopy {
				b.CopyButton(c)
			}
		})
		if o.Collapsible && *collapsed {
			ui.Text(c, tr(o.Labels, "Code collapsed")).Padding(8, 12).TextColor(th.TextMuted).Unselectable()
			return
		}
		fileOptions := FileOptions{Labels: o.Labels, FontSize: *fontSize, LineHeight: *fontSize * theme.CodeLineHeight, Font: theme.CodeFont, Theme: theme.CodeColors}
		if b.owner != nil {
			fileOptions.Wrap = b.owner.props.WrapCode
		}
		if o.File != nil {
			fileOptions = *o.File
			if fileOptions.Labels == nil {
				fileOptions.Labels = o.Labels
			}
			if fileOptions.FontSize <= 0 || o.ShowFontSize {
				fileOptions.FontSize = *fontSize
			}
			if fileOptions.LineHeight <= 0 {
				fileOptions.LineHeight = fileOptions.FontSize * theme.CodeLineHeight
			}
			if fileOptions.Font == "" {
				fileOptions.Font = theme.CodeFont
			}
			if fileOptions.Theme == nil {
				fileOptions.Theme = theme.CodeColors
			}
		}
		if o.MaxHeight > 0 {
			fileOptions.MaxHeight = o.MaxHeight
		}
		if b.Language == "diff" || b.Language == "patch" {
			fileOptions.Streaming = b.Open
			fields := strings.Fields(b.Info)
			if len(fields) > 1 {
				fileOptions.Language = fields[1]
			}
			diffBody(c, b.Code(), fileOptions)
		} else {
			fileBody(c, FileData{Language: b.Language, Content: b.Code(), Streaming: b.Open}, fileOptions, b.NoteElement)
		}
	})
}

// FenceFilename returns filename metadata from a fence such as
// ```go title=main.go or ```go filename="hello world.go". It does not read files.
func FenceFilename(b *CodeBlock) string {
	info := b.Info
	for _, key := range []string{"filename=", "title="} {
		start := strings.Index(info, key)
		if start < 0 || start > 0 && info[start-1] != ' ' && info[start-1] != '\t' {
			continue
		}
		rest := info[start+len(key):]
		if rest == "" {
			continue
		}
		if rest[0] == '"' || rest[0] == '\'' {
			if end := strings.IndexByte(rest[1:], rest[0]); end >= 0 {
				return rest[1 : end+1]
			}
			continue
		}
		if fields := strings.Fields(rest); len(fields) > 0 {
			return fields[0]
		}
		return ""
	}
	return ""
}
