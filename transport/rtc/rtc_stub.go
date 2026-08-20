//go:build js || wasip1

// Package rtc is the WebRTC implementation of api:transport-interface.
//
// On js/wasm the native pion stack cannot build; the browser's own
// WebRTC API takes this role there, behind the same transport.Conn
// interface, via a JS bridge that arrives with the wasm client work.
// This stub keeps the package present on every target
// (rule:build-tag-only-for-linkage — the tag changes linkage, never
// behavior).
package rtc
