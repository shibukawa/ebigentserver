package admission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shibukawa/ebigentserver/transport"
)

// The handshake is the first exchange on a new connection's reliable
// channel, before any state or input message. Its JSON envelope is
// version-independent on purpose: the protocol version comparison
// (rule:protocol-version-must-match) must be expressible between peers
// whose generated CBOR schemas already disagree.

// Hello is the client's first message.
type Hello struct {
	// Protocol is the client's data:protocol-version.
	Protocol string `json:"protocol"`
	// Ticket is the compact data:session-ticket.
	Ticket string `json:"ticket"`
}

// Welcome is the server's reply.
type Welcome struct {
	OK bool `json:"ok"`
	// Reason is the stable failure reason when OK is false.
	Reason string `json:"reason,omitempty"`
	// Seat confirms the admitted slot.
	Seat uint16 `json:"seat,omitempty"`
	// Role confirms the admitted role.
	Role string `json:"role,omitempty"`
	// Seed is the session's shared RNG seed (rule:shared-rng-seed,
	// carried on join).
	Seed uint64 `json:"seed,omitempty"`
}

// ErrRejected is returned by Join when the server refuses admission; its
// message carries the server's stable reason.
var ErrRejected = errors.New("admission rejected")

// Accept runs the server side of the handshake on a fresh connection:
// version check first (a mismatch is an explicit protocol error, never a
// negotiation), then local ticket verification and jti redemption. On
// success the Welcome carrying seat, role, and seed is already sent.
func Accept(ctx context.Context, conn transport.Conn, protocol string, verifier *Verifier, seed uint64) (Claims, error) {
	msg, err := conn.Receive(ctx)
	if err != nil {
		return Claims{}, err
	}
	var hello Hello
	if err := json.Unmarshal(msg.Payload, &hello); err != nil {
		return Claims{}, reject(ctx, conn, "malformed hello")
	}
	if hello.Protocol != protocol {
		// rule:protocol-version-must-match — reject with an explicit
		// version error naming both sides.
		return Claims{}, reject(ctx, conn,
			fmt.Sprintf("protocol version mismatch: server %s, client %s", protocol, hello.Protocol))
	}
	claims, err := verifier.Verify(hello.Ticket)
	if err != nil {
		// Stable reasons, no internals: the client learns why in
		// category terms, observability gets the rest.
		reason := "invalid ticket"
		switch {
		case errors.Is(err, ErrExpired):
			reason = "ticket expired"
		case errors.Is(err, ErrReplayed):
			reason = "ticket already redeemed"
		}
		return Claims{}, errors.Join(err, reject(ctx, conn, reason))
	}
	ok := Welcome{OK: true, Seat: claims.Seat, Role: claims.Role, Seed: seed}
	body, err := json.Marshal(ok)
	if err != nil {
		return Claims{}, err
	}
	if err := conn.SendReliable(ctx, body); err != nil {
		return Claims{}, err
	}
	return claims, nil
}

func reject(ctx context.Context, conn transport.Conn, reason string) error {
	body, _ := json.Marshal(Welcome{OK: false, Reason: reason})
	_ = conn.SendReliable(ctx, body)
	return fmt.Errorf("%w: %s", ErrRejected, reason)
}

// Join runs the client side: send the Hello, wait for the Welcome.
func Join(ctx context.Context, conn transport.Conn, protocol, ticket string) (Welcome, error) {
	body, err := json.Marshal(Hello{Protocol: protocol, Ticket: ticket})
	if err != nil {
		return Welcome{}, err
	}
	if err := conn.SendReliable(ctx, body); err != nil {
		return Welcome{}, err
	}
	msg, err := conn.Receive(ctx)
	if err != nil {
		return Welcome{}, err
	}
	var w Welcome
	if err := json.Unmarshal(msg.Payload, &w); err != nil {
		return Welcome{}, fmt.Errorf("admission: malformed welcome: %w", err)
	}
	if !w.OK {
		return w, fmt.Errorf("%w: %s", ErrRejected, w.Reason)
	}
	return w, nil
}
