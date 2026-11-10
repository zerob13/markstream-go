# API guide

## Views and identity

```go
el := markstream.View(c, source, markstream.Props{Streaming: receiving})
shown := markstream.Revealed(el)
```

`View` receives the full source-so-far, not just the new chunk. MyGo's incremental document detects append/edit/replacement and caches completed content. Set `Streaming: false` at EOF, in the same keyed position. This restores strict final syntax while draining reveal animation. Repeated controls need `c.Key(id)` immediately before the view.

Use an element only in the frame that built it. `Revealed` must run in that frame. Completion of a network request alone does not mean reveal animation has completed.

## Concurrent sessions

`Session` implements `io.Writer`. Its zero value is usable; `NewSession(maxBytes)` adds an optional byte limit. Concurrent writes are serialized in lock-acquisition order, so one producer per logical response is usually appropriate. Do not copy a session after first use.

```go
reply := markstream.NewSession(1 << 20)
// Background producer:
_, err := reply.Write(chunk)
if err != nil {
    reply.Fail(err)
}
// UI thread:
el := markstream.Stream(c, reply, markstream.Props{})
finished := reply.Snapshot().Done() && markstream.Revealed(el)
_ = finished
```

`Stream` subscribes with the window's thread-safe invalidation service. The subscription is detached when the view is removed or assigned a different session. Several writes before a snapshot produce one invalidation; snapshot retrieval acknowledges those writes. It does not parse, copy the accumulated source, or build UI. Previously returned strings remain immutable and valid after subsequent writes or resets.

| Operation | Behavior |
| --- | --- |
| `Append(string)` / `Write([]byte)` | Accept a chunk while receiving; hold an incomplete final UTF-8 rune until its remaining bytes arrive |
| `Finish()` | Complete the reply; incomplete final UTF-8 fails with `ErrInvalidUTF8` |
| `Cancel()` | Preserve complete source and enter `Canceled` with `context.Canceled` |
| `Fail(err)` | Preserve complete source and enter `Failed`; a nil error does nothing |
| `Reset()` | Clear source/error, enter `Receiving`, increment the generation, retain subscriptions and byte limit |
| `Snapshot()` | Return source, state, error, revision, generation and buffered-byte count |
| `Consume(ctx, reader)` | Read raw Markdown to EOF, then finish; preserve partial output on errors |

Invalid input and byte-limit failures reject an entire write with `(0, error)` and leave its existing content intact. The byte limit includes buffered partial UTF-8. Call `Fail(err)` to turn a direct write rejection into a terminal failure. Writes after finalization return `ErrClosed`.

`Consume` captures its generation. A reset while it is reading causes old data to be rejected with `ErrReset`; the old producer cannot append to or finish the new reply. This protection applies to `Consume`; application-managed producers using `Append` must be canceled/joined before a reset, or use a new `Session` per reply.

### HTTP and model transports

```go
req, err := http.NewRequestWithContext(ctx, http.MethodGet, markdownURL, nil)
if err != nil { return err }
resp, err := client.Do(req)
if err != nil { return err }
defer resp.Body.Close()
if resp.StatusCode != http.StatusOK {
    return fmt.Errorf("Markdown request: %s", resp.Status)
}
return reply.Consume(ctx, resp.Body)
```

`Consume` accepts **raw Markdown**, not SSE wire framing or provider JSON. For model APIs, decode each event with your provider's SDK, append only its text delta, then call `Finish` at the provider's completion event. On cancellation/error call `Cancel`/`Fail`. Do not append `data:`, `[DONE]` or JSON wrappers as Markdown.

Cancellation is checked between reader operations. A blocked arbitrary `io.Reader.Read` cannot be interrupted by a context alone: use an HTTP request with the same context, or close the reader from its owner. The session does not close a reader it does not own.

## Reveal, themes and layout

```go
props := markstream.Props{
    Variant: markstream.ChatStyle,
    Theme: &markstream.Theme{
        FontSize: 15,
        Link: ui.Hex("#7c3aed"),
        CodeFont: "monospace",
    },
    Reveal: &markstream.Reveal{
        Unit: markstream.ByWord,
        CatchUp: 120 * time.Millisecond,
        Fade: 180 * time.Millisecond,
    },
}
```

The default is 16 DIP document typography, character reveal, a 120 ms catch-up and 180 ms fade. `ChatStyle` uses tighter 14 DIP text. Colors follow MyGo's window theme. An explicit `Theme` replaces the variant defaults; zero fields are resolved by MyGo's Markdown theme. MyGo also respects reduced-motion preferences.

Units: `ByCharacter`, `ByWord`, `BySentence`, `ByLine`, `ByParagraph`, `AtOnce`. Set `Fade: -1` to disable fade. `ByLine`/`ByParagraph` can hold incomplete units until EOF; always finalize. `Reveal` and theme pointers should remain stable when possible.

## Code blocks and custom renderers

```go
props.CodeOptions = &markstream.CodeOptions{
    Collapsible: true,
    DefaultCollapsed: false,
    MaxHeight: 360,
    Filename: markstream.FenceFilename,
}
props.WrapCode = true
```

`FenceFilename` reads `title=main.go` or `filename="hello world.go"` metadata; it never accesses files. `PlainCode` omits the enhanced header. Unknown languages remain readable code. The body uses MyGo's tokenizer and 32-line cached text chunks. The default copy action copies the entire code block, even when collapsed.

Per-language renderers use lowercase keys:

```go
props.Code = map[string]func(*ui.Context, *markstream.CodeBlock){
    "notice": func(c *ui.Context, block *markstream.CodeBlock) {
        ui.Column(c).Padding(12).Background(ui.Hex("#ede9fe")).Children(func() {
            ui.Text(c, block.Code())
        })
    },
}
```

A language hook runs after its closing fence arrives, or when receiving ends (including an unclosed final fence). Until then the regular code renderer stays visible. `RenderOpenCode` opts into incomplete input. A general `CodeBlock` callback handles fallback languages. `RenderCode(c, block, options)` is available to custom renderers that need the library's code UI. Callback precedence is language hook → general `CodeBlock` → plain/code-options/default surface.

Hooks are synchronous UI builders: cache expensive results and perform I/O off the UI thread. They may run more than once a frame due to layout. Use `block.Open`, `Language`, `Info`, `Index`, `Code()`, `Body(c)` and `CopyButton(c)` as needed. Do not retain a code-block handle after that frame.

Further `Props` hooks:

- `Block(c, document, index) bool`: replace any parsed block; return false for native rendering.
- `Math(c, tex)` / `InlineMath(tex, size)`: native display/inline math. A missing renderer falls back to readable TeX.
- `HTML(c, source)`: render HTML blocks with native controls. By default raw blocks are text, not an executable browser surface.
- `LoadImage(url)`: asynchronously load images through MyGo. Without it image descriptions remain visible. You control network destinations, size limits and local-file policy.
- `OnLink(url)`: custom link routing guarded by `SafeLink`. The default MyGo handler supports safe web/mail links and anchors. Custom callbacks do not receive executable/local URL schemes.
- `Anchor` / `AnchorVersion`: scroll to a heading, and request it again when the version changes.
- `ShareSelection`: share a surrounding native selectable container's selection.

`SafeLink` rejects control characters, backslash ambiguity, protocol-relative links and executable/local schemes. It is a routing filter, not a network sandbox. Hosts should still apply their own external-link and image policies.

## Virtualized conversation

```go
// Keep state and messages in application state, not inside the frame function.
state := &ui.ListState{FollowEnd: true}
messages := []markstream.Message{
    {ID: "intro", Source: "Welcome **back**."},
    {ID: "reply-1", Session: reply},
}
// Inside a UI frame:
markstream.Messages(c, state, messages, markstream.Props{
    Variant: markstream.ChatStyle,
}).Fill()
```

Message IDs must be unique, stable, comparable strings. `Session` takes priority over `Source`. MyGo builds only visible rows plus a small overscan; it measures variable heights and anchors the visible row as heights change. `FollowEnd` starts at the bottom, follows new/growing content, pauses when the user scrolls away and resumes when they return. Use `state.ScrollTo(index, ui.Start)` / `ui.End` to navigate.

This is **message-level virtualization**. A single large Markdown message still builds its block tree; completed parse/highlight/layout caches reduce work, but the library does not claim block-level virtualization or cross-message selection over unmounted rows.

## Parser access

`NewDocument(extensions)` returns MyGo's incremental `Document`; zero selects `DefaultExtensions` (GFM, footnotes, math including `$...$`, alerts and GitHub references/mentions). `Parse(source)` is the one-shot convenience. Extensions can be restricted, for example with `GFM`.

```go
doc := markstream.NewDocument(0)
doc.SetStreaming(true)
doc.Append("Hello **wor")
doc.Append("ld**")
doc.SetStreaming(false)
```

Use `Set` for a replacement or `Edit` for a byte-range edit. `Children`, `Block`, `Inlines`, `Lines` and `Source` expose the tree. Mutate a document on one goroutine. Block pointers/indices are valid until the next mutation. `Session` owns concurrent transport; it does not expose a mutable parser across goroutines.
