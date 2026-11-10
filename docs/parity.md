# Compatibility with markstream-vue

This first version provides the core streaming Markdown workflow through native MyGo controls. It is not a port of Vue components or a claim that every optional browser integration has a native equivalent.

| Capability | markstream-go 0.1 implementation |
| --- | --- |
| CommonMark / GFM | MyGo native parser and UI: headings, emphasis, lists, tasks, tables, quotes, links, images and code |
| Footnotes / alerts | Native rendering |
| Streaming incomplete inline syntax | Parser-based healing of the open tail; strict finalization |
| Incremental updates | MyGo incremental parse/edit with finished-block and text/highlight caches |
| Concurrent chunk delivery | Thread-safe `io.Writer` session; split UTF-8, limits, snapshots and coalesced notifications |
| Completion / cancel / reset / error | Explicit lifecycle; old `Consume` generations cannot corrupt a reset reply |
| Smooth reveal | Character, word, sentence, line, paragraph or immediate; EOF drains pending reveal |
| Syntax highlighting | MyGo native editor tokenizers; unknown languages use plain text |
| Code header / copy / wrap / folding | Native language/filename header, full-block copy, max height, wrap and collapse |
| Text selection / clipboard | Native MyGo selection, text/HTML/Markdown clipboard formats |
| Theme | Window light/dark colors plus custom Markdown theme and document/chat typography |
| Large conversation | Native variable-height virtualized message list, stable IDs, scroll anchoring and follow-end |
| One huge document | Incremental parse and cached block/line layout; **no block-level virtualization** |
| Links / anchors | Native safe links and custom guarded routing; heading anchors |
| Images | Native loader hook; image descriptions until a host supplies loading |
| Custom blocks / code languages | Native callbacks; closed-fence gating and readable fallback |
| Math syntax | Parsed, with TeX fallback and native display/inline rendering hooks; **no bundled TeX layout engine** |
| Mermaid / D2 / infographic | Readable code fallback and per-language renderer hooks; **no bundled graph engines** |
| HTML | Native inline formatting subset; raw HTML blocks displayed as source unless overridden; **no arbitrary DOM** |
| File/FileDiff / stream-diffs UI | **Not implemented**; diff fences use the code surface |
| Multiple frontend frameworks / browser workers / SSR | Not applicable to a Go-native MyGo component |

MyGo supplies a substantial incremental Markdown implementation already. Reusing it preserves its CommonMark/GFM behavior, selection integration, code chunking and native accessibility. The new library concentrates on an ergonomic session lifecycle, native component APIs, virtualized conversations, reusable code controls and validation.

Optional rich renderers are deliberate host extension points. A hook accepting a language is not equivalent to providing that language's renderer. Never present source fallback as rendered diagrams or fully typeset mathematics.

Potential follow-up work: measured block virtualization for a single huge message; native TeX/diagram packages with explicit syntax coverage; a native File/FileDiff component. These are not prerequisites for running the documented core demo and are not represented as completed features.
