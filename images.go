package markstream

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type ImageOptions struct {
	BaseURL  string
	Client   *http.Client
	MaxBytes int64
	// AllowURL can apply an application-specific host policy before every fetch.
	AllowURL func(string) bool
}

// ImageLoader loads bounded HTTP(S)/data images off the UI thread. Native MyGo
// decodes raster formats; a configured rich renderer handles full SVG layout.
func ImageLoader(o ImageOptions, renderer RichRenderer) func(string) (ui.ImageSource, error) {
	limit := o.MaxBytes
	if limit <= 0 {
		limit = 16 << 20
	}
	return func(address string) (ui.ImageSource, error) {
		var dataBytes []byte
		var err error
		if strings.HasPrefix(address, "data:") {
			header, data, found := strings.Cut(address, ",")
			if !found || !strings.HasPrefix(header, "data:image/") {
				return nil, errors.New("image: unsupported data URL")
			}
			if int64(len(data)) > limit*2 {
				return nil, errors.New("image: data exceeds byte limit")
			}
			if strings.HasSuffix(header, ";base64") {
				dataBytes, err = base64.StdEncoding.DecodeString(data)
			} else {
				var decoded string
				decoded, err = url.PathUnescape(data)
				dataBytes = []byte(decoded)
			}
		} else {
			parsed, e := url.Parse(address)
			if e != nil {
				return nil, e
			}
			if parsed.Scheme == "" && o.BaseURL != "" {
				base, e := url.Parse(o.BaseURL)
				if e != nil {
					return nil, e
				}
				parsed = base.ResolveReference(parsed)
				address = parsed.String()
			}
			if parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
				return nil, errors.New("image: only HTTP(S) and image data URLs are supported")
			}
			if o.AllowURL != nil && !o.AllowURL(address) {
				return nil, errors.New("image: URL rejected by host policy")
			}
			client := &http.Client{Timeout: 20 * time.Second}
			if o.Client != nil {
				copy := *o.Client
				client = &copy
			}
			redirect := client.CheckRedirect
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("image: too many redirects")
				}
				if req.URL.Scheme != "https" && req.URL.Scheme != "http" {
					return errors.New("image: unsafe redirect")
				}
				if o.AllowURL != nil && !o.AllowURL(req.URL.String()) {
					return errors.New("image: redirect rejected")
				}
				if redirect != nil {
					return redirect(req, via)
				}
				return nil
			}
			response, e := client.Get(address)
			if e != nil {
				return nil, e
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("image: HTTP %d", response.StatusCode)
			}
			dataBytes, err = io.ReadAll(io.LimitReader(response.Body, limit+1))
		}
		if err != nil {
			return nil, err
		}
		if int64(len(dataBytes)) > limit {
			return nil, errors.New("image: byte limit exceeded")
		}
		trimmed := strings.TrimSpace(string(dataBytes))
		if strings.HasPrefix(trimmed, "<svg") || strings.HasPrefix(trimmed, "<?xml") {
			if renderer != nil {
				out, e := renderer.Render(context.Background(), RichRequest{Kind: "svg", Source: string(dataBytes), Width: 1024})
				if e != nil {
					return nil, e
				}
				return ui.DecodeBitmap(out.PNG)
			}
			return ui.ParseSVG(dataBytes)
		}
		config, _, e := image.DecodeConfig(bytes.NewReader(dataBytes))
		if e != nil {
			return nil, e
		}
		if config.Width <= 0 || config.Height <= 0 || uint64(config.Width)*uint64(config.Height) > 32<<20 {
			return nil, errors.New("image: dimensions exceed 32 megapixels")
		}
		return ui.DecodeBitmap(dataBytes)
	}
}
