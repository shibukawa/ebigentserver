package game_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/run/lan"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/tutorial/step2-lobby/game"
	"github.com/shibukawa/ebigentserver/tutorial/step2-lobby/msg"
)

// player is what the window does, with the mouse replaced by a rule: it
// waits to be told the board, and when it is this seat's turn it claims
// the lowest cell it was told is legal. It runs against run.Controls, so
// the same code drives the host's match and the guest's link.
type player struct {
	mu    sync.Mutex
	world game.State
	got   bool
	you   session.SlotID
	moves int
}

func (p *player) apply(_ session.Tick, world *game.State) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.world = *world
	p.world.Cells = append([]uint8(nil), world.Cells...)
	p.got = true
}

// intake is the play scene's Intake, called every frame.
func (p *player) intake(seating run.Controls[game.Action]) {
	p.mu.Lock()
	world, got := p.world, p.got
	p.mu.Unlock()
	if !got || world.Over {
		return
	}
	for _, seat := range seating.LocalSeats() {
		p.mu.Lock()
		p.you = seat.Slot
		p.mu.Unlock()
		if session.SlotID(world.Turn) != seat.Slot {
			continue
		}
		obs := (game.RuleSet{}).Project(&world, seat.Slot)
		if len(obs.Legal) == 0 {
			continue
		}
		if err := seating.Submit(seat.Slot, game.Action{Cell: obs.Legal[0]}); err == nil {
			p.mu.Lock()
			p.moves++
			p.mu.Unlock()
		}
	}
}

func (p *player) board() (game.State, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.world, p.got
}

// lanOptions is what main.go declares at the entry point, repeated here
// because a test is another entry point: it chooses a transport too.
func lanOptions() lan.Options[game.State, game.Action, msg.TTTStateDelta, game.Sight] {
	return lan.Options[game.State, game.Action, msg.TTTStateDelta, game.Sight]{
		Name:        "tictactoe",
		Protocol:    game.Protocol,
		Codec:       game.Codec(),
		Tuning:      game.Tuning(),
		EncodeInput: game.EncodeAction,
		DecodeInput: game.DecodeAction,
		Project:     game.RuleSet{}.Project,
	}
}

// TestTwoInstancesPlayOneBoard is step 2 end to end without a window:
// one instance hosts and one joins, each holds one seat, and the board
// they see is the same board because only one of them is running the
// rules.
func TestTwoInstancesPlayOneBoard(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := lanOptions()
	roster, err := run.NewRoster[game.State, game.Action, game.Sight](
		game.Options(), game.Slots())
	if err != nil {
		t.Fatal(err)
	}
	// The host player sits down, exactly as a click would seat them.
	if _, err := roster.SitLocal("host"); err != nil {
		t.Fatal(err)
	}
	host, err := lan.Open(ctx, opts, roster, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()

	guestReady := make(chan *lan.Guest[game.State, game.Action, msg.TTTStateDelta, game.Sight], 1)
	guestFail := make(chan error, 1)
	go func() {
		// Two acts, as the second instance really performs them: reach
		// the room, then take a seat in it.
		g, err := lan.MatchAt(ctx, opts, host.Endpoint())
		if err != nil {
			guestFail <- err
			return
		}
		if err := g.Sit(ctx); err != nil {
			_ = g.Close()
			guestFail <- err
			return
		}
		guestReady <- g
	}()

	// The lobby's own condition: nobody presses anything, the roster
	// simply completes when the other person arrives.
	waitUntil(t, 5*time.Second, guestFail, roster.Complete)

	hostPlayer := &player{}
	cfg := game.Config("tutorial-step2-0000", 1)
	appBroadcast := cfg.Broadcast
	cfg.Broadcast = func(tick session.Tick, world *game.State) {
		if appBroadcast != nil {
			appBroadcast(tick, world)
		}
		hostPlayer.apply(tick, world)
	}
	host.Attach(&cfg)

	match, err := roster.Finalize(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Serve(ctx, match); err != nil {
		t.Fatal(err)
	}
	match.Start(ctx, session.Paced)

	var guest *lan.Guest[game.State, game.Action, msg.TTTStateDelta, game.Sight]
	select {
	case guest = <-guestReady:
	case err := <-guestFail:
		t.Fatalf("join: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("the guest was never admitted")
	}
	defer guest.Close()

	guestPlayer := &player{}
	guest.OnWorld(guestPlayer.apply)
	go func() { _ = guest.Play(ctx) }()

	// Both frame loops, at a frame rate neither the tick rate nor the
	// send rate has to agree with.
	frames, stopFrames := context.WithCancel(ctx)
	defer stopFrames()
	go pump(frames, hostPlayer, match)
	go pump(frames, guestPlayer, guest)

	select {
	case <-match.Done():
	case <-ctx.Done():
		final, _ := hostPlayer.board()
		t.Fatalf("the match never finished: %+v", final)
	}
	if err := match.Err(); err != nil {
		t.Fatalf("match: %v", err)
	}
	stopFrames()

	authoritative, ok := hostPlayer.board()
	if !ok {
		t.Fatal("the host never saw a board")
	}
	if !authoritative.Over {
		t.Fatalf("the match ended with an unfinished board: %+v", authoritative)
	}
	if authoritative.Moves < 5 {
		t.Fatalf("only %d marks were placed", authoritative.Moves)
	}

	// The guest reached the same board without ever running the rules.
	deadline := time.Now().Add(3 * time.Second)
	for {
		seen, ok := guestPlayer.board()
		if ok && seen.Over && seen.Moves == authoritative.Moves {
			for i := range authoritative.Cells {
				if seen.Cells[i] != authoritative.Cells[i] {
					t.Fatalf("guest cell %d = %d, host has %d", i, seen.Cells[i], authoritative.Cells[i])
				}
			}
			if seen.Winner != authoritative.Winner {
				t.Fatalf("guest winner = %d, host winner = %d", seen.Winner, authoritative.Winner)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the guest never caught up: saw %+v, host had %+v", seen, authoritative)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Both seats actually played; a match one side sat out would pass
	// every check above.
	hostPlayer.mu.Lock()
	hostMoves := hostPlayer.moves
	hostPlayer.mu.Unlock()
	guestPlayer.mu.Lock()
	guestMoves := guestPlayer.moves
	guestPlayer.mu.Unlock()
	if hostMoves == 0 || guestMoves == 0 {
		t.Fatalf("host submitted %d and guest %d; both seats must have played", hostMoves, guestMoves)
	}
	t.Logf("board %v, winner %d, after %d marks", authoritative.Cells, authoritative.Winner, authoritative.Moves)
}

// pump is the frame loop: intake, over and over, at no particular rate.
func pump(ctx context.Context, p *player, seating run.Controls[game.Action]) {
	tick := time.NewTicker(8 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			p.intake(seating)
		}
	}
}

func waitUntil(t *testing.T, d time.Duration, fail <-chan error, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !cond() {
		select {
		case err := <-fail:
			t.Fatalf("waiting: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("condition never held")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
