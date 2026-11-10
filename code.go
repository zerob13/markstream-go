package markstream

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// CodeOptions configures the native code surface. Zero values retain the built-in
// highlighted, selectable body and Copy action. Language renderers take priority.
type CodeOptions struct {
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
	root := ui.Column(c).AlignSelf(ui.Stretch).Background(th.Surface).Border(1, th.Border).Radius(8).Clip().Role(ui.RoleGroup).Label("Code, " + b.Language)
	collapsed := ui.Local(root, "collapsed", func() bool { return o.DefaultCollapsed })
	return root.Children(func() {
		ui.Row(c).Padding(7, 12).Gap(8).AlignItems(ui.Center).Children(func() {
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
				ui.Text(c, "Receiving").FontSize(11).TextColor(th.TextMuted).Unselectable()
			}
			if o.Collapsible {
				label := "Collapse"
				if *collapsed {
					label = "Expand"
				}
				if ui.Button(c, label).Clicked() {
					*collapsed = !*collapsed
				}
			}
			if !o.HideCopy {
				b.CopyButton(c)
			}
		})
		if o.Collapsible && *collapsed {
			ui.Text(c, "Code collapsed").Padding(8, 12).TextColor(th.TextMuted).Unselectable()
			return
		}
		if o.MaxHeight > 0 {
			ui.Scroll(c).MaxHeight(o.MaxHeight).AlignSelf(ui.Stretch).Children(func() { b.Body(c) })
		} else {
			b.Body(c)
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
