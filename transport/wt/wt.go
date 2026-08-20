//go:build !js && !wasip1

// Package wt is the WebTransport implementation of api:transport-interface
// — the primary transport for browser-reachable realtime play
// (decision:webtransport-primary-for-wasm): a reliable ordered stream for
// control traffic plus true unreliable datagrams for the state stream.
//
// The reliable channel is one bidirectional WebTransport stream opened by
// the client at connect, carrying length-prefixed messages; datagrams map
// to QUIC datagrams as-is.
//
// This file is native-only linkage (rule:build-tag-only-for-linkage): the
// quic-go server and native client cannot build for js/wasm, where the
// browser's own WebTransport API takes this role instead.
package wt

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"

	"github.com/shibukawa/ebigentserver/transport"
)

// maxReliableMessage bounds one length-prefixed message on the reliable
// stream; api:message-framing above splits anything larger.
const maxReliableMessage = 1 << 20

// preamble is the first byte the client writes on the reliable stream,
// so the stream materializes on the wire and the server can verify it is
// talking to this layer.
const preamble = 0x57

// Conn adapts one WebTransport session.
type Conn struct {
	sess   *webtransport.Session
	stream *webtransport.Stream

	writeMu sync.Mutex
	inbox   chan transport.Message
	done    chan struct{}
	closed  sync.Once
}

var _ transport.Conn = (*Conn)(nil)

func wrap(sess *webtransport.Session, stream *webtransport.Stream) *Conn {
	c := &Conn{sess: sess, stream: stream, inbox: make(chan transport.Message, 256), done: make(chan struct{})}
	go c.readStream()
	go c.readDatagrams()
	return c
}

// Capability: the full profile — this is the transport the fallback rule
// falls back FROM.
func (c *Conn) Capability() transport.Capability {
	return transport.Capability{ReliableStream: true, UnreliableDatagram: true, Browser: true}
}

func (c *Conn) readStream() {
	var lenBuf [4]byte
	for {
		if _, err := io.ReadFull(c.stream, lenBuf[:]); err != nil {
			c.Close()
			return
		}
		n := binary.BigEndian.Uint32(lenBuf[:])
		if n > maxReliableMessage {
			c.Close()
			return
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(c.stream, body); err != nil {
			c.Close()
			return
		}
		select {
		case c.inbox <- transport.Message{Channel: transport.Reliable, Payload: body}:
		case <-c.done:
			return
		}
	}
}

func (c *Conn) readDatagrams() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-c.done
		cancel()
	}()
	for {
		data, err := c.sess.ReceiveDatagram(ctx)
		if err != nil {
			return
		}
		msg := transport.Message{Channel: transport.Unreliable, Payload: data}
		select {
		case c.inbox <- msg:
		default: // datagrams may drop under backpressure
		}
	}
}

// SendReliable writes one length-prefixed message to the stream.
func (c *Conn) SendReliable(ctx context.Context, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-c.done:
		return transport.ErrClosed
	default:
	}
	if len(payload) > maxReliableMessage {
		return transport.ErrTooLarge
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := c.stream.Write(lenBuf[:]); err != nil {
		return transport.ErrClosed
	}
	if _, err := c.stream.Write(payload); err != nil {
		return transport.ErrClosed
	}
	return nil
}

// SendUnreliable sends one QUIC datagram.
func (c *Conn) SendUnreliable(ctx context.Context, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-c.done:
		return transport.ErrClosed
	default:
	}
	if err := c.sess.SendDatagram(payload); err != nil {
		if strings.Contains(err.Error(), "too large") {
			return transport.ErrTooLarge
		}
		return transport.ErrClosed
	}
	return nil
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

// Close tears the session down; idempotent.
func (c *Conn) Close() error {
	c.closed.Do(func() {
		close(c.done)
		_ = c.sess.CloseWithError(0, "closed")
	})
	return nil
}

// Server accepts WebTransport connections over HTTP/3.
type Server struct {
	wt *webtransport.Server
}

// NewServer builds a server listening on addr (UDP) with the given TLS
// configuration. Register handlers on mux and call ListenAndServe.
func NewServer(addr string, tlsConf *tls.Config, mux http.Handler) *Server {
	s := &Server{wt: &webtransport.Server{
		H3: &http3.Server{
			Addr:      addr,
			TLSConfig: http3.ConfigureTLSConfig(tlsConf), // adds the h3 ALPN

			Handler:         mux,
			EnableDatagrams: true,
			QUICConfig: &quic.Config{
				EnableDatagrams:                  true,
				EnableStreamResetPartialDelivery: true, // required by webtransport-go
			},
		},
		CheckOrigin: func(*http.Request) bool { return true },
	}}
	// Advertise the WebTransport HTTP/3 settings; without this a client
	// refuses the session before CONNECT.
	webtransport.ConfigureHTTP3Server(s.wt.H3)
	return s
}

// ListenAndServe blocks serving the UDP listener. It must go through the
// webtransport server (not H3 directly) so session tracking hooks are
// installed.
func (s *Server) ListenAndServe() error { return s.wt.ListenAndServe() }

// Close stops the server.
func (s *Server) Close() error { return s.wt.Close() }

// Upgrade turns one CONNECT request into a transport connection: it
// accepts the session and waits for the client's reliable stream.
func (s *Server) Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	sess, err := s.wt.Upgrade(w, r)
	if err != nil {
		return nil, err
	}
	// The session outlives the CONNECT request handler, so its context —
	// not the request's — bounds the stream accept.
	ctx, cancel := context.WithTimeout(sess.Context(), 10*time.Second)
	defer cancel()
	stream, err := sess.AcceptStream(ctx)
	if err != nil {
		_ = sess.CloseWithError(1, "no reliable stream")
		return nil, err
	}
	var pre [1]byte
	if _, err := io.ReadFull(stream, pre[:]); err != nil || pre[0] != preamble {
		_ = sess.CloseWithError(1, "bad preamble")
		return nil, transport.ErrClosed
	}
	return wrap(sess, stream), nil
}

// Dial connects to a WebTransport endpoint (https:// URL) and opens the
// reliable stream.
func Dial(ctx context.Context, url string, tlsConf *tls.Config) (*Conn, error) {
	if tlsConf == nil {
		tlsConf = &tls.Config{}
	}
	if len(tlsConf.NextProtos) == 0 {
		tlsConf = tlsConf.Clone()
		tlsConf.NextProtos = []string{http3.NextProtoH3}
	}
	d := webtransport.Transport{
		TLSClientConfig: tlsConf,
		QUICConfig: &quic.Config{
			EnableDatagrams:                  true,
			EnableStreamResetPartialDelivery: true, // required by webtransport-go
		},
	}
	// The CONNECT response body IS the session's control stream: it
	// stays open for the session's life, so it must not be closed here.
	_, sess, err := d.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	stream, err := sess.OpenStreamSync(ctx)
	if err != nil {
		_ = sess.CloseWithError(1, "no reliable stream")
		return nil, err
	}
	// The preamble opens the stream on the wire so the server's
	// AcceptStream fires without waiting for the first application
	// message.
	if _, err := stream.Write([]byte{preamble}); err != nil {
		_ = sess.CloseWithError(1, "stream write failed")
		return nil, err
	}
	return wrap(sess, stream), nil
}
