package main

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	ms "github.com/zerob13/markstream-go"
	"github.com/zerob13/markstream-go/rich"
)

//go:embed rich.md
var richSample string

func (d *demo) renderer() *rich.Engine {
	if d.engine == nil {
		d.engine = rich.New(rich.Options{BrowserPath: os.Getenv("MARKSTREAM_CHROME_PATH"), NoSandbox: os.Getenv("MARKSTREAM_CHROME_NO_SANDBOX") == "1"})
	}
	return d.engine
}
func (d *demo) Close() error {
	if d.engine != nil {
		return d.engine.Close()
	}
	return nil
}
func (d *demo) richProps() ms.Props {
	return ms.Props{Emoji: true, Rich: &ms.RichOptions{Renderer: d.renderer(), Progressive: true, MaxHeight: 400, OnExport: d.exporter, OnRender: func(state ms.RichRenderState) {
		if state.Err != nil {
			d.richErrors = append(d.richErrors, state.Err)
		}
	}}, OnUpdate: func(status ms.RenderStatus) { d.renderStatus = status }, CustomHTML: map[string]func(*ui.Context, *ms.Node){"status-card": func(c *ui.Context, n *ms.Node) {
		ui.Column(c).Padding(14).Radius(8).Background(c.Theme().Accent.Alpha(.12)).Children(func() {
			ui.Text(c, "Native status card").Bold()
			ui.Text(c, "This custom tag is rendered by markstream's node registry.")
		})
	}}}
}
func (d *demo) gallery(c *ui.Context) bool {
	switch d.page {
	case "Rich content":
		ui.Row(c).Padding(8, 28).Gap(10).Children(func() {
			ui.Text(c, "Mermaid · KaTeX · D2 · Infographic · HTML").TextColor(c.Theme().TextMuted)
			ui.Spacer(c)
			if ui.PrimaryButton(c, "Stream this gallery").Clicked() {
				d.source = richSample
				d.page = "Playground"
				d.start()
				c.Invalidate()
			}
		})
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).Padding(20, 28).AlignSelf(ui.Stretch).Children(func() { ms.View(c, richSample, d.richProps()) })
		})
		return true
	case "Large document":
		if d.longSource == "" {
			var source strings.Builder
			for i := range 2500 {
				fmt.Fprintf(&source, "## Section %04d\n\nA stable paragraph with **bold text**, `inline code`, and a [shared reference][go].\n\n", i+1)
			}
			source.WriteString("[go]: https://go.dev\n")
			d.longSource = source.String()
		}
		ui.Row(c).Padding(8, 28).Gap(8).Children(func() {
			ui.Text(c, fmt.Sprintf("%d nodes · %d rendered · %d measured", d.document.Metrics().NodeCount, d.document.Metrics().RenderedCount, d.document.Metrics().MeasuredCount))
			ui.Spacer(c)
			if ui.Button(c, "Middle").Clicked() {
				d.document.ScrollTo(2500, ui.Start)
			}
			if ui.Button(c, "End").Clicked() {
				d.document.ScrollToEnd()
			}
			if ui.Button(c, "Save position").Clicked() {
				saved := d.document.Capture()
				d.saved = &saved
			}
			if ui.Button(c, "Restore position").Disabled(d.saved == nil).Clicked() {
				d.document.Reset()
				d.document.Restore(*d.saved)
			}
		})
		ui.Column(c).Fill().Padding(0, 28, 20).Children(func() { ms.View(c, d.longSource, ms.Props{Virtual: &d.document}) })
		return true
	case "File diff":
		if d.oldFile == "" {
			d.oldFile = "package main\n\nfunc greet(name string) string {\n    return \"Hello, \" + name\n}\n" + strings.Repeat("// unchanged context\n", 25)
			d.newFile = strings.Replace(d.oldFile, "Hello, ", "Welcome, ", 1) + "\nfunc version() string { return \"v1\" }\n"
		}
		ui.Row(c).Padding(8, 28).Gap(10).Children(func() {
			ui.Text(c, "Native FileDiff · word/character highlights · line selection · context expansion").TextColor(c.Theme().TextMuted)
			ui.Spacer(c)
			ui.Select(c, &d.diffMode, []string{"Word", "Character", "None"}).Width(130)
		})
		inline := editor.DiffInlineWord
		if d.diffMode == "Character" {
			inline = editor.DiffInlineChar
		} else if d.diffMode == "None" {
			inline = editor.DiffInlineNone
		}
		ui.Column(c).Fill().Padding(16, 28).Children(func() {
			ms.Compare(c, ms.FileData{Name: "greet.go", Language: "go", Content: d.oldFile}, ms.FileData{Name: "greet.go", Language: "go", Content: d.newFile}, ms.CompareOptions{FileOptions: ms.FileOptions{MaxHeight: 600, DiffMode: ms.Split, HideUnchanged: true}, Inline: inline, LineSelection: true, OnLineSelection: func(selection editor.DiffLineRange) {
				d.selection = fmt.Sprintf("Selected lines %d–%d", selection.Start+1, selection.End+1)
			}})
			ui.Text(c, d.selection).FontSize(12).TextColor(c.Theme().TextMuted)
		})
		return true
	}
	return false
}
