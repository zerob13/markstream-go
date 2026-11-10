# Changelog

## 0.2.0 — 2026-11-10

First full native capability implementation on `main`; this entry does not create a release tag.

- Replace the initial Markdown-widget wrapper with Markstream's independent nodes, renderer, reveal and virtual document implementation. Reuse only low-level syntax and native UI/editor primitives.
- Add immutable parsed-node input, stable identities across edits/prepends, parse transforms, scoped inline/block overrides and custom HTML/container attributes.
- Add definitions, nested containers/details, mark/insert/sub/sup, emoji and footnote round-trip navigation.
- Add actual grapheme-aware reveal/fade, reduced motion and async-aware settled/final events.
- Add single-document block virtualization, global references/footnotes, variable heights and serializable validated position caches.
- Add native File/FileDiff/Compare, streaming patch fallback, final editor adaptation, split/unified and word/character diffs, context expansion, selections, gutter actions and annotations.
- Bundle offline Mermaid/KaTeX/D2/Infographic/DOMPurify assets, fonts and licenses. Optional background Chromium layout provides real diagrams/math/HTML, progressive recovery, native zoom/pan/fullscreen and SVG/PNG exports.
- Add bounded image loading/SVG, native image previews, formatting-aware selected Markdown/HTML copying and localized controls.
- Expand the native demo to five pages and test real graphics alongside software-rendered native controls, pixel output, race safety and cross-platform builds.
- Rewrite API, compatibility and performance documentation around the independent implementation.

## 0.1.0 — 2026-11-10

Initial transport and native streaming workflow: concurrent UTF-8 sessions, lifecycle, code controls, message virtualization, native demo, tests, CI and MIT license. The initial widget-based UI is superseded by 0.2.0.
