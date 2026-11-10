// Package markstream provides independent native MyGo components for streaming
// Markdown. Session is the concurrent UTF-8 transport; Parser owns incremental
// syntax and immutable Markstream nodes; View and Stream build native controls.
// VirtualState virtualizes individual documents, while Messages virtualizes
// conversations. File, FileDiff and Compare expose native code/diff surfaces.
//
// The optional rich subpackage provides offline math/diagram/HTML engines using
// a background Chromium process. The application window has no WebView.
package markstream
