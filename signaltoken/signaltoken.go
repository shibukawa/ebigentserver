// Package signaltoken implements api:manual-signaling-token: a
// self-contained WebRTC offer or answer encoded as a text-safe token
// that players exchange out of band — chat, wiki, mail — when no
// signaling server exists (the rendezvous-capability arm of
// rule:unauthenticated-admission-requires-scope-or-capability).
//
// Wire layout, before base64: version byte, type byte
// (invitation/answer), expiry unix seconds, declared payload length,
// then the payload — {session_id, sdp} JSON put through a fixed
// dictionary of common SDP substrings, then flate. The whole blob is
// base64.RawURLEncoding. The declared length lets Decode ignore
// trailing junk: chat and wiki software appends things, so anything
// past the declared payload is not read.
//
// Carrier hygiene: the token travels in a URL fragment — a fragment is
// never sent to the HTTP server — and is read into memory at startup,
// then stripped from the visible URL. It never contains TURN
// credentials; those stay in page memory only.
//
// A token is bearer data, so its safety is being short-lived and
// redeemable once (rule:invitation-is-single-use-and-expiring): Decode
// enforces expiry, and the accepting side enforces single use with a
// Redeemer.
package signaltoken

import (
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Type distinguishes the two token forms.
type Type byte

const (
	// Invitation carries the host's offer.
	Invitation Type = 1
	// Answer carries the joiner's response.
	Answer Type = 2
)

// version is the current wire version byte.
const version = 1

// headerLen = version(1) + type(1) + expiry(8) + payload length(4).
const headerLen = 14

// maxInflated bounds the decompressed payload; an SDP is a few KB, so
// anything near this is hostile.
const maxInflated = 64 << 10

// Validation errors; every one of them fails decoding closed.
var (
	ErrMalformed = errors.New("signaltoken: malformed token")
	ErrVersion   = errors.New("signaltoken: unsupported token version")
	ErrType      = errors.New("signaltoken: unknown token type")
	ErrTooLarge  = errors.New("signaltoken: payload exceeds size bound")
	ErrExpired   = errors.New("signaltoken: token expired")
	ErrRedeemed  = errors.New("signaltoken: token already redeemed")
)

// Payload is the token body.
type Payload struct {
	// SessionID names the session the token invites into.
	SessionID string `json:"session_id"`
	// SDP is the complete offer or answer, candidates included
	// (non-trickle ICE — there is no channel to trickle over).
	SDP string `json:"sdp"`
}

// Token is one decoded token.
type Token struct {
	Type      Type
	ExpiresAt time.Time
	Payload   Payload
}

// dict is the common-SDP-substring substitution table. Entries are
// written in their JSON-escaped form (SDP CRLFs surface as literal
// `\r\n` inside the JSON string). Codes are control bytes, which never
// appear raw in JSON output (encoding/json escapes them), so the
// substitution is unambiguous. flate does the heavy lifting; the
// dictionary shaves the highest-frequency runs first.
var dict = []struct {
	code byte
	s    string
}{
	{0x01, `\r\na=candidate:`},
	{0x02, `\r\na=fingerprint:sha-256 `},
	{0x03, `\r\na=ice-ufrag:`},
	{0x04, `\r\na=ice-pwd:`},
	{0x05, `\r\na=setup:`},
	{0x06, `webrtc-datachannel`},
	{0x07, `\r\na=sctp-port:`},
	{0x08, `\r\na=group:BUNDLE `},
	{0x0B, `\r\nm=application `},
	{0x0C, `\r\na=mid:`},
	{0x0E, ` typ host`},
	{0x0F, `udp`},
	{0x10, `UDP/DTLS/SCTP`},
	{0x11, `IN IP4 `},
	{0x12, `\r\na=max-message-size:`},
	{0x13, `\r\ns=-\r\nt=0 0`},
	{0x14, `network-cost `},
}

func substitute(b []byte) []byte {
	for _, e := range dict {
		b = bytes.ReplaceAll(b, []byte(e.s), []byte{e.code})
	}
	return b
}

func unsubstitute(b []byte) []byte {
	for i := len(dict) - 1; i >= 0; i-- {
		e := dict[i]
		b = bytes.ReplaceAll(b, []byte{e.code}, []byte(e.s))
	}
	return b
}

// Encode packs one offer or answer into a token. Keep expiresAt short —
// minutes, not hours (rule:invitation-is-single-use-and-expiring).
func Encode(typ Type, p Payload, expiresAt time.Time) (string, error) {
	if typ != Invitation && typ != Answer {
		return "", ErrType
	}
	if p.SessionID == "" || !saneSDP(p.SDP) {
		return "", ErrMalformed
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	var comp bytes.Buffer
	fw, err := flate.NewWriter(&comp, flate.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(substitute(raw)); err != nil {
		return "", err
	}
	if err := fw.Close(); err != nil {
		return "", err
	}
	bin := make([]byte, headerLen, headerLen+comp.Len())
	bin[0] = version
	bin[1] = byte(typ)
	binary.BigEndian.PutUint64(bin[2:10], uint64(expiresAt.Unix()))
	binary.BigEndian.PutUint32(bin[10:14], uint32(comp.Len()))
	bin = append(bin, comp.Bytes()...)
	return base64.RawURLEncoding.EncodeToString(bin), nil
}

// alphabetEnd returns the index of the first byte outside the
// RawURLEncoding alphabet; everything from there on is trailing junk a
// chat client glued on.
func alphabetEnd(s string) int {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return i
		}
	}
	return len(s)
}

// saneSDP is the shape check: a session description starts with v=0 and
// a data-channel one carries an application media section and a DTLS
// fingerprint.
func saneSDP(sdp string) bool {
	return strings.HasPrefix(sdp, "v=0") &&
		strings.Contains(sdp, "m=application") &&
		strings.Contains(sdp, "a=fingerprint:")
}

// Decode validates and unpacks a token: version, type, declared length
// (trailing junk beyond it is ignored), alphabet, inflated size bound,
// UTF-8, JSON shape, session id, SDP shape, expiry.
func Decode(token string) (Token, error) {
	return decodeAt(token, time.Now())
}

func decodeAt(token string, now time.Time) (Token, error) {
	// Base64 works in 4-character units; cut at the first non-alphabet
	// byte, then trim to a decodable length ≥ the declared payload.
	trimmed := token[:alphabetEnd(token)]
	if rem := len(trimmed) % 4; rem == 1 {
		trimmed = trimmed[:len(trimmed)-1]
	}
	bin, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return Token{}, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if len(bin) < headerLen {
		return Token{}, ErrMalformed
	}
	if bin[0] != version {
		return Token{}, fmt.Errorf("%w: %d", ErrVersion, bin[0])
	}
	typ := Type(bin[1])
	if typ != Invitation && typ != Answer {
		return Token{}, fmt.Errorf("%w: %d", ErrType, bin[1])
	}
	expiresAt := time.Unix(int64(binary.BigEndian.Uint64(bin[2:10])), 0)
	n := binary.BigEndian.Uint32(bin[10:14])
	if n > maxInflated {
		return Token{}, ErrTooLarge
	}
	if uint32(len(bin)-headerLen) < n {
		return Token{}, fmt.Errorf("%w: truncated payload", ErrMalformed)
	}
	payload := bin[headerLen : headerLen+int(n)] // beyond n: appended junk, not read

	fr := flate.NewReader(bytes.NewReader(payload))
	inflated, err := io.ReadAll(io.LimitReader(fr, maxInflated+1))
	if err != nil {
		return Token{}, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if len(inflated) > maxInflated {
		return Token{}, ErrTooLarge
	}
	raw := unsubstitute(inflated)
	if !utf8.Valid(raw) {
		return Token{}, fmt.Errorf("%w: not utf-8", ErrMalformed)
	}
	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return Token{}, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if p.SessionID == "" || !saneSDP(p.SDP) {
		return Token{}, fmt.Errorf("%w: bad payload shape", ErrMalformed)
	}
	if now.After(expiresAt) {
		return Token{}, ErrExpired
	}
	return Token{Type: typ, ExpiresAt: expiresAt, Payload: p}, nil
}

// Redeemer enforces the single-use half of
// rule:invitation-is-single-use-and-expiring on the accepting side: the
// second presentation of the same token fails. It remembers token
// hashes, pruning them once their expiry passes (an expired token is
// already dead via Decode).
type Redeemer struct {
	// Now overrides the clock (tests).
	Now func() time.Time

	mu   sync.Mutex
	used map[[sha256.Size]byte]time.Time
}

// Redeem marks one decoded token as used. Call it with the token string
// and the ExpiresAt that Decode returned.
func (r *Redeemer) Redeem(token string, expiresAt time.Time) error {
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	h := sha256.Sum256([]byte(token))
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.used == nil {
		r.used = map[[sha256.Size]byte]time.Time{}
	}
	t := now()
	for k, exp := range r.used {
		if t.After(exp) {
			delete(r.used, k)
		}
	}
	if _, dup := r.used[h]; dup {
		return ErrRedeemed
	}
	r.used[h] = expiresAt
	return nil
}
