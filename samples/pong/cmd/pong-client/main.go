// Command pong-client renders sample:pong with Ebitengine. It is a client
// entry point — the one kind of package allowed to import the engine
// (rule:engine-import-confined-to-client-entry; the boundary test admits
// samples/*/cmd/*client*).
//
//	pong-client              # you play left (W/S or ↑/↓) against the bot
//	pong-client -left=bot    # bot vs bot, watch
//
// The window is a client of the same loopback link the bots use: it
// reconstructs the world from the hub's snapshot/delta stream and submits
// data:player-input through the slot inbox. Keyboard reading happens only
// here, at the entry point, where raw device input becomes a game action
// (rule:no-engine-input-in-game-logic — api:input-adapter's role); the
// session-side seat is session.Detached exactly as for any remote client.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image/color"
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/shibukawa/ebigentserver/samples/pong/msg"
	"github.com/shibukawa/ebigentserver/samples/pong/pong"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
)

// Field size in game units; the window renders at 3x.
const (
	fieldW, fieldH = 320, 180
	scale          = 3
)

func main() {
	leftKind := flag.String("left", "human", "controller for the left paddle: human or bot")
	rightKind := flag.String("right", "bot", "controller for the right paddle: bot (human needs a second keyboard scheme)")
	flag.Parse()
	if *rightKind != "bot" {
		fatal(fmt.Errorf("right paddle supports only -right=bot for now"))
	}

	// Rendering wants fresh state every frame, so the local profile
	// sends at the tick rate.
	tuning := session.TuningProfile{TickRate: 60, SendRate: 60, HistoryDepth: 8, SnapshotEvery: 120}
	hub, err := statesync.NewHub(pong.Codec(), tuning)
	if err != nil {
		fatal(err)
	}
	s, err := session.New(session.Config[pong.State, pong.Input, pong.Observation]{
		ID:        "pong-client",
		Slots:     pong.Slots(),
		Game:      pong.Game{},
		Validator: pong.Validator{},
		Canonical: pong.Canonical,
		Tuning:    &tuning,
		Broadcast: hub.Broadcast,
	})
	if err != nil {
		fatal(err)
	}
	if err := s.OpenAdmission(); err != nil {
		fatal(err)
	}
	for _, slot := range pong.Slots() {
		if err := s.Admit(slot, session.Detached[pong.Observation, pong.Input]{}); err != nil {
			fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	// Right paddle: the chase bot behind the ordinary loopback client.
	attachBot := func(slot session.SlotID) {
		down, err := hub.Attach(slot)
		if err != nil {
			fatal(err)
		}
		inbox, err := s.Inbox(slot)
		if err != nil {
			fatal(err)
		}
		c := &pong.Client{Slot: slot, Agent: &pong.Bot{}, Inbox: inbox, Hub: hub, Down: down, Tuning: tuning}
		wg.Add(1)
		go c.Run(ctx, &wg)
	}
	attachBot(pong.SlotRight)
	if *leftKind == "bot" {
		attachBot(pong.SlotLeft)
	}

	// The window is the left slot's client (or a spectator in bot mode).
	viewID := pong.SlotLeft
	if *leftKind == "bot" {
		viewID = session.SlotID(99) // spectator attachment, receive-only
	}
	down, err := hub.Attach(viewID)
	if err != nil {
		fatal(err)
	}
	receiver, err := statesync.NewReceiver(pong.Codec(), tuning)
	if err != nil {
		fatal(err)
	}
	view := &window{
		hub: hub, down: down, receiver: receiver, viewID: viewID,
	}
	if *leftKind == "human" {
		inbox, err := s.Inbox(pong.SlotLeft)
		if err != nil {
			fatal(err)
		}
		view.inbox = inbox
	}

	// The authoritative session runs beside the window.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.RunRealtime(ctx, session.Paced); err != nil {
			fmt.Fprintln(os.Stderr, "pong-client: session:", err)
		}
		hub.Close()
	}()

	ebiten.SetWindowSize(fieldW*scale, fieldH*scale)
	ebiten.SetWindowTitle("pong")
	err = ebiten.RunGame(view)
	cancel()
	wg.Wait()
	if err != nil && !errors.Is(err, errQuit) {
		fatal(err)
	}
}

var errQuit = errors.New("quit")

// window renders the reconstructed state and adapts the keyboard.
type window struct {
	hub      *statesync.Hub[pong.State, msg.PongStateDelta]
	down     <-chan statesync.Packet
	receiver *statesync.Receiver[pong.State, msg.PongStateDelta]
	inbox    *session.Inbox[pong.Input] // nil when spectating
	viewID   session.SlotID
	synced   bool
}

func (w *window) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return errQuit
	}
	// Drain whatever the hub produced since the last frame.
	for {
		select {
		case pkt, ok := <-w.down:
			if !ok {
				return nil
			}
			if err := w.receiver.Apply(pkt); err != nil {
				if errors.Is(err, statesync.ErrResyncNeeded) {
					w.hub.RequestResync(w.viewID)
					continue
				}
				return err
			}
			w.synced = true
		default:
			goto drained
		}
	}
drained:
	// Keyboard → data:player-input, at most one per frame.
	if w.inbox != nil {
		var move int8
		switch {
		case ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyArrowUp):
			move = -1
		case ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyArrowDown):
			move = 1
		}
		if move != 0 {
			if st, _, ok := w.receiver.State(); ok {
				w.inbox.Submit(pong.Input{Tick: uint32(st.Tick), MoveY: move})
			}
		}
	}
	return nil
}

var (
	colBG     = color.RGBA{16, 24, 32, 255}
	colNet    = color.RGBA{60, 76, 92, 255}
	colPaddle = color.RGBA{235, 235, 235, 255}
	colBall   = color.RGBA{255, 210, 90, 255}
)

func (w *window) Draw(screen *ebiten.Image) {
	screen.Fill(colBG)
	st, _, ok := w.receiver.State()
	if !ok || !w.synced {
		ebitenutil.DebugPrintAt(screen, "waiting for snapshot...", fieldW/2-50, fieldH/2)
		return
	}
	// Center net.
	for y := 0; y < fieldH; y += 12 {
		vector.DrawFilledRect(screen, fieldW/2-1, float32(y), 2, 6, colNet, false)
	}
	// Wire scale is 1/1024: presentation math is float, which is legal
	// exactly and only here (rule:no-float-in-simulation).
	px := func(v msg.Fixed1024) float32 { return float32(v) / 1024 }
	const paddleW, paddleHalf, ballR = 4, 16, 3
	vector.DrawFilledRect(screen, 8-paddleW/2, px(st.LeftY)-paddleHalf, paddleW, paddleHalf*2, colPaddle, false)
	vector.DrawFilledRect(screen, fieldW-8-paddleW/2, px(st.RightY)-paddleHalf, paddleW, paddleHalf*2, colPaddle, false)
	if !st.Over {
		vector.DrawFilledCircle(screen, px(st.BallX), px(st.BallY), ballR, colBall, false)
	}
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", st.ScoreL), fieldW/2-24, 6)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", st.ScoreR), fieldW/2+18, 6)
	if st.Over {
		side := "left"
		if st.Winner == uint16(pong.SlotRight) {
			side = "right"
		}
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s wins — ESC to quit", side), fieldW/2-56, fieldH/2)
	}
}

func (w *window) Layout(int, int) (int, int) { return fieldW, fieldH }

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "pong-client:", err)
	os.Exit(1)
}
