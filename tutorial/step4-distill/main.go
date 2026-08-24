// Command step4-distill is step 3's tic-tac-toe with the opponent
// replaced by one nobody wrote.
//
//	go run ./tutorial/step4-distill             # records into ./corpus
//	go run ./tutorial/step4-distill -corpus ""  # records nothing
//
// The bot in the other seat is generated Go, mined out of the recordings
// step 3 produced. Reading distill/gen/agent_gen.go is the point of the
// step: it is a switch, the cases are in the order the corpus justified,
// and every condition is a term somebody approved.
//
// Nothing about seating it is special. A distilled policy implements
// api:agent-interface like anything else, so the diff from step 3 is one
// assignment in binding below — and swapping it back for the
// hand-written bot is the same assignment the other way. The window
// cannot tell, which is the claim the distillation is making.
package main

import (
	"context"
	"flag"
	"fmt"
	"image/color"
	"os"
	"os/signal"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/run/eb"
	"github.com/shibukawa/ebigentserver/run/lan"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/distill/gen"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/game"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/msg"
)

// Board geometry in logical pixels, unchanged from step 1.
const (
	cellPx  = 100
	margin  = 10
	boardPx = cellPx * 3
	screenW = boardPx + margin*2
	screenH = screenW + 40
	scale   = 2
)

const noCell = -1

func main() {
	corpus := flag.String("corpus", "corpus", "directory to record episodes into; empty records nothing")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := eb.Run(ctx, eb.Options[msg.TTTWorld, msg.Move, game.Sight]{
		Options:     game.Options(),
		Binding:     binding(),
		Client:      &view{hover: noCell},
		Matchmaking: matchmaking(),
		Lobby: eb.LobbyOptions{
			// The other seat still belongs to a person, so it stays
			// empty and their arrival is still what starts the match.
			NoBots: true,
			// But waiting is now one of two answers rather than the
			// only one: a click seats the stand-in and plays.
			StandIn:    true,
			Background: colBG,
		},
		// One field, and ordinary play becomes a corpus. Every seat is
		// recorded the same way, so the rows a person produced and the
		// rows a bot produced differ in their agent_kind column and in
		// nothing else.
		Record:       run.RecordOptions{Root: *corpus},
		Time:         session.Paced,
		Seed:         1,
		WindowWidth:  screenW * scale,
		WindowHeight: screenH * scale,
		OnMatch:      report,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "step3-record:", err)
		os.Exit(1)
	}
}

// report says where the match went, on the terminal rather than on the
// screen. A file that appears without being mentioned is a file nobody
// looks at, and looking at it is the point of this step.
func report(res run.MatchResult) {
	for _, seat := range res.Seats {
		if !seat.LocalHuman() {
			continue
		}
		if sig, ok := res.Outcome(seat.Slot); ok {
			fmt.Printf("match %d: you %s after %d ticks\n", res.Index, sig.Terminal, res.Ticks)
		}
	}
	if res.EpisodeDir != "" {
		fmt.Printf("  recorded to %s\n", res.EpisodeDir)
	}
}

// binding is the game's rules with the empty seat filled in.
//
// This is the whole of step 4's integration. game.Binding leaves
// NewAgent nil — the rules cannot import an agent distilled from their
// own recordings — so the entry point names one, exactly as it names a
// transport, and for the same reason: both are properties of what was
// built here rather than of the game.
//
// Swap gen.Distilled for game.HandWritten and the window plays the bot
// the corpus came from. The two are indistinguishable from this file,
// which is the claim the distillation makes.
func binding() run.Binding[msg.TTTWorld, msg.Move, game.Sight] {
	b := game.Binding()
	b.NewAgent = func(session.SlotID) (string, session.Agent[game.Sight, msg.Move]) {
		return "distilled", &gen.Distilled{}
	}
	return b
}

// matchmaking is the LAN preset. The lobby asks it who is out there; if
// somebody answers, the player picks, and if nobody does this instance
// opens a room and waits.
//
// It lives here rather than beside the rules because which transport
// reaches the other player is a property of where this build runs. A
// browser build of the same rules would name a different one.
func matchmaking() run.Matchmaking[msg.TTTWorld, msg.Move, game.Sight] {
	return lan.Preset(lan.Options[msg.TTTWorld, msg.Move, msg.TTTWorldDelta, game.Sight]{
		Name:        "tictactoe",
		Protocol:    game.Protocol,
		Codec:       game.Codec(),
		Tuning:      game.Tuning(),
		EncodeInput: game.EncodeAction,
		DecodeInput: game.DecodeAction,
		Project:     game.RuleSet{}.Project,
	})
}

// view is the play scene. It holds the last board it was given and no
// rules at all.
type view struct {
	mu    sync.Mutex
	world msg.TTTWorld
	got   bool

	you   session.SlotID
	hover int
}

// cellAt maps a cursor position to a board cell — still the one place a
// pixel turns into a move.
func cellAt(x, y int) (uint8, bool) {
	x, y = x-margin, y-margin
	if x < 0 || y < 0 || x >= boardPx || y >= boardPx {
		return 0, false
	}
	return uint8((y/cellPx)*3 + x/cellPx), true
}

// Intake turns this frame's click into an action for the seat this
// machine plays. Where the action goes — an inbox one goroutine away or
// a socket one machine away — is the seating's business, not this file's.
func (v *view) Intake(seating run.Controls[msg.Move]) {
	seats := seating.LocalSeats()

	// Which seat this machine plays is known from the first frame, not
	// from the first click. Learning it late is why the status line
	// used to open on "waiting" during a turn that was already yours.
	v.mu.Lock()
	if len(seats) > 0 {
		v.you = seats[0].Slot
	}
	v.mu.Unlock()

	mx, my := ebiten.CursorPosition()
	cell, on := cellAt(mx, my)

	v.mu.Lock()
	v.hover = noCell
	if on && v.playable(cell) {
		v.hover = int(cell)
	}
	v.mu.Unlock()

	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || !on {
		return
	}
	for _, seat := range seats {
		// An illegal cell is refused upstream by the validator, so a
		// click anywhere on the board is safe to send.
		_ = seating.Submit(seat.Slot, msg.Move{Cell: cell})
	}
}

// playable reports whether the cell is one this seat may take now. It
// only dims the cursor; the authority is upstream.
func (v *view) playable(cell uint8) bool {
	if !v.got || v.world.Over {
		return false
	}
	if v.you != 0 && session.SlotID(v.world.Turn) != v.you {
		return false
	}
	return int(cell) < len(v.world.Cells) && v.world.Cells[cell] == uint8(game.Empty)
}

// Apply receives each committed board. It runs on whichever goroutine
// produced it — the session's when this instance hosts, the link's when
// it joined — so it copies rather than retaining, and the board is a
// slice, so the copy has to be deep.
func (v *view) Apply(_ session.Tick, world *msg.TTTWorld) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.world = *world
	v.world.Cells = append([]uint8(nil), world.Cells...)
	v.world.Line = append([]uint8(nil), world.Line...)
	v.got = true
}

var (
	colBG      = color.RGBA{0x1c, 0x20, 0x26, 0xff}
	colGrid    = color.RGBA{0x3c, 0x45, 0x50, 0xff}
	colX       = color.RGBA{0x6f, 0xb1, 0xf0, 0xff}
	colO       = color.RGBA{0xf0, 0xa8, 0x6f, 0xff}
	colWinLine = color.RGBA{0xf2, 0xe6, 0x8a, 0xff}
	hoverX     = color.RGBA{0x24, 0x33, 0x45, 0xff}
	hoverO     = color.RGBA{0x3a, 0x2e, 0x24, 0xff}
)

// Draw renders the board this instance was told about.
func (v *view) Draw(screen *ebiten.Image) {
	screen.Fill(colBG)

	v.mu.Lock()
	world, got, you, hover := v.world, v.got, v.you, v.hover
	v.mu.Unlock()

	if hover != noCell {
		x, y := cellOrigin(hover)
		tint := hoverX
		if game.MarkOf(you) == game.O {
			tint = hoverO
		}
		vector.DrawFilledRect(screen, x, y, cellPx, cellPx, tint, false)
	}
	for i := 1; i < 3; i++ {
		p := float32(margin + i*cellPx)
		lo, hi := float32(margin), float32(margin+boardPx)
		vector.StrokeLine(screen, lo, p, hi, p, 2, colGrid, true)
		vector.StrokeLine(screen, p, lo, p, hi, 2, colGrid, true)
	}
	if !got {
		ebitenutil.DebugPrintAt(screen, "waiting for the first board...", margin, margin+boardPx+12)
		return
	}

	const radius = cellPx * 0.28
	for cell, mark := range world.Cells {
		cx, cy := cellCenter(cell)
		switch game.Mark(mark) {
		case game.X:
			vector.StrokeLine(screen, cx-radius, cy-radius, cx+radius, cy+radius, 6, colX, true)
			vector.StrokeLine(screen, cx+radius, cy-radius, cx-radius, cy+radius, 6, colX, true)
		case game.O:
			vector.StrokeCircle(screen, cx, cy, radius, 6, colO, true)
		}
	}
	if len(world.Line) == 3 {
		ax, ay := cellCenter(int(world.Line[0]))
		bx, by := cellCenter(int(world.Line[2]))
		vector.StrokeLine(screen, ax, ay, bx, by, 5, colWinLine, true)
	}
	ebitenutil.DebugPrintAt(screen, status(world, you), margin, margin+boardPx+12)
}

// status is the one line under the board. The debug font covers Latin-1
// only, so it stays ASCII.
func status(world msg.TTTWorld, you session.SlotID) string {
	mine := game.MarkOf(you)
	switch {
	case world.Winner != 0 && uint16(you) == world.Winner:
		return fmt.Sprintf("you (%s) win", mine)
	case world.Winner != 0:
		return fmt.Sprintf("%s wins", game.Mark(game.MarkOf(session.SlotID(world.Winner))))
	case world.Over:
		return "draw"
	case you != 0 && session.SlotID(world.Turn) == you:
		return fmt.Sprintf("your move - you are %s", mine)
	default:
		return fmt.Sprintf("waiting for %s", game.MarkOf(session.SlotID(world.Turn)))
	}
}

// Layout fixes the logical resolution; Ebitengine scales it.
func (v *view) Layout(int, int) (int, int) { return screenW, screenH }

func cellOrigin(cell int) (x, y float32) {
	return float32(margin + (cell%3)*cellPx), float32(margin + (cell/3)*cellPx)
}

func cellCenter(cell int) (x, y float32) {
	x, y = cellOrigin(cell)
	return x + cellPx/2, y + cellPx/2
}
