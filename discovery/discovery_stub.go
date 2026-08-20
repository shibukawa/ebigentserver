//go:build js || wasip1

// Package discovery implements api:lan-discovery.
//
// Browsers cannot send or receive UDP broadcast, so this package is
// native-only; a wasm build discovers sessions through
// concept:static-host-mode (a rendezvous capability such as an
// api:manual-signaling-token) instead. This stub keeps the package
// present on every target (rule:build-tag-only-for-linkage — the tag
// changes linkage, never behavior).
package discovery
