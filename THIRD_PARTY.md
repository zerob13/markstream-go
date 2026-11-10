# Third-party software and design references

- **MyGo**, https://github.com/egoist/mygo, MIT, copyright EGOIST and contributors. Imported through Go modules, including its native UI, editor highlighting and incremental Markdown parser. Its license is distributed with the module; see `go.mod` / `go.sum` for the exact version.
- **markstream-vue**, https://github.com/Simon-He95/markstream-vue, MIT, copyright Simon He and contributors. Architectural reference for streaming Markdown, stable completed blocks, progressive rendering and extension fallback. No Vue or parser source was copied into this repository.
- **Lobe MyGo**, https://github.com/lobehub/lobe-mygo, MIT, copyright LobeHub and contributors. Architectural reference for Go-native UI components and a native catalog. No Lobe component source was vendored.

Transitive dependencies retain their respective licenses. `go list -m all` lists the resolved modules. The MyGo module depends on packages including purego, go-text/typesetting, x/image and x/text; application builds additionally include MyGo's platform and media dependencies.
