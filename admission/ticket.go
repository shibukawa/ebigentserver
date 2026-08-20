// Package admission implements flow:session-admission: a client joins a
// game session by presenting a short-lived signed data:session-ticket in
// its first message, and the process verifies it locally — no control
// plane on the connection path (rule:local-ticket-verification).
//
// Tickets are JWTs signed with Ed25519 (rule:asymmetric-ticket-signature:
// eddsa or es256, never a shared symmetric secret — in listen-server and
// p2p topologies the verifier is a player machine, where a symmetric
// secret would be a forgery key for every session). The implementation
// here is a deliberately minimal EdDSA-only JWT: header {alg,kid,typ},
// registered claims, detached from any negotiation.
package admission

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Claims is the ticket body: JWT registered claims plus the game claims
// of data:session-ticket.
type Claims struct {
	// Subject is the player id.
	Subject string `json:"sub"`
	// Audience is the session endpoint that may accept the ticket.
	Audience string `json:"aud"`
	// ExpiresAt is Unix seconds; keep it short (seconds to a couple of
	// minutes).
	ExpiresAt int64 `json:"exp"`
	// ID is the jti, redeemable once per session.
	ID string `json:"jti"`
	// SessionID names the session this ticket admits into.
	SessionID string `json:"session_id"`
	// Seat is the agent slot the holder may occupy
	// (permission:agent-seat-control).
	Seat uint16 `json:"seat"`
	// Role is player, spectator, or observer.
	Role string `json:"role"`
}

type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

var b64 = base64.RawURLEncoding

// Sign issues a ticket. The private key lives only with the issuer
// (concept:control-plane); kid names the key so rotation keeps live
// sessions verifiable.
func Sign(priv ed25519.PrivateKey, kid string, claims Claims) (string, error) {
	if claims.ExpiresAt == 0 || claims.ID == "" {
		return "", errors.New("admission: ticket needs exp and jti")
	}
	h, err := json.Marshal(header{Alg: "EdDSA", Typ: "JWT", Kid: kid})
	if err != nil {
		return "", err
	}
	c, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signing := b64.EncodeToString(h) + "." + b64.EncodeToString(c)
	sig := ed25519.Sign(priv, []byte(signing))
	return signing + "." + b64.EncodeToString(sig), nil
}

// Verification errors; all of them fail admission closed.
var (
	ErrMalformed  = errors.New("admission: malformed ticket")
	ErrUnknownKey = errors.New("admission: unknown key id")
	ErrSignature  = errors.New("admission: signature invalid")
	ErrExpired    = errors.New("admission: ticket expired")
	ErrAudience   = errors.New("admission: ticket for another endpoint")
	ErrReplayed   = errors.New("admission: jti already redeemed")
)

// Verifier checks tickets locally. Keys is the issuer's public key set by
// kid — accepting several at once is what makes rotation non-disruptive.
type Verifier struct {
	// Keys maps kid to public key.
	Keys map[string]ed25519.PublicKey
	// Audience is this endpoint's identity; tickets must name it.
	Audience string
	// Leeway tolerates bounded clock skew, since verifiers include
	// player machines.
	Leeway time.Duration
	// Now overrides the clock (tests).
	Now func() time.Time

	mu       sync.Mutex
	redeemed map[string]struct{}
}

// Verify validates one compact ticket and redeems its jti: the second
// presentation of the same ticket fails with ErrReplayed
// (rule:ticket-bound-to-connection's first binding step; transport
// fingerprint binding is per-transport and layered on where available).
func (v *Verifier) Verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrMalformed
	}
	hb, err := b64.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrMalformed
	}
	var h header
	if err := json.Unmarshal(hb, &h); err != nil || h.Alg != "EdDSA" {
		return Claims{}, ErrMalformed
	}
	key, ok := v.Keys[h.Kid]
	if !ok {
		return Claims{}, fmt.Errorf("%w: %q", ErrUnknownKey, h.Kid)
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrMalformed
	}
	if !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), sig) {
		return Claims{}, ErrSignature
	}
	cb, err := b64.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrMalformed
	}
	var c Claims
	if err := json.Unmarshal(cb, &c); err != nil {
		return Claims{}, ErrMalformed
	}
	now := time.Now
	if v.Now != nil {
		now = v.Now
	}
	if now().After(time.Unix(c.ExpiresAt, 0).Add(v.Leeway)) {
		return Claims{}, ErrExpired
	}
	if c.Audience != v.Audience {
		return Claims{}, fmt.Errorf("%w: %q", ErrAudience, c.Audience)
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.redeemed == nil {
		v.redeemed = map[string]struct{}{}
	}
	if _, dup := v.redeemed[c.ID]; dup {
		return Claims{}, ErrReplayed
	}
	v.redeemed[c.ID] = struct{}{}
	return c, nil
}
