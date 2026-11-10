# Contributing

Use Go 1.27.3+ and the pinned MyGo dependency. Keep Markstream's node/renderer ownership: do not introduce `github.com/egoist/mygo/ui/markdown` widget imports or copied widget implementations. The low-level parser and native UI/editor primitives are intentional dependencies.

1. Run `go mod download`.
2. Make focused changes and test observable rendering, lifecycle or performance behavior.
3. Run `gofmt`, `go vet ./...`, `go test ./...` and `go test -race ./...`.
4. Run `go run ./examples/demo -smoke`; inspect native software-rendered snapshots for UI changes.
5. With Chromium installed, set `MARKSTREAM_REQUIRE_CHROME=1` and run all tests. Set `MARKSTREAM_CHROME_PATH` for a nonstandard executable. Use `MARKSTREAM_CHROME_NO_SANDBOX=1` only in isolated test containers that cannot run the browser sandbox.
6. Commit focused changes with `feat:`, `fix:`, `test:` or `docs:` prefixes.

Background work must not block UI builders. Session supports concurrent transport; parser mutation and native builds are single-goroutine operations. Treat parsed-node snapshots as immutable; transforms receive copies. Do not retain frame-local UI elements or CodeBlock handles.

## Offline rich assets

Runtime users need Go and (for rich content) Chromium, not Node. Rebuild committed assets with a supported Node version (tested with Node 24):

```sh
npm ci --prefix tools/renderer
npm run build --prefix tools/renderer
go test ./rich
```

Exact engine/build versions and integrity values are locked. Commit source, lockfile and generated assets together. Preserve notices in `rich/assets/THIRD_PARTY.txt`, generated legal-comment assets, `rich/LICENSE.d2` and THIRD_PARTY.md. Do not disable sanitization or silently relax worker limits for a syntax fixture. Configuration changes need actual engine output tests, not a hook-only test.

Document reproducible performance fixtures and distinguish startup/parse/steady-layout/GPU costs. Keep docs/parity.md aligned with behavior and native platform boundaries. Generated binaries and test artifacts belong in ignored `artifacts/`; documentation PNGs are intentionally tracked.
