# API guide

## Views and identity

```go
el := ms.View(c, sourceSoFar, ms.Props{Streaming: receiving})
shown := ms.Revealed(el)
```

`View` receives the complete source-so-far. `Stream(c, session, props)` subscribes to a Session and derives `Streaming` from its state. Keep the view in the same keyed position when finalizing; use `c.Key(id)` for repeated views. UI elements and `CodeBlock` handles are valid only during their build frame. All UI builders run on the UI thread.

`Revealed` means the current reveal/fade queue and mounted async work have drained; it can be true while waiting for more transport input. `Props.OnUpdate` runs during builds, `OnSettled` signals a drained queue and `OnFinal` additionally requires EOF. UI layout may build several times per frame; keep callbacks cheap and do not synchronously change the source from them. `Pending` lets an application include its own outstanding work in completion.

## Concurrent sessions and transport

The zero value of `Session` is usable. `NewSession(maxBytes)` sets a byte limit; zero is unlimited. It implements `io.Writer` and must not be copied after first use. Writes are serialized in lock-acquisition order; normally use one producer per reply.

| Operation | Behavior |
| --- | --- |
| `Append(string)` / `Write([]byte)` | Append while receiving; buffer split UTF-8 codepoints |
| `Finish()` | Enter Completed; fail incomplete UTF-8 with `ErrInvalidUTF8` |
| `Cancel()` | Preserve complete source, enter Canceled with `context.Canceled` |
| `Fail(err)` | Preserve complete source and enter Failed; nil does nothing |
| `Reset()` | Clear source/error, increment generation and retain subscribers/limit |
| `Snapshot()` | Immutable source, state/error, revision/generation and buffered-byte count |
| `Consume(ctx, reader)` | Consume raw Markdown until EOF and finish |

Snapshots do not copy the accumulated response; returned source strings stay valid after writes/reset. Writes between snapshots are coalesced into one invalidation. `Stream` detaches subscriptions on removal or session replacement.

Invalid UTF-8 and capacity violations reject the entire write with `(0, error)` and preserve previous content. Buffered partial UTF-8 counts toward the limit. Direct write rejection is not terminal until the producer calls `Fail`; writes after terminal state return `ErrClosed`.

`Consume` captures its generation; reset makes stale data/EOF fail with `ErrReset`. Application-managed producers using `Append` must be canceled/joined before resetting, or use a new Session per reply.

```go
req, err := http.NewRequestWithContext(ctx, http.MethodGet, markdownURL, nil)
if err != nil { return err }
resp, err := client.Do(req)
if err != nil { return err }
defer resp.Body.Close()
if resp.StatusCode != http.StatusOK { return fmt.Errorf("Markdown: %s", resp.Status) }
return reply.Consume(ctx, resp.Body)
```

For SSE/provider JSON, use the provider SDK to decode events, append only the text delta, then Finish at the completion event. The session does not parse SSE framing. A context cannot interrupt an arbitrary blocked `Read`; use a context-bound HTTP request or close the reader from its owner. `Consume` does not close caller-owned readers.

## Reveal, theme and labels

```go
props := ms.Props{
    Variant: ms.ChatStyle,
    Theme: &ms.Theme{FontSize: 15, Link: ui.Hex("#7c3aed")},
    Reveal: &ms.Reveal{Unit: ms.ByWord,
        CatchUp: 120 * time.Millisecond, Fade: 180 * time.Millisecond},
    Locale: "zh-CN",
    Labels: ms.Labels{"Copy": "复制代码"},
}
```

Default typography is 16 DIP/1.7 line height; ChatStyle uses 14 DIP. Nonzero theme fields override independent Markstream defaults. Themes include heading styles, strong/link/code colors, quote/alert colors, table metrics and an editor code palette.

Reveal units are ByCharacter (Unicode graphemes), ByWord, BySentence, ByLine, ByParagraph and AtOnce. Default catch-up is 120 ms and fade 180 ms. Explicit `Fade: 0` or `-1` disables fade. Incomplete lines/paragraphs can wait until EOF. Desktop ReduceMotion displays immediately without fade. Static initial documents display at once.

`Locale: "zh-CN"` selects bundled Chinese labels; English is the default. `Labels` overrides individual English keys. Standalone File/Compare/RichView options also accept Labels. MyGo's native editor/platform menus follow its platform localization.

## Node model and parser hooks

```go
parser := ms.NewParser(0)
nodes := parser.Append("Hello **wor", false)
nodes = parser.Append("ld**", true)
// UI frame:
ms.View(c, "", ms.Props{Nodes: nodes})
```

`Parser` owns an incremental syntax document and Markstream's node cache. Set replaces source; Append adds a chunk; `final` controls strict parsing. `ParseNodes` is the one-shot helper. Treat returned nodes as immutable; parser mutation is single-goroutine state. Synchronize publication to a UI goroutine through application state. Nodes contain Type, ID, Raw/Content, byte ranges, Loading, language/info, URL/title, Attributes and Children. Stable block IDs survive offset edits and prepends; supply stable unique IDs for custom node input when possible.

`Props.Nodes` bypasses UI parsing. Pass matching source as the View argument for source-preserving clipboard output. Footnote definitions in the node array are collected globally. Without source, raw node strings form a fallback source.

Pipeline hooks:

- `SourceTransform(string) string` runs before parsing, cached by source/ParseRevision.
- `DocumentTransform(*Document)` runs when the low-level document changes.
- `TransformNodes([]*Node) []*Node` receives a writable tree copy after node conversion.
- `NodeRenderers map[string]func(*ui.Context, *Node)` overrides block or inline types in this view only.
- `CustomHTML map[string]func(*ui.Context, *Node)` maps lower-case tags to native controls. Isolated custom tags are block controls; inline callbacks must build native text elements. Attributes are parsed, not executed.
- `OnTaskToggle(*Node, bool)` enables task checkboxes; the application owns source updates.
- `Block(c, document, index) bool` is a lower-level block override; true consumes the block.

For example:

```go
props.CustomHTML = map[string]func(*ui.Context, *ms.Node){
    "status-card": func(c *ui.Context, n *ms.Node) {
        ui.Column(c).Padding(12).Children(func() {
            ui.Text(c, n.Attributes["title"]).Bold()
        })
    },
}
```

`:::` containers expose Label, Title, Attributes and children; JSON metadata is normalized to Attributes, and remaining arguments are available as `args`. Node types follow the native model (e.g. `quote`, `item`, inline `code`/`math`, and `code_block`/`math_block`). They are not TypeScript AST aliases.

For advanced syntax manipulation, `NewDocument`, `Parse`, `Document` and `Extensions` expose the low-level MyGo parser dependency. Defaults include GFM, footnotes, math/dollar math and alerts. Document block/inline handles expire at the next mutation. `Props.Document` accepts a caller-owned document; synchronize its mutation yourself.

## Code, File and Diff

```go
props.CodeOptions = &ms.CodeOptions{
    Collapsible: true, ShowFontSize: true, MaxHeight: 360,
    Filename: ms.FenceFilename,
    File: &ms.FileOptions{Wrap: true, TabSize: 4},
    HeaderRight: func(c *ui.Context, b *ms.CodeBlock) {
        if ui.Button(c, "Use snippet").Clicked() { use(b.Code()) }
    },
}
```

Code supports language/name headers, copy, collapse, bounded height, wrap or horizontal scrolling, font controls and header builders. `FenceFilename` recognizes title/filename metadata without reading files. `PlainCode` omits the header. Copy waits while a fence is open; strict EOF releases an unclosed final fence.

Override order: per-language `Code` hook (closed/final fences unless RenderOpenCode), general `CodeBlock`, configured rich language renderer, then PlainCode/RenderCode. A `CodeBlock` supplies Code(), Body(), NoteElement() and CopyButton(). Use `RenderCode` for fallback. Expensive work belongs outside synchronous UI callbacks.

Standalone surfaces:

```go
ms.File(c, ms.FileData{Name: "main.go", Language: "go", Content: source},
    ms.FileOptions{MaxHeight: 400, TabSize: 4})
ms.FileDiff(c, unifiedPatch, ms.FileOptions{DiffMode: ms.Split})
ms.Compare(c, oldFile, newFile, ms.CompareOptions{
    FileOptions: ms.FileOptions{DiffMode: ms.Split, HideUnchanged: true},
    Inline: editor.DiffInlineWord,
    LineSelection: true,
    OnLineSelection: func(r editor.DiffLineRange) { /* application action */ },
})
```

File caches tokens by unchanged prefix and builds visible lines. FileOptions includes font/line metrics, code palette, gutter visibility, OnLineClick and Annotation. Tab expansion retains syntax token positions. FileDiff accepts Git/unified/multi-file or loose `-old/+new` patches. Set Streaming for incomplete input. Final patches retain original hunk line offsets in the native editor.

Compare additionally configures word/character/none inline diffs, indicators, tabs, background, context expansion, line selection, gutter utilities, annotations and a Controller callback exposing editor.DiffState. The native editor's line ranges use its documented zero-based indexing. FileOptions.Compare configures these capabilities for final patch fences. MyGo editor primitives supply native Diff layout; no Markdown widget is involved.

## Rich engines and asynchronous graphics

```go
engine := rich.New(rich.Options{BrowserPath: chromePath})
defer engine.Close() // when the application exits
props.Rich = &ms.RichOptions{
    Renderer: engine, Progressive: true, MaxHeight: 500,
    Config: map[string]map[string]any{
        "math": {"macros": map[string]string{`\RR`: `\mathbb{R}`}},
        "mermaid": {"themeVariables": map[string]string{"primaryColor": "#ddd6fe"}},
    },
    OnExport: func(e ms.Export) { save(e.Kind, e.Format, e.Data) },
    OnRender: func(s ms.RichRenderState) { report(s.Err) },
}
```

Use one retained engine for the application. `rich.Options` supports BrowserPath, Timeout, CacheEntries, CacheBytes and container-only NoSandbox. Renderer assets/fonts are embedded. Runtime Chromium performs background layout; no Node/package install or network asset fetch is needed.

Kinds: math, inline-math, mermaid, d2, infographic, html and svg. Fences tex/latex/math select KaTeX. Block and inline math also use it. RichOptions.Config selects per-kind engine configuration; do not mutate configuration maps while work is active. Trust, sanitization, sandbox policy and resource limits remain enforced. `RichView` exposes the same native graphic controls independently of Markdown; its RichRequest.Config supplies options directly.

The worker serializes browser work and caches immutable results. View requests coalesce while busy. Invalid progressive input preserves the last valid preview and its matching export source. Theme or non-prefix replacements discard incompatible previews. A retry control handles failures. Exports use actual SVG/PNG; without OnExport the native Save dialog opens. PNG works for every kind; SVG requires an SVG-producing engine.

Built-in bounds: 1 MiB request source, width ≤4096, graphics ≤4096×16000, PNG ≤16 MiB; default cache 128 entries/64 MiB. KaTeX disallows trusted commands and limits expansion/size; HTML/SVG is sanitized with DOMPurify and worker network policy restricts asset access to loopback. This is a rendering process, not an arbitrary browser scripting API.

A custom RichRenderer can replace the backend. Math/InlineMath/HTML callbacks override bundled behavior; InlineObject places a correctly sized/baselined object in native text. Diagram/HTML graphics are bitmaps with source/export controls, rather than selectable DOM text. Safe links extracted by the engine are exposed as native links.

## Images, selection and anchors

Images load off the UI thread by default, with HTTP(S), image data URLs and SVG support. ImageOptions configures BaseURL, Client, MaxBytes and AllowURL (checked on every redirect). Defaults bound downloads to 16 MiB and decoded rasters to 32 megapixels. `LoadImage` replaces loading; `DisableImages` shows descriptions. SVG uses a configured RichRenderer for full layout or MyGo's native SVG subset. Keep custom loaders bounded; they do not receive a cancellation context. Image loading and rich work count toward current mounted-view completion.

`OnLink` receives only SafeLink-approved targets. The default opens web/mail/relative targets through MyGo; hash links scroll native heading anchors. `Anchor`/`AnchorVersion` requests heading navigation. Footnotes preserve document-global definitions through virtualization.

Native mounted text can be selected across paragraphs. Normal Copy offers plain text, HTML and Markdown representations. Copy Markdown in the context menu or Cmd/Ctrl+Shift+C puts the selected Markdown on the plain-text clipboard. Partial selections preserve inline formatting; whole paragraphs preserve source. `ShareSelection` delegates to an outer selectable scope. Virtualized/unmounted text is not part of a native selection.

## Virtual documents and messages

```go
virtual := &ms.VirtualState{} // application state
// UI frame, with a bounded filling viewport:
ms.View(c, source, ms.Props{Virtual: virtual, MeasurementKey: "my-layout-v1"})
// Application actions:
virtual.ScrollTo(1200, ui.Start)
snapshot := virtual.Capture() // JSON serializable
virtual.Restore(snapshot)
metrics := virtual.Metrics()
```

VirtualState owns a variable-height native list and tracked scroll state. Only visible nodes plus overscan are built; parsing and global reference lookup still cover the document. Stable keys preserve scroll anchoring during appends/prepends. `List.FollowEnd` enables follow-to-end.

Metrics distinguish NodeCount/RenderedCount, measured/estimated nodes, visible range, Width, estimated TotalHeight and Final/Stable/Phase. Capture records source hash, width, layout key, anchor/offset and signature-bound measured heights. Restore validates source/width/key **before** moving the viewport; incompatible snapshots are ignored. Use MeasurementKey for custom component geometry/font changes not expressed by the built-in theme. Rich result revisions invalidate measurements. A virtual list does not pre-render offscreen diagrams just to fire OnFinal.

Without explicit Virtual, views automatically virtualize above MaxLiveNodes (default 220) using the window height. DisableVirtualization opts out. Explicit Virtual is preferable when composing multiple panes or nested scrolling.

For conversation history, retain `ui.ListState` and `[]Message{ID, Source, Session}` and call `Messages(c, state, messages, props).Fill()`. Session takes precedence over Source. IDs must be stable and unique. Native follow-end pauses when the user scrolls away. Use list ScrollTo/ScrollToEnd for navigation. Selection covers mounted text; huge open paragraphs still require open-tail parse work.
