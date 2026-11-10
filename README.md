# markstream-go

**Streaming Markdown components for [MyGo](https://github.com/egoist/mygo), with an independent Go renderer.**

[中文说明](README.zh-CN.md) · [API](docs/api.md) · [markstream-vue comparison](docs/parity.md) · [Performance](docs/performance.md) · [Demo](examples/demo)

![Native dark playground](docs/images/demo-dark.png)

A component library for LLM conversations, streamed documents and long Markdown files. The streaming behavior and feature set follow [markstream-vue](https://github.com/Simon-He95/markstream-vue); the native component approach is informed by [Lobe MyGo](https://github.com/lobehub/lobe-mygo).

**This library does not import or copy MyGo's Markdown widget.** Markstream owns its node model, renderer, streaming projection, reveal animation, code surfaces, virtual document state and extension registry. It reuses MyGo's low-level CommonMark/GFM parser, native UI primitives and editor/diff primitives.

## Features

- CommonMark/GFM, nested/task lists, tables, alerts, footnotes, definitions, `:::` containers and details.
- Incomplete-inline repair while receiving, strict EOF finalization, incremental parsing, cached completed nodes and stable identities across edits/prepends.
- Thread-safe `io.Writer` sessions: UTF-8 chunk buffering, limits, immutable snapshots, coalesced notifications, cancellation and generation-safe reset.
- Grapheme-aware character reveal, word/sentence/line/paragraph pacing, visible fade, reduced-motion support and completion events that wait for background work.
- Native highlighted code, filename headers, copy, collapse, wrap, size controls and custom header slots.
- **File / FileDiff / Compare**: line virtualization, split/unified layouts, word/character highlights, original patch line numbers, context expansion, selections, gutter actions and annotations. Diff fences have a streaming surface and a final native editor surface.
- **Single-document virtualization** and virtualized message histories, variable-height measurement, stable row keys, scroll anchors, follow-end and serializable position/height snapshots.
- **Real Mermaid, KaTeX, D2 and AntV Infographic renderers**, progressive valid-prefix previews, source mode, pan/zoom, fullscreen and SVG/PNG export.
- Bounded image loading, SVG, native image previews, safe links, heading/footnote anchors, extended inline syntax, emoji and sanitized HTML.
- Per-view node/tag/language overrides, parsed-node input, parse transforms, light/dark themes and localized control labels.
- A native desktop demo with five pages: streaming playground, rich-content gallery, 5,000-node document, FileDiff and 1,000-message history.

The application window uses native MyGo controls, **without a WebView**. Core Markdown, code and diff need only Go. The optional `rich` package embeds the actual rendering engines/fonts and uses a shared **headless Chromium worker** for graphics and HTML layout. It requires a locally installed Chromium/Chrome executable; it does not download assets at runtime. Node is needed only to rebuild bundled assets. See [architecture and compatibility](docs/parity.md).

## Install

Requires **Go 1.27.3+**, matching the pinned MyGo dependency.

```sh
go get github.com/zerob13/markstream-go@main
```

MyGo supports Linux, macOS and Windows without cgo. A desktop window needs the platform libraries and a display; software-rendered native tests and PNG snapshots work without a display server.

## Quick start

```go
package main

import (
    "log"

    "github.com/egoist/mygo"
    "github.com/egoist/mygo/ui"
    ms "github.com/zerob13/markstream-go"
)

func main() {
    reply := ms.NewSession(1 << 20)
    mygo.App.WhenReady(func() {
        mygo.NewWindow(mygo.WindowOptions{
            Title: "Streaming Markdown", Width: 800, Height: 600,
            Content: ui.View(func(c *ui.Context) {
                ui.Scroll(c).Fill().Children(func() {
                    ms.Stream(c, reply, ms.Props{})
                })
            }),
        })
        go func() {
            if err := reply.Append("# Hello\n\nNative **streaming** Markdown."); err != nil {
                reply.Fail(err)
                return
            }
            _ = reply.Finish()
        }()
    })
    if err := mygo.App.Run(); err != nil { log.Fatal(err) }
}
```

Write chunks from a producer goroutine; build UI on the UI thread. Use `Finish`, `Cancel` or `Fail` at the end. `Consume(ctx, reader)` accepts raw Markdown; decode SSE/provider JSON before appending text deltas. Keep repeated views keyed with `c.Key(messageID)`.

For source already in application state:

```go
ms.View(c, sourceSoFar, ms.Props{
    Streaming: receiving,
    Emoji: true,
    CodeOptions: &ms.CodeOptions{
        Collapsible: true, ShowFontSize: true,
        MaxHeight: 360, Filename: ms.FenceFilename,
    },
    OnFinal: func(status ms.RenderStatus) {
        // EOF, reveal/fade and outstanding mounted graphics/images have settled.
    },
})
```

Enable the bundled rich renderers with one shared engine, retained for the application's lifetime:

```go
import "github.com/zerob13/markstream-go/rich"

engine := rich.New(rich.Options{}) // optionally set BrowserPath
// Close engine when the application exits.
props := ms.Props{Rich: &ms.RichOptions{
    Renderer: engine, Progressive: true, MaxHeight: 500,
}}
// Inside a UI frame:
ms.View(c, source, props)
```

Without an engine, math/diagram/HTML source remains readable. Invalid progressive diagram input keeps the last successful preview and exposes an error/source view. Applications can replace the backend through `RichRenderer`.

For a long document, retain a `VirtualState` and give its view a bounded viewport:

```go
// Application state:
virtual := &ms.VirtualState{}
// Inside a filling native container:
ms.View(c, source, ms.Props{Virtual: virtual})
// virtual.Capture() / virtual.Restore(snapshot) preserve a compatible position.
```

Automatic block virtualization starts above 220 top-level nodes; use `MaxLiveNodes` to change the threshold or `DisableVirtualization` to opt out. Explicit `Virtual` gives the host control of viewport sizing and navigation.

## Run the demo

```sh
git clone https://github.com/zerob13/markstream-go.git
cd markstream-go
go run ./examples/demo
```

Install Chromium/Chrome for the rich-content gallery. Set `MARKSTREAM_CHROME_PATH` when it is not discoverable. `MARKSTREAM_CHROME_NO_SANDBOX=1` is available for isolated test containers that cannot run Chromium's sandbox; desktop use keeps the sandbox enabled.

```sh
go run ./examples/demo -smoke
go run ./examples/demo -snapshot artifacts/demo-light.png
go run ./examples/demo -dark -snapshot artifacts/demo-dark.png
go run ./examples/demo -page 'Rich content' -snapshot artifacts/gallery.png
go run ./examples/demo -page 'File diff' -snapshot artifacts/diff.png
```

These PNGs come from MyGo's native software renderer. The rich engines generate graphics in the background; the component controls, layout and window are native.

## Develop

```sh
go mod download
go test ./...
go test -race ./...
go vet ./...
go build ./examples/demo
go test -run '^$' -bench . -benchmem
```

The rich tests use an installed Chromium and skip when none is present. CI has a separate required rich-engine job with Chromium installed and mandatory browser detection. To rebuild the committed offline assets:

```sh
npm ci --prefix tools/renderer
npm run build --prefix tools/renderer
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and the [API guide](docs/api.md).

## License

The Go library is [MIT](LICENSE). Bundled third-party engines retain their own licenses, including D2's MPL-2.0; all notices and the corresponding-source reference are in [THIRD_PARTY.md](THIRD_PARTY.md). No Vue or Lobe component implementation is vendored.
