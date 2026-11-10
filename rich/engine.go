// Package rich provides bundled, offline renderers for Mermaid, KaTeX, D2,
// AntV Infographic and sanitized HTML. A shared headless Chromium process does
// layout off the UI thread; the application window uses native MyGo controls.
package rich

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	ms "github.com/zerob13/markstream-go"
)

//go:embed assets/*
var assets embed.FS

type Options struct {
	BrowserPath  string
	Timeout      time.Duration
	CacheEntries int
	CacheBytes   int
	// NoSandbox is for container test environments that cannot create Chromium's
	// sandbox. Desktop applications should retain the default sandbox.
	NoSandbox bool
}
type Engine struct {
	opts        Options
	gate        chan struct{}
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	allocCancel context.CancelFunc
	tab         context.Context
	tabCancel   context.CancelFunc
	server      *http.Server
	origin      string
	cache       map[[32]byte]ms.RichResult
	order       [][32]byte
	closed      bool
	cacheBytes  int
}

func New(o Options) *Engine {
	if o.Timeout <= 0 {
		o.Timeout = 30 * time.Second
	}
	if o.CacheEntries <= 0 {
		o.CacheEntries = 128
	}
	if o.CacheBytes <= 0 {
		o.CacheBytes = 64 << 20
	}
	return &Engine{opts: o, gate: make(chan struct{}, 1), cache: map[[32]byte]ms.RichResult{}}
}
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}
	e.closed = true

	if e.cancel != nil {
		e.cancel()
	}
	if e.allocCancel != nil {
		e.allocCancel()
	}
	if e.server != nil {
		return e.server.Close()
	}
	return nil
}
func (e *Engine) start() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return errors.New("rich: renderer is closed")
	}
	if e.ctx != nil {
		return nil
	}
	browser := e.opts.BrowserPath
	if browser == "" {
		for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "chrome"} {
			if p, err := exec.LookPath(name); err == nil {
				browser = p
				break
			}
		}
	}
	if browser == "" {
		return errors.New("rich: Chromium is required; set rich.Options.BrowserPath")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	e.origin = "http://" + ln.Addr().String()
	e.server = &http.Server{Handler: http.HandlerFunc(e.serve), ReadHeaderTimeout: 5 * time.Second}
	go e.server.Serve(ln)
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts, chromedp.ExecPath(browser), chromedp.Flag("host-resolver-rules", "MAP * ~NOTFOUND, EXCLUDE 127.0.0.1"), chromedp.Flag("disable-background-networking", true))
	if e.opts.NoSandbox {
		opts = append(opts, chromedp.NoSandbox)
	}
	alloc, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	e.allocCancel = cancel
	e.ctx, e.cancel = chromedp.NewContext(alloc)
	if err := chromedp.Run(e.ctx); err != nil {
		e.cancel()
		e.allocCancel()
		e.ctx = nil
		return err
	}
	return nil
}
func (e *Engine) serve(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}
	if strings.Contains(name, "/") {
		http.NotFound(w, r)
		return
	}
	f, err := assets.Open("assets/" + name + ".gz")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Security-Policy", "default-src 'self' data: blob:; script-src 'self' blob: 'wasm-unsafe-eval' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self'; font-src 'self' data:; object-src 'none'; base-uri 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(name)))
	z, err := gzip.NewReader(f)
	if err != nil {
		http.Error(w, "invalid asset", 500)
		return
	}
	defer z.Close()
	io.Copy(w, z)
}
func (e *Engine) Render(ctx context.Context, r ms.RichRequest) (ms.RichResult, error) {
	if len(r.Source) > 1<<20 {
		return ms.RichResult{}, errors.New("rich: source exceeds 1 MiB")
	}
	if r.Width <= 0 {
		r.Width = 900
	}
	if r.Width > 4096 {
		return ms.RichResult{}, errors.New("rich: width exceeds 4096")
	}
	data, _ := json.Marshal(r)
	key := sha256.Sum256(data)
	select {
	case e.gate <- struct{}{}:
	case <-ctx.Done():
		return ms.RichResult{}, ctx.Err()
	}
	defer func() { <-e.gate }()
	if err := e.start(); err != nil {
		return ms.RichResult{}, err
	}
	if result, ok := e.cache[key]; ok {
		return result, nil
	}
	if e.tab == nil {
		e.tab = e.ctx
		e.tabCancel = func() {}
		initCtx, cancel := context.WithTimeout(e.tab, e.opts.Timeout)
		err := chromedp.Run(initCtx, chromedp.Navigate(e.origin), chromedp.WaitReady("body"), chromedp.Poll("window.markstreamReady === true", nil, chromedp.WithPollingInterval(50*time.Millisecond)))
		cancel()
		if err != nil {
			e.tabCancel()
			e.tab = nil
			return ms.RichResult{}, fmt.Errorf("rich: initialize: %w", err)
		}
	}
	job, cancel := context.WithTimeout(e.tab, e.opts.Timeout)
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	defer cancel()
	var out ms.RichResult
	err := chromedp.Run(job, chromedp.Evaluate("window.markstreamRender("+string(data)+")", &out, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }))
	if err == nil {
		err = chromedp.Run(job, chromedp.ActionFunc(func(c context.Context) error {
			bytes, e := page.CaptureScreenshot().WithFormat(page.CaptureScreenshotFormatPng).WithCaptureBeyondViewport(true).WithClip(&page.Viewport{X: 0, Y: 0, Width: float64(out.Width), Height: float64(out.Height), Scale: 1}).Do(c)
			out.PNG = bytes
			return e
		}))
	}
	if err != nil {
		e.tabCancel()
		e.tab = nil
		return ms.RichResult{}, fmt.Errorf("rich: %s: %w", r.Kind, err)
	}
	if len(out.PNG) > 16<<20 {
		return ms.RichResult{}, errors.New("rich: image exceeds 16 MiB")
	}
	cost := len(out.PNG) + len(out.SVG)
	if cost <= e.opts.CacheBytes {
		for len(e.order) > 0 && (len(e.order) >= e.opts.CacheEntries || e.cacheBytes+cost > e.opts.CacheBytes) {
			old := e.cache[e.order[0]]
			e.cacheBytes -= len(old.PNG) + len(old.SVG)
			delete(e.cache, e.order[0])
			e.order = e.order[1:]
		}
		e.cache[key] = out
		e.cacheBytes += cost
		e.order = append(e.order, key)
	}
	return out, nil
}
