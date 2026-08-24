package game_test

import (
	"context"
	"testing"

	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/game"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/msg"
)

// decide runs one bot decision from the position cells produced. It
// goes through Project rather than handing the bot a board, because a
// sight is the only thing a bot is ever given and a test that skipped
// that step would prove less than it appears to.
func decide(t *testing.T, cells ...uint8) (msg.Move, bool) {
	t.Helper()
	world := play(t, cells...)
	bot := &game.Bot{}
	bot.Observe(game.RuleSet{}.Project(&world, session.SlotID(world.Turn)))
	return bot.Decide(context.Background())
}

func TestBotTakesTheWin(t *testing.T) {
	// O holds 0 and 1 and can finish at 2; X holds 3 and 4 and would
	// finish at 5. Winning outranks blocking, so the block is the wrong
	// answer even though it is an urgent one.
	move, ok := decide(t, 3, 0, 4, 1, 8)
	if !ok {
		t.Fatal("bot passed with the win on the board")
	}
	if move.Cell != 2 {
		t.Fatalf("bot played %d, want the winning cell 2", move.Cell)
	}
}

func TestBotBlocksTheLoss(t *testing.T) {
	// X holds 0 and 1 and finishes at 2. O has nothing of its own, so
	// the block is the only rule that can fire.
	move, ok := decide(t, 0, 4, 1, 8, 5)
	if !ok {
		t.Fatal("bot passed with a loss on the board")
	}
	if move.Cell != 2 {
		t.Fatalf("bot played %d, want the blocking cell 2", move.Cell)
	}
}

func TestBotTakesTheCentreWhenNothingIsUrgent(t *testing.T) {
	move, ok := decide(t, 0)
	if !ok {
		t.Fatal("bot passed on its first move")
	}
	if move.Cell != 4 {
		t.Fatalf("bot played %d, want the centre 4", move.Cell)
	}
}

// TestBotPassesWhenTheSeatIsNotItsTurn is the one that keeps the events
// stream clean. The local controller is pumped on every committed world,
// not only on the ticks it may act, so a bot that always answered would
// have most of its answers refused by the validator and written down as
// rejections — noise in a file whose whole purpose is to be read later.
func TestBotPassesWhenTheSeatIsNotItsTurn(t *testing.T) {
	world := game.RuleSet{}.Start(0)
	bot := &game.Bot{}
	bot.Observe(game.RuleSet{}.Project(&world, game.SlotO))
	if _, ok := bot.Decide(context.Background()); ok {
		t.Fatal("bot answered on X's turn")
	}
}

func TestBotPassesOnceTheGameIsOver(t *testing.T) {
	world := play(t, 0, 3, 1, 4, 2) // X takes the top row
	if !world.Over {
		t.Fatal("setup did not finish the game")
	}
	bot := &game.Bot{}
	bot.Observe(game.RuleSet{}.Project(&world, game.SlotO))
	if _, ok := bot.Decide(context.Background()); ok {
		t.Fatal("bot answered after the game ended")
	}
}

// TestTheBotCanBeBeaten fixes the property the corpus depends on.
//
// A stand-in that never loses would record a corpus of nothing but
// draws and defeats, and a corpus with no wins in it cannot show what
// winning looks like — which is what makes it worthless to step 4. The
// line below forks the bot: X threatens two lines at once and the block
// can only cover one. If somebody makes the bot smarter and this test
// goes red, the recording got worse even though the opponent got better.
func TestTheBotCanBeBeaten(t *testing.T) {
	world := versusBot(t, 0, 8, 6, 3)
	if !world.Over {
		t.Fatal("the game did not finish")
	}
	if world.Winner != uint16(game.SlotX) {
		t.Fatalf("winner = %d, want X (%d)", world.Winner, game.SlotX)
	}
}

// versusBot plays a whole game: X follows the script, O is the bot, and
// both of them see the board only through Project.
func versusBot(t *testing.T, script ...uint8) msg.TTTWorld {
	t.Helper()
	var rules game.RuleSet
	bot := &game.Bot{}
	world := rules.Start(0)
	for !world.Over {
		acting := rules.ActingSlots(&world)
		if len(acting) != 1 {
			t.Fatalf("%d seats acting, want 1", len(acting))
		}
		seat := acting[0]

		var move msg.Move
		if seat == game.SlotX {
			if len(script) == 0 {
				t.Fatal("the script ran out with the game still open")
			}
			move, script = msg.Move{Cell: script[0]}, script[1:]
		} else {
			bot.Observe(rules.Project(&world, seat))
			answer, ok := bot.Decide(context.Background())
			if !ok {
				t.Fatal("bot passed on its own turn")
			}
			move = answer
		}

		if err := (game.Validator{}).Legal(&world, seat, move); err != nil {
			t.Fatalf("seat %d played illegally: %v", seat, err)
		}
		rules.Apply(&world, seat, move)
	}
	if len(script) > 0 {
		t.Fatalf("the game ended with %d scripted moves unplayed", len(script))
	}
	return world
}
