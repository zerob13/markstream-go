package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	ms "github.com/zerob13/markstream-go"
)

func TestDemoPlayback(t *testing.T) {
	d := newDemo()
	defer d.Close()
	if err := smoke(d, ""); err != nil {
		t.Fatal(err)
	}
}
func TestDemoCancelAndRestart(t *testing.T) {
	d := newDemo()
	defer d.Close()
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
	defer d.Close()
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

func TestDemoLargeDocumentSaveAndRestore(t *testing.T) {
	d := newDemo()
	defer d.Close()
	d.page = "Large document"
	tt := ui.NewTester(d.view, 1280, 850)
	if !tt.HasText("Section 0001") || d.document.Metrics().RenderedCount > 100 {
		t.Fatal("large document is not virtualized", d.document.Metrics())
	}
	if err := tt.Click("Middle"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Section 1251") {
		t.Fatal("middle navigation failed", tt.Texts())
	}
	tt.Click("Save position")
	tt.Click("End")
	tt.Frame()
	if !tt.HasText("Section 2500") {
		t.Fatal("end navigation failed")
	}
	tt.Click("Restore position")
	tt.Frame()
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Section 1251") {
		t.Fatal("saved anchor was not restored", d.document.Metrics())
	}
}
func TestDemoNativeFileDiff(t *testing.T) {
	d := newDemo()
	defer d.Close()
	d.page = "File diff"
	tt := ui.NewTester(d.view, 1280, 850)
	if !tt.HasText("greet.go") || !tt.HasText("New text") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("Unified"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Copy updated"); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != d.newFile {
		t.Fatal("updated file copy lost content")
	}
}
func TestDemoRichGallery(t *testing.T) {
	if os.Getenv("MARKSTREAM_CHROME_PATH") == "" {
		if _, err := exec.LookPath("chromium"); err != nil {
			if os.Getenv("MARKSTREAM_REQUIRE_CHROME") == "1" {
				t.Fatal("Chromium is required by this test job")
			}
			t.Skip("Chromium is needed for the rich gallery")
		}
	}
	d := newDemo()
	defer d.Close()
	d.page = "Rich content"
	tt := ui.NewTester(d.view, 1280, 850)
	deadline := time.Now().Add(45 * time.Second)
	for !d.renderStatus.Final && time.Now().Before(deadline) {
		tt.Advance(20 * time.Millisecond)
		tt.Frame()
		time.Sleep(5 * time.Millisecond)
	}
	if !d.renderStatus.Final {
		t.Fatal("gallery did not settle", d.renderStatus)
	}
	if len(d.richErrors) > 0 {
		t.Fatal("rich engines failed", d.richErrors)
	}
	if !tt.HasText("Mathematics") || !tt.HasText("Mermaid") {
		t.Fatal("gallery missing", tt.Texts())
	}
	if err := tt.Click("Stream this gallery"); err != nil {
		t.Fatal(err)
	}
	if d.page != "Playground" || d.source != richSample || !d.playing {
		t.Fatal("gallery did not start streaming")
	}
}
