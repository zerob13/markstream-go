package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	ms "github.com/zerob13/markstream-go"
)

func TestDemoPlayback(t *testing.T) {
	if err := smoke(newDemo(), ""); err != nil {
		t.Fatal(err)
	}
}
func TestDemoCancelAndRestart(t *testing.T) {
	d := newDemo()
	tt := ui.NewTester(d.view, 1280, 850)
	tt.Click("Stream reply")
	tt.Advance(50 * time.Millisecond)
	tt.Frame()
	tt.Click("Cancel")
	if d.reply.Snapshot().State != ms.Canceled || d.playing {
		t.Fatal("cancel failed")
	}
	tt.Click("Reset sample")
	if s := d.reply.Snapshot(); s.State != ms.Completed || s.Source != sample {
		t.Fatal("reset failed")
	}
	d.source = "# Edited source\n\nNew text."
	tt.Click("Stream reply")
	for range 20 {
		tt.Advance(50 * time.Millisecond)
		tt.Frame()
	}
	if s := d.reply.Snapshot(); s.Source != d.source || s.State != ms.Completed {
		t.Fatal("edited source not streamed")
	}
}
func TestDemoVirtualHistoryAndDarkTheme(t *testing.T) {
	d := newDemo()
	d.dark = true
	d.page = "1,000 messages"
	tt := ui.NewTester(d.view, 1280, 850)
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), "Reply 1000") {
		t.Fatal("history did not follow latest")
	}
	if e := tt.Click("First message"); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), "Reply 0001") {
		t.Fatal("history did not scroll to first")
	}
	img := tt.Image()
	if img.Bounds().Dx() != 1280 || img.Bounds().Dy() != 850 {
		t.Fatal("native renderer did not produce image")
	}
}
