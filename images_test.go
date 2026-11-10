package markstream

import (
	"bytes"
	"encoding/base64"
	"github.com/egoist/mygo/ui"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testImagePNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 48, 32))
	for y := range 32 {
		for x := range 48 {
			img.Set(x, y, color.RGBA{uint8(x * 5), uint8(y * 7), 120, 255})
		}
	}
	var out bytes.Buffer
	png.Encode(&out, img)
	return out.Bytes()
}
func TestImageLoaderAndNativePreview(t *testing.T) {
	data := testImagePNG()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	defer server.Close()
	loader := ImageLoader(ImageOptions{BaseURL: server.URL, MaxBytes: 1 << 20}, nil)
	if _, err := loader("/image.png"); err != nil {
		t.Fatal(err)
	}
	if _, err := loader("data:image/png;base64," + base64.StdEncoding.EncodeToString(data)); err != nil {
		t.Fatal(err)
	}
	if _, err := ImageLoader(ImageOptions{MaxBytes: 1}, nil)(server.URL); err == nil {
		t.Fatal("byte limit ignored")
	}
	if _, err := ImageLoader(ImageOptions{AllowURL: func(string) bool { return false }}, nil)(server.URL); err == nil {
		t.Fatal("URL policy ignored")
	}
	src := "![Native image](" + server.URL + "/image.png)"
	tt := ui.NewTester(func(c *ui.Context) { View(c, src, Props{}) }, 700, 400)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if e := tt.Click("Native image"); e == nil && tt.HasText("Close image") {
			break
		}
		time.Sleep(time.Millisecond * 5)
	}
	if !tt.HasText("Close image") {
		t.Fatal("native image did not load/open preview", tt.Texts())
	}
	if err := tt.Click("Close image"); err != nil {
		t.Fatal(err)
	}
}
