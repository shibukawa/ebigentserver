package distill

import (
	"bytes"
	"context"
	"fmt"

	"github.com/shibukawa/ebigentserver/behavior"
	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/matchloop"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/game"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/msg"
)

// Opponent is one entry in a pairing policy: a name for the log and a
// way to make a fresh instance for each match.
type Opponent struct {
	Name string
	New  func(seed uint64) session.Agent[game.Sight, msg.Move]
}

// Pairing is the policy half of concept:continuous-match-loop: given a
// match index, who is in the other seat.
//
// The framework's matchloop owns seeding and aggregation and leaves this
// to the caller on purpose — who plays whom is a fact about the game,
// and no loop can guess it. What a policy is worth is measurable, which
// is what step 5 measures.
type Pairing struct {
	Name      string
	Opponents []Opponent
}

// Pick returns the opponent for one match.
func (p Pairing) Pick(match int) Opponent {
	return p.Opponents[match%len(p.Opponents)]
}

// RandomOnly is step 4's policy, named so it can be compared rather than
// assumed: one opponent, choosing uniformly among legal cells.
//
// It is the cheapest source of variety there is, and step 4 ended on
// what that costs — the order of the mined list came out right by how
// the frequencies happened to fall, not because anything checked it.
func RandomOnly() Pairing {
	return Pairing{
		Name: "random",
		Opponents: []Opponent{{
			Name: "random",
			New:  func(seed uint64) session.Agent[game.Sight, msg.Move] { return newRandom(seed) },
		}},
	}
}

// RoundRobin rotates over a set that disagrees with itself: a coin, the
// hand-written tactic bot, and a perfect player.
//
// The three are not there to be strong. They are there to be different —
// diversity_requirement of concept:continuous-match-loop is about the
// corpus, not about the difficulty. A coin wanders into positions no
// competent player reaches, a tactic bot manufactures the wins and
// blocks that a coin rarely sets up, and a perfect player is the only
// one that reliably produces the positions where a preference has to
// yield to something urgent.
func RoundRobin() Pairing {
	return Pairing{
		Name: "round_robin",
		Opponents: []Opponent{
			{"random", func(seed uint64) session.Agent[game.Sight, msg.Move] { return newRandom(seed) }},
			{"tactic", func(uint64) session.Agent[game.Sight, msg.Move] { return &game.Bot{} }},
			{"perfect", func(uint64) session.Agent[game.Sight, msg.Move] { return &Perfect{Principled: true} }},
		},
	}
}

// SelfPlay puts the teacher against itself.
//
// It is here to be measured and found wanting. Two deterministic players
// produce one game however many times the loop runs — step 3 has a test
// that says so — and self play only escapes that when one side carries
// randomness or the policies differ. Naming it lets step 5 show the
// number rather than assert the caveat.
func SelfPlay(teacher func() session.Agent[game.Sight, msg.Move]) Pairing {
	return Pairing{
		Name:      "self_play",
		Opponents: []Opponent{{"self", func(uint64) session.Agent[game.Sight, msg.Move] { return teacher() }}},
	}
}

// CorpusFrom plays matches under a pairing policy and returns the
// teacher's decisions, featurized against v.
//
// The loop is matchloop.Run: it owns the per-match seed so a whole run
// reproduces from one number (rule:shared-rng-seed), and everything
// about who plays whom comes from the policy.
func CorpusFrom(matches int, v *behavior.Vocabulary, teacher func() session.Agent[game.Sight, msg.Move], p Pairing) ([]behavior.Record, matchloop.Summary, error) {
	var records []behavior.Record

	summary, err := matchloop.Run(matches, 1, func(match int, seed uint64) (matchloop.Result, error) {
		opponent := p.Pick(match)
		id := fmt.Sprintf("%s-%s-%04d", p.Name, opponent.Name, match)

		var decisions bytes.Buffer
		writer := episode.NewWriter[msg.TTTWorld, msg.Move, game.Sight](
			episode.Streams{Decisions: &decisions},
			episode.ReplayComplete,
			episode.Meta{
				EpisodeID: id,
				AgentKinds: map[session.SlotID]string{
					game.SlotX: "teacher",
					game.SlotO: opponent.Name,
				},
			})

		cfg := game.Config(id, seed)
		cfg.Recorder = writer
		cfg.Clock = func() int64 { return 0 }

		s, err := session.New(cfg)
		if err != nil {
			return matchloop.Result{}, err
		}
		if err := s.OpenAdmission(); err != nil {
			return matchloop.Result{}, err
		}
		if err := s.Admit(game.SlotX, teacher()); err != nil {
			return matchloop.Result{}, err
		}
		if err := s.Admit(game.SlotO, opponent.New(seed)); err != nil {
			return matchloop.Result{}, err
		}
		if err := s.Run(context.Background()); err != nil {
			return matchloop.Result{}, err
		}

		recs, err := behavior.Segment(v, "", &decisions, func(row episode.Decision) bool {
			return row.Slot == uint16(game.SlotX)
		})
		if err != nil {
			return matchloop.Result{}, err
		}
		records = append(records, recs...)

		return matchloop.Result{Ticks: s.Tick()}, nil
	})
	if err != nil {
		return nil, summary, err
	}
	if len(records) == 0 {
		return nil, summary, behavior.ErrEmptyCorpus
	}
	return records, summary, nil
}

// Tactic is the teacher the committed agent came from: step 3's
// hand-written bot, the same one step 4 distilled.
//
// It is still the teacher after a step about stronger ones, and the
// measurements are why. What made step 4 work was not that the bot was
// good; it was that every move it made had a reason a predicate could
// name.
func Tactic() session.Agent[game.Sight, msg.Move] { return &game.Bot{} }

// Canonical mines the library the committed sources came from: the
// hand-written teacher, the judgement vocabulary, and a rotating
// opponent.
func Canonical() (*behavior.Library, []behavior.Record, error) {
	return MineFrom(CorpusMatches, Judgement(), Tactic, RoundRobin())
}

// MineFrom is Synthesize over a pairing policy: record, mine, approve
// what came back clean.
func MineFrom(matches int, v *behavior.Vocabulary, teacher func() session.Agent[game.Sight, msg.Move], p Pairing) (*behavior.Library, []behavior.Record, error) {
	records, _, err := CorpusFrom(matches, v, teacher, p)
	if err != nil {
		return nil, nil, err
	}
	cands, uncovered, err := behavior.SequentialCovering{}.Propose(v, records)
	if err != nil {
		return nil, nil, err
	}
	if len(uncovered) > 0 {
		return nil, nil, fmt.Errorf("distill: %d of %d decisions matched no rule at all", len(uncovered), len(records))
	}
	lib := &behavior.Library{Game: "tictactoe"}
	behavior.Merge(lib, cands)
	for i := range lib.Chips {
		if lib.Chips[i].Counterexamples == 0 {
			lib.Chips[i].Approved = true
			lib.Chips[i].Tags = []string{"style:perfect"}
		}
	}
	return lib, records, nil
}
