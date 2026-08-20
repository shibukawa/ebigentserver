//go:build !js && !wasip1

// Package rtc is the WebRTC implementation of api:transport-interface —
// the peer-to-peer transport of system:webrtc: a reliable ordered data
// channel for control and admission traffic plus an unordered,
// zero-retransmit data channel for the state stream.
//
// Channel plan (system:webrtc): channel "reliable" (ordered, reliable)
// carries data:snapshot, control messages, and admission; channel
// "state" (unordered, maxRetransmits 0) carries data:player-input and
// data:state-delta. Both are pre-negotiated with fixed stream ids, so
// neither side waits for the other's channel announcement — only for
// the SCTP association to open.
//
// Signaling is the caller's problem by design: NewOffer/Accept/Complete
// expose the SDP blobs so they can travel over concept:control-plane or
// an api:manual-signaling-token (see package signaltoken). ICE is
// non-trickle only — the SDP is produced after candidate gathering
// completes (bounded wait), because manual signaling has no channel to
// trickle candidates over. mDNS candidate obfuscation is disabled: a
// manually-signaled peer has no rendezvous to resolve .local names
// through.
//
// This file is native-only linkage (rule:build-tag-only-for-linkage):
// the pion stack does not target js/wasm, where the browser's own
// WebRTC API takes this role behind the same interface.
package rtc

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"

	"github.com/shibukawa/ebigentserver/transport"
)

// maxMessage bounds one message on either data channel; pion's default
// negotiated SCTP max-message-size. api:message-framing above splits
// larger reliable payloads.
const maxMessage = 65535

// defaultGatherTimeout bounds non-trickle ICE gathering
// (api:manual-signaling-token: "bounded wait around 20 seconds, then
// emit whatever candidates exist").
const defaultGatherTimeout = 20 * time.Second

// Config shapes one peer endpoint.
type Config struct {
	// ICEServers lists STUN/TURN servers; empty means host candidates
	// only, which suffices on a LAN or loopback. TURN credentials stay
	// in memory — they never enter a signaling token
	// (api:manual-signaling-token never_contains).
	ICEServers []webrtc.ICEServer
	// GatherTimeout bounds the non-trickle candidate wait; zero means
	// the 20s default. On timeout the SDP ships whatever candidates
	// exist.
	GatherTimeout time.Duration
	// IncludeLoopback adds loopback host candidates (pion excludes
	// them by default). Tests and single-machine topologies need it.
	IncludeLoopback bool
}

// Peer is one endpoint of an in-progress or established WebRTC
// connection. Build it with NewOffer or Accept, finish signaling
// (Complete on the offerer), then call Conn to wait for the channels.
type Peer struct {
	pc       *webrtc.PeerConnection
	conn     *Conn
	ready    chan struct{} // closed when both channels are open
	openOnce sync.Once
	openMu   sync.Mutex
	open     int
}

func newPeer(cfg Config) (*Peer, error) {
	se := webrtc.SettingEngine{}
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	if cfg.IncludeLoopback {
		se.SetIncludeLoopbackCandidate(true)
	}
	api := webrtc.NewAPI(webrtc.WithSettingEngine(se))
	pc, err := api.NewPeerConnection(webrtc.Configuration{ICEServers: cfg.ICEServers})
	if err != nil {
		return nil, err
	}
	p := &Peer{pc: pc, ready: make(chan struct{})}
	p.conn = &Conn{
		peer:  p,
		inbox: make(chan transport.Message, 256),
		done:  make(chan struct{}),
	}

	// Both channels are negotiated with fixed ids, so both sides create
	// them locally and only the SCTP open event matters — no 1-byte
	// preamble dance like a QUIC stream needs.
	tru, fls := true, false
	var idReliable, idState, zeroRtx uint16 = 0, 1, 0
	rel, err := pc.CreateDataChannel("reliable", &webrtc.DataChannelInit{
		Negotiated: &tru, ID: &idReliable, Ordered: &tru,
	})
	if err != nil {
		_ = pc.Close()
		return nil, err
	}
	st, err := pc.CreateDataChannel("state", &webrtc.DataChannelInit{
		Negotiated: &tru, ID: &idState, Ordered: &fls, MaxRetransmits: &zeroRtx,
	})
	if err != nil {
		_ = pc.Close()
		return nil, err
	}
	p.conn.reliable, p.conn.state = rel, st

	rel.OnOpen(p.channelOpened)
	st.OnOpen(p.channelOpened)
	rel.OnMessage(func(m webrtc.DataChannelMessage) {
		p.conn.deliver(transport.Reliable, m.Data, true)
	})
	st.OnMessage(func(m webrtc.DataChannelMessage) {
		p.conn.deliver(transport.Unreliable, m.Data, false)
	})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed {
			_ = p.conn.Close()
		}
	})
	return p, nil
}

func (p *Peer) channelOpened() {
	p.openMu.Lock()
	p.open++
	both := p.open == 2
	p.openMu.Unlock()
	if both {
		p.openOnce.Do(func() { close(p.ready) })
	}
}

// gather runs non-trickle ICE: set the local description, wait (bounded)
// for gathering to complete, and return the candidate-laden SDP.
func (p *Peer) gather(desc webrtc.SessionDescription, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = defaultGatherTimeout
	}
	done := webrtc.GatheringCompletePromise(p.pc)
	if err := p.pc.SetLocalDescription(desc); err != nil {
		_ = p.pc.Close()
		return "", err
	}
	select {
	case <-done:
	case <-time.After(timeout):
		// Bounded wait elapsed: ship whatever candidates exist.
	}
	local := p.pc.LocalDescription()
	if local == nil {
		_ = p.pc.Close()
		return "", transport.ErrClosed
	}
	return local.SDP, nil
}

// NewOffer creates the inviting peer and returns its offer SDP, complete
// with gathered candidates. Feed the answer back through Complete.
func NewOffer(cfg Config) (*Peer, string, error) {
	p, err := newPeer(cfg)
	if err != nil {
		return nil, "", err
	}
	offer, err := p.pc.CreateOffer(nil)
	if err != nil {
		_ = p.pc.Close()
		return nil, "", err
	}
	sdp, err := p.gather(offer, cfg.GatherTimeout)
	if err != nil {
		return nil, "", err
	}
	return p, sdp, nil
}

// Accept creates the answering peer from a received offer SDP and
// returns its answer SDP with gathered candidates. Signaling is complete
// on this side once Accept returns.
func Accept(cfg Config, offerSDP string) (*Peer, string, error) {
	p, err := newPeer(cfg)
	if err != nil {
		return nil, "", err
	}
	if err := p.pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer, SDP: offerSDP,
	}); err != nil {
		_ = p.pc.Close()
		return nil, "", err
	}
	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		_ = p.pc.Close()
		return nil, "", err
	}
	sdp, err := p.gather(answer, cfg.GatherTimeout)
	if err != nil {
		return nil, "", err
	}
	return p, sdp, nil
}

// Complete feeds the answer SDP back into an offering peer, finishing
// signaling. Connectivity establishment starts here.
func (p *Peer) Complete(answerSDP string) error {
	return p.pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer, SDP: answerSDP,
	})
}

// Conn blocks until both data channels are open, then returns the
// established connection. It fails if ctx expires or the peer dies
// first.
func (p *Peer) Conn(ctx context.Context) (transport.Conn, error) {
	select {
	case <-p.ready:
		return p.conn, nil
	case <-p.conn.done:
		return nil, transport.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Fingerprint returns the remote peer's DTLS certificate fingerprint
// (e.g. "sha-256 AB:CD:…"), the identity handle of
// rule:ticket-bound-to-connection: flow:peer-authentication compares a
// ticket's fingerprint claim against this value, so a copied ticket
// presented from another machine fails. Empty until a remote
// description is set.
//
// The value is read from the verified remote description; DTLS refuses
// the handshake if the actual certificate does not hash to it.
func (p *Peer) Fingerprint() string {
	desc := p.pc.RemoteDescription()
	if desc == nil {
		return ""
	}
	for _, line := range strings.Split(desc.SDP, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if v, ok := strings.CutPrefix(line, "a=fingerprint:"); ok {
			return v
		}
	}
	return ""
}

// Close tears the peer connection down; idempotent.
func (p *Peer) Close() error { return p.conn.Close() }

// Conn adapts one established WebRTC peer connection to
// api:transport-interface, merging both data channels into one inbox.
type Conn struct {
	peer     *Peer
	reliable *webrtc.DataChannel
	state    *webrtc.DataChannel

	inbox  chan transport.Message
	done   chan struct{}
	closed sync.Once
}

var _ transport.Conn = (*Conn)(nil)

// Capability: the full peer profile — the only transport that offers
// PeerToPeer (rule:transport-selected-by-capability keys off this, not
// the protocol name).
func (c *Conn) Capability() transport.Capability {
	return transport.Capability{ReliableStream: true, UnreliableDatagram: true, PeerToPeer: true, Browser: true}
}

// deliver moves one received data-channel message into the inbox. The
// payload is copied: ownership-safe delivery is part of the Conn
// contract, and pion owns the callback's slice.
func (c *Conn) deliver(ch transport.Channel, data []byte, block bool) {
	payload := make([]byte, len(data))
	copy(payload, data)
	msg := transport.Message{Channel: ch, Payload: payload}
	if block {
		select {
		case c.inbox <- msg:
		case <-c.done:
		}
		return
	}
	select {
	case c.inbox <- msg:
	default: // datagrams may drop under backpressure
	}
}

func (c *Conn) send(dc *webrtc.DataChannel, ctx context.Context, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-c.done:
		return transport.ErrClosed
	default:
	}
	if len(payload) > maxMessage {
		return transport.ErrTooLarge
	}
	if err := dc.Send(payload); err != nil {
		return transport.ErrClosed
	}
	return nil
}

// SendReliable queues one message on the ordered channel.
func (c *Conn) SendReliable(ctx context.Context, payload []byte) error {
	return c.send(c.reliable, ctx, payload)
}

// SendUnreliable queues one message on the zero-retransmit channel; it
// may be dropped or arrive out of order.
func (c *Conn) SendUnreliable(ctx context.Context, payload []byte) error {
	return c.send(c.state, ctx, payload)
}

// Receive blocks for the next message from either channel.
func (c *Conn) Receive(ctx context.Context) (transport.Message, error) {
	select {
	case m := <-c.inbox:
		return m, nil
	default:
	}
	select {
	case m := <-c.inbox:
		return m, nil
	case <-ctx.Done():
		return transport.Message{}, ctx.Err()
	case <-c.done:
		return transport.Message{}, transport.ErrClosed
	}
}

// Close tears the whole peer connection down; idempotent. The peer
// connection is the session's lifeline — individual channels are never
// closed on their own.
func (c *Conn) Close() error {
	c.closed.Do(func() {
		close(c.done)
		_ = c.peer.pc.Close()
	})
	return nil
}
