# A reply, as it arrives

A **native Go component** for streaming Markdown. Every token stays readable — even while a sentence, link or code fence is still being written.

> [!TIP]
> Completed blocks keep their parse and layout caches. Only the open tail needs more work.

## A small example

```go
reply := markstream.NewSession(1 << 20)

// In a background goroutine:
reply.Append("Hello **世界**")
reply.Finish()

// In your MyGo view:
markstream.Stream(c, reply, markstream.Props{})
```

## Made for conversation

- [x] Incremental CommonMark and GFM
- [x] UTF-8 safe chunks, including 中文 and 🙂
- [x] Selectable text and code copying
- [x] Light and dark themes
- [x] Virtualized message history
- [ ] Connect your model's streaming API

| Surface | While receiving | At completion |
| :--- | :--- | :--- |
| **Text** | Smooth reveal | Drain queued words |
| Code | Highlight growing lines | Copy the whole block |
| Custom fences | Readable code | Invoke native renderer |

## Useful details

A [safe link](https://go.dev), `inline code`, ~~an old idea~~, and a footnote[^1].

1. Create a session.
2. Append chunks from your transport.
3. Finish at EOF; cancel or fail on interruption.

---

[^1]: No HTML, JavaScript, browser or webview is needed for the native component.
