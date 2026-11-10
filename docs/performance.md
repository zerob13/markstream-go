# Performance and validation

## Work avoided

- Session snapshots reuse immutable source; coalesced notifications wake a window once for a burst of writes.
- Low-level parsing appends from the open outer block and retains completed inline caches. Markstream projects its own cached nodes, invalidating late references and preserving stable IDs through edits/prepends.
- Native File caches unchanged tokenized lines. Diff uses virtual native editor/list primitives.
- A virtual document builds visible blocks plus overscan, while retaining global references/footnotes. Position and measured heights can be serialized with source/width/layout validation. Message histories virtualize each reply separately.
- Rich graphics run off the UI thread in one shared browser worker. Requests coalesce while busy, successful outputs are bounded/cached, and invalid prefixes retain the previous valid preview.

The complete AST is retained. An arbitrarily long open paragraph can need reparsing; node projection is not constant work per token. Height totals include estimates for unmeasured blocks. Custom synchronous UI callbacks, fonts and image/graphic layout can dominate cost. A virtual viewport does not pre-render offscreen graphics.

## Reproduce

```sh
go test -run '^$' -bench . -benchmem -benchtime=20x -count=1
```

Sample measured with Go 1.27.3, Linux amd64, Intel Xeon Platinum 8573C, four available Go processors:

| Benchmark | Time / operation | Bytes allocated | Allocations |
| --- | ---: | ---: | ---: |
| Incremental low-level document | 0.458 ms | 362,703 | 270 |
| Reparse every received prefix | 13.978 ms | 15,953,397 | 12,028 |
| Native steady frame | 1.113 ms | 115,154 | 899 |
| 5,000-node virtual document frame | 0.779 ms | 32,520 | 308 |

The parser fixture repeats the mixed corpus 20 times (6,520 bytes), delivered in 64-byte chunks; each operation processes a whole reply. The incremental comparison measures block syntax parsing, not the entire Markstream renderer. The native steady fixture repeats the corpus five times in an 800×700 scroll view. The virtual fixture is 2,500 headings plus paragraphs in an 800×600 viewport. Its initial parse is outside the steady-frame timer. These native frame measurements include software-test build/layout, **not GPU frame times** or graphics-engine startup. This is a 20-iteration development sample, not a hardware-independent guarantee.

## Verified behavior

The independent renderer and demo have been exercised with:

- `go test -count=1 ./...`, including actual Chromium rich-engine integrations.
- `go test -race -count=1 ./...`, covering session concurrency, native interactions, worker cancellation and all demo pages.
- `go vet ./...`, gofmt and diff whitespace checks.
- Linux amd64 build and cgo-free macOS arm64/Windows amd64 cross-builds.
- Native software-rendered light/dark playground, rich gallery, FileDiff and large-document screenshots.
- Mixed CommonMark/GFM comparison at every byte split, UTF-8 split delivery, strict EOF, stale producer/reset rejection and notification coalescing.
- 4,000+/5,000-node virtual documents, bounded visible builds, shared references/footnotes, middle save/restore, incompatible-width rejection, prepend identity and anchor preservation.
- Partial text/HTML/Markdown clipboard formatting, native code copying/folding, stable folded-diff height/layout at final handoff, custom inline tags/attributes, Chinese labels, visible fade and actual inline-object pixels.
- Actual KaTeX display/inline math and macros, D2/Infographic SVG and nonblank PNG output, HTML sanitization, cancellation, immutable result caches and native source/fullscreen/export controls.
- Mermaid flowchart, sequence, class, state, ER, Gantt, pie, Git, mindmap, timeline, XY, sankey, block and journey fixtures.

The rich test suite skips if Chromium is absent, unless `MARKSTREAM_REQUIRE_CHROME=1` makes that a failure. CI's dedicated rich job installs Chrome and sets this requirement. Native core tests never require a display.

This cloud host has no X11/Wayland display. Actual OS-window startup was attempted and could not open a display; component controls and layout were verified through MyGo's real software renderer. Cross-compilation is not runtime testing on macOS/Windows. Hosted CI results must be checked on GitHub; the presence of a workflow does not assert that those runners have already passed.
