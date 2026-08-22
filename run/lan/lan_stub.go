//go:build js || wasip1

// Package lan is the LAN preset of api:lan-preset.
//
// It rests on api:lan-discovery and on a listening socket, and a browser
// has neither: no udp broadcast to find anybody with, and no way to
// accept a connection. A wasm build reaches other players through
// concept:static-host-mode instead — system:webrtc with an
// api:manual-signaling-token. This stub keeps the package present on
// every target (rule:build-tag-only-for-linkage — the tag changes
// linkage, never behavior).
package lan
