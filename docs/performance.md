# Performance and validation

## Work avoided

- `Session` accumulates text in a growing builder and returns immutable string snapshots without copying the complete response on each frame.
- Writes are serialized, and a burst before a snapshot produces one window invalidation. Views read the latest accumulated source once they build.
- MyGo's parser appends/reparses from its open outer block and maintains completed-block inline caches. A finalization pass restores strict syntax.
- MyGo highlights changed code lines and retains 32-line text chunks. Idle frames reuse parse, token and text-layout state.
- `Messages` delegates to MyGo's virtual list: only visible messages plus overscan are built. Native variable-height measurement and anchoring handle growing replies.

There is no claim that arbitrarily long open paragraphs take constant work: the open tail can need reparsing. Neither does message virtualization make one huge message block-virtualized. Hooks and image loaders can dominate frame cost; keep synchronous UI callbacks cheap.

## Reproduce

```sh
go test -run '^$' -bench . -benchmem -benchtime=3x -count=1
```

The parser fixture is the mixed Markdown corpus in `document_test.go`, repeated 20 times (6,520 bytes). Chunks are 64 bytes. Both parser benchmarks process a whole reply: one reuses an incremental document, the other reparses every received prefix. These measure block parsing/allocation, not a full browser or native app workload. Inline layout is exercised separately by the native frame benchmark.

One development-machine sample, Go 1.27.3, Linux amd64, Intel Xeon Platinum 8573C, 4 available Go processors:

| Benchmark | Time / operation | Bytes allocated | Allocations |
| --- | ---: | ---: | ---: |
| Incremental document | 0.583 ms | 362,696 | 270 |
| Reparse every prefix | 47.823 ms | 15,955,504 | 12,034 |
| Native steady frame | 0.883 ms | 14,928 | 186 |

The native fixture repeats the same corpus five times in an 800×700 MyGo scroll view. The software test renderer builds/layouts frames; these numbers are **not GPU frame times**. This is a three-iteration sample, not a statistically robust performance guarantee. CPU load, fonts, viewport, source and syntax change results. Re-run on the target hardware before setting budgets.

## Readiness checks

The initial implementation was validated locally with:

- `go test ./...` including native component interactions and demo playback.
- `go test -race -count=1 ./...`, including concurrent session writers and native demo tests.
- `go vet ./...`.
- Linux amd64 build; `CGO_ENABLED=0` cross-builds for macOS arm64 and Windows amd64.
- `go run ./examples/demo -smoke` plus actual native software-rendered light/dark PNGs.
- A parser differential test at **every byte split** of the mixed-syntax corpus, with forced inline materialization before the remaining input arrives and final strict comparison.

The tests also exercise incomplete syntax, late reference definitions, UTF-8 split codepoints, source replacement, reset while reading, cancellation/errors, byte limits, callback coalescing, code copy/folding, unsafe links, subscription cleanup and a 1,000-message virtual list.

Desktop startup was attempted in the cloud machine and reported no `DISPLAY`/`WAYLAND_DISPLAY`. The native software renderer, controls and layout run without a display; opening an OS window was therefore not validated in that machine. Cross-compilation does not establish runtime validation on macOS or Windows. The CI configuration runs tests on those operating systems when GitHub Actions executes it; writing the workflow is not an assertion that hosted CI has run.
