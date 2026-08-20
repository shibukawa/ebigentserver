//go:build js || wasip1

// Package wt is the WebTransport implementation of api:transport-interface.
//
// On js/wasm the native QUIC stack cannot build; the browser's own
// WebTransport API takes this role there, behind the same
// transport.Conn interface, via a JS bridge that arrives with the wasm
// client work. This stub keeps the package present on every target
// (rule:build-tag-only-for-linkage — the tag changes linkage, never
// behavior).
package wt
