// Demo is a native MyGo app. -smoke runs the same view with MyGo's software
// renderer, checking streaming controls without a display server.
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	ms "github.com/zerob13/markstream-go"
)

//go:embed sample.md
var sample string

type demo struct {
	source                         string
	reply                          *ms.Session
	active                         []rune
	position                       int
	playing, paused, dark, instant bool
	page                           string
	last                           time.Time
	history                        []ms.Message
	list                           ui.ListState
	revealed                       bool
}

func newDemo() *demo {
	d := &demo{source: sample, reply: ms.NewSession(1 << 20), page: "Playground", list: ui.ListState{FollowEnd: true}}
	d.reply.Append(sample)
	d.reply.Finish()
	for i := range 1000 {
		d.history = append(d.history, ms.Message{ID: fmt.Sprint(i), Source: fmt.Sprintf("### Reply %04d\n\nA cached native Markdown message with **bold text**, `code`, and a [link](https://go.dev).\n\n- Stable identity\n- Variable row height\n", i+1)})
	}
	return d
}
func (d *demo) start() {
	d.reply.Reset()
	d.active = []rune(d.source)
	d.position = 0
	d.playing = true
	d.paused = false
	d.last = time.Time{}
}
func (d *demo) step(c *ui.Context) {
	if !d.playing || d.paused {
		return
	}
	now := c.Now()
	if d.last.IsZero() || now.Sub(d.last) >= 20*time.Millisecond {
		end := min(d.position+9, len(d.active))
		if err := d.reply.Append(string(d.active[d.position:end])); err != nil {
			d.reply.Fail(err)
			d.playing = false
			return
		}
		d.position = end
		d.last = now
		if end == len(d.active) {
			d.reply.Finish()
			d.playing = false
		}
	}
	if d.playing {
		c.After(20 * time.Millisecond)
	}
}
func (d *demo) view(c *ui.Context) {
	theme := *ui.LightTheme()
	if d.dark {
		theme = *ui.DarkTheme()
	}
	theme.Accent = ui.Hex("#6d5ce8")
	c.SetTheme(&theme)
	d.step(c)
	ui.Column(c).Fill().Background(theme.Background).Children(func() {
		ui.Row(c).Padding(20, 28).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "m/").FontSize(28).Bold().TextColor(theme.Accent)
			ui.Column(c).Gap(3).Children(func() {
				ui.Text(c, "markstream-go").FontSize(22).Bold()
				ui.Text(c, "Streaming Markdown · native MyGo components").FontSize(12).TextColor(theme.TextMuted)
			})
			ui.Spacer(c)
			ui.Select(c, &d.page, []string{"Playground", "1,000 messages"}).Width(165)
			ui.Checkbox(c, &d.dark, "Dark")
		})
		if d.page == "1,000 messages" {
			ui.Row(c).Padding(8, 28).Children(func() {
				ui.Text(c, "1,000 messages · only visible rows are built").TextColor(theme.TextMuted)
				ui.Spacer(c)
				if ui.Button(c, "First message").Clicked() {
					d.list.ScrollTo(0, ui.Start)
				}
				if ui.Button(c, "Latest message").Clicked() {
					d.list.ScrollTo(999, ui.End)
				}
			})
			ms.Messages(c, &d.list, d.history, ms.Props{Variant: ms.ChatStyle}).Fill()
			return
		}
		ui.Row(c).Padding(4, 28, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.PrimaryButton(c, "Stream reply").Clicked() {
				d.start()
				c.Invalidate()
			}
			label := "Pause"
			if d.paused {
				label = "Resume"
			}
			if ui.Button(c, label).Disabled(!d.playing).Clicked() {
				d.paused = !d.paused
				d.last = time.Time{}
				c.Invalidate()
			}
			if ui.Button(c, "Finish now").Disabled(!d.playing).Clicked() {
				if err := d.reply.Append(string(d.active[d.position:])); err != nil {
					d.reply.Fail(err)
				} else {
					d.reply.Finish()
				}
				d.playing = false
			}
			if ui.Button(c, "Cancel").Disabled(!d.playing).Clicked() {
				d.reply.Cancel()
				d.playing = false
			}
			if ui.Button(c, "Reset sample").Clicked() {
				d.source = sample
				d.playing = false
				d.reply.Reset()
				d.reply.Append(sample)
				d.reply.Finish()
			}
			ui.Spacer(c)
			ui.Checkbox(c, &d.instant, "Instant reveal")
		})
		ui.Row(c).Fill().Padding(0, 28, 20).Gap(20).Children(func() {
			ui.Column(c).Width(400).Gap(8).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, "MARKDOWN SOURCE").FontSize(11).Bold().TextColor(theme.TextMuted)
					ui.Spacer(c)
					ui.Text(c, "Editable").FontSize(11).TextColor(theme.TextMuted)
				})
				ui.TextArea(c, &d.source).Fill().Font("monospace").FontSize(12).Label("Markdown source")
			})
			ui.Column(c).Fill().Gap(8).Children(func() {
				snap := d.reply.Snapshot()
				ui.Row(c).Children(func() {
					ui.Text(c, "NATIVE PREVIEW").FontSize(11).Bold().TextColor(theme.TextMuted)
					ui.Spacer(c)
					status := snap.State.String()
					if d.playing && d.paused {
						status = "paused"
					}
					ui.Text(c, fmt.Sprintf("%s · %d bytes", status, len(snap.Source))).FontSize(11).TextColor(theme.Accent)
				})
				ui.Scroll(c).Fill().Background(theme.Surface).Border(1, theme.Border).Radius(12).Children(func() {
					ui.Column(c).Padding(24).AlignSelf(ui.Stretch).Children(func() {
						p := ms.Props{CodeOptions: &ms.CodeOptions{Collapsible: true, MaxHeight: 340, Filename: ms.FenceFilename}}
						if d.instant {
							p.Reveal = &ms.Reveal{Unit: ms.AtOnce, Fade: -1}
						}
						d.revealed = ms.Revealed(ms.Stream(c, d.reply, p))
						if snap.Err != nil {
							ui.Text(c, snap.Err.Error()).TextColor(theme.Danger)
						}
					})
				})
			})
		})
		ui.Row(c).Padding(0, 28, 14).Children(func() {
			ui.Text(c, "Go only. No webview. Select text, copy code, or explore the virtualized history.").FontSize(11).TextColor(theme.TextMuted)
		})
	})
}
func saveSnapshot(tt *ui.Tester, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = png.Encode(f, tt.Image())
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func smoke(d *demo, path string) error {
	tt := ui.NewTester(d.view, 1280, 850)
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), "A reply, as it arrives") {
		return fmt.Errorf("initial Markdown missing")
	}
	if err := tt.Click("Stream reply"); err != nil {
		return err
	}
	for range 350 {
		tt.Advance(25 * time.Millisecond)
		tt.Frame()
	}
	if snap := d.reply.Snapshot(); snap.State != ms.Completed || snap.Source != sample || !d.revealed {
		return fmt.Errorf("stream did not settle: state=%s revealed=%v", snap.State, d.revealed)
	}
	if err := tt.Click("Stream reply"); err != nil {
		return err
	}
	if err := tt.Click("Pause"); err != nil {
		return err
	}
	paused := d.reply.Snapshot().Source
	tt.Advance(time.Second)
	tt.Frame()
	if d.reply.Snapshot().Source != paused {
		return fmt.Errorf("pause failed")
	}
	if err := tt.Click("Resume"); err != nil {
		return err
	}
	if err := tt.Click("Finish now"); err != nil {
		return err
	}
	tt.Advance(time.Second)
	tt.Frame()
	if path != "" {
		if err := saveSnapshot(tt, path); err != nil {
			return err
		}
	}
	fmt.Println("Native demo smoke passed: initial rendering, streaming EOF, pause, resume, finish, and software rendering.")
	return nil
}
func main() {
	smokeFlag := flag.Bool("smoke", false, "run native UI smoke checks without a display")
	snapshot := flag.String("snapshot", "", "write a native software-rendered PNG without a display")
	dark := flag.Bool("dark", false, "start with the dark theme")
	flag.Parse()
	d := newDemo()
	d.dark = *dark
	if *smokeFlag {
		if err := smoke(d, *snapshot); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *snapshot != "" {
		tt := ui.NewTester(d.view, 1280, 850)
		if err := saveSnapshot(tt, *snapshot); err != nil {
			log.Fatal(err)
		}
		return
	}
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{Title: "markstream-go · Native Markdown", Width: 1280, Height: 850, MinWidth: 1000, MinHeight: 650, Content: ui.View(d.view)})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
