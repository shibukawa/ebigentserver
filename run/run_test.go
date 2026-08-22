package run_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/session"
)

// The wrapper is tested against a game small enough to fit in this file:
// two seats race a counter to the target. It exists so these tests
// exercise run and nothing else — no rendering, no fixed-point physics,
// no sample.

const target = 20

type state struct {
	Count [2]int
}

type action struct {
	Step int `json:"step"`
}

type observation struct {
	You    session.SlotID `json:"you"`
	Mine   int            `json:"mine"`
	Theirs int            `json:"theirs"`
}

type rules struct{}

func (rules) Start(seed uint64) state { return state{} }

func (rules) ActingSlots(s *state) []session.SlotID {
	if s.Count[0] >= target || s.Count[1] >= target {
		return nil
	}
	return []session.SlotID{1, 2}
}

func (rules) Apply(s *state, slot session.SlotID, a action) {
	s.Count[slot-1] += a.Step
}

func (rules) Advance(s *state) {}

func (rules) Project(s *state, slot session.SlotID) observation {
	return observation{You: slot, Mine: s.Count[slot-1], Theirs: s.Count[2-slot]}
}

func (rules) Evaluate(s *state, slot session.SlotID) session.EvaluationSignal {
	mine, theirs := s.Count[slot-1], s.Count[2-slot]
	sig := session.EvaluationSignal{Score: int64(mine)}
	if mine < target && theirs < target {
		return sig
	}
	switch {
	case mine >= target && theirs >= target:
		sig.Terminal = session.Draw
	case mine >= target:
		sig.Terminal = session.Win
	default:
		sig.Terminal = session.Lose
	}
	return sig
}

// stepper always adds the same amount, so the winner is decided by the
// seat it was given rather than by anything random.
type stepper struct{ by int }

func (*stepper) Joined(session.SlotID)                   {}
func (*stepper) Observe(observation)                     {}
func (s *stepper) Decide(context.Context) (action, bool) { return action{Step: s.by}, true }
func (*stepper) Ended(session.Result)                    {}

func config(id string, seed uint64) session.Config[state, action, observation] {
	tuning := session.TuningProfile{TickRate: 60, SendRate: 60, HistoryDepth: 1}
	return session.Config[state, action, observation]{
		ID:        id,
		Slots:     []session.SlotID{1, 2},
		Game:      rules{},
		Seed:      seed,
		Tuning:    &tuning,
		Canonical: func(s *state) []byte { b, _ := json.Marshal(s); return b },
	}
}

func newAgent(slot session.SlotID) (string, session.Agent[observation, action]) {
	if slot == 1 {
		return "fast", &stepper{by: 2}
	}
	return "slow", &stepper{by: 1}
}

func options() run.Options {
	return run.Options{Name: "countup", Devices: run.Keyboard}
}

func binding() run.Binding[state, action, observation] {
	return run.Binding[state, action, observation]{
		Slots:             []session.SlotID{1, 2},
		Config:            config,
		NewAgent:          newAgent,
		ProtocolVersion:   "countup-1",
		EvaluationVersion: 1,
	}
}

func newRoster(t *testing.T, opts run.Options) *run.Roster[state, action, observation] {
	t.Helper()
	r, err := run.NewRoster[state, action, observation](opts, []session.SlotID{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestSeatingRules covers what a lobby and a link may and may not do to
// one roster: a seat is taken once, a person may not take more seats than
// the options allow, and a bot seat needs a controller.
func TestSeatingRules(t *testing.T) {
	r := newRoster(t, options())

	if _, err := r.JoinLocal("me"); err != nil {
		t.Fatalf("first join: %v", err)
	}
	if _, err := r.JoinLocal("also me"); err == nil {
		t.Error("a second person joined a game declaring one local seat")
	}
	if err := r.JoinRemote(1, "someone"); err == nil {
		t.Error("an occupied seat was taken again")
	}
	if err := r.Take(2, run.LocalBot, "no agent", nil); err == nil {
		t.Error("a bot seat was accepted without a controller")
	}
	if err := r.JoinRemote(9, "elsewhere"); err == nil {
		t.Error("a slot the rules never declared was seated")
	}

	if r.Complete() {
		t.Error("roster reported complete with a seat still open")
	}
	if err := r.FillBots(newAgent); err != nil {
		t.Fatal(err)
	}
	if !r.Complete() || !r.Ready() {
		t.Fatalf("after FillBots: complete=%v ready=%v", r.Complete(), r.Ready())
	}

	kinds := r.AgentKinds()
	if kinds[1] != "human" || kinds[2] != "bot" {
		t.Errorf("episode labels wrong: %v", kinds)
	}
}

// TestSharedScreenAllowsSeveralPeople checks the other half of the local
// seat rule: a game that declares a shared screen may seat more than one
// person at this machine.
func TestSharedScreenAllowsSeveralPeople(t *testing.T) {
	opts := options()
	opts.SharedScreen, opts.MaxLocalSeats = true, 2
	r := newRoster(t, opts)

	for i := range 2 {
		if _, err := r.JoinLocal("p"); err != nil {
			t.Fatalf("join %d: %v", i, err)
		}
	}
	if !r.Complete() {
		t.Error("two joins did not fill two seats")
	}
}

// TestOptionsRejectSharedSeatsWithoutSharedScreen keeps the declaration
// honest: several people at one machine share its screen by definition.
func TestOptionsRejectSharedSeatsWithoutSharedScreen(t *testing.T) {
	opts := options()
	opts.MaxLocalSeats = 2
	if err := opts.Validate(); err == nil {
		t.Error("two local seats were accepted without a shared screen")
	}
	if _, err := run.NewRoster[state, action, observation](run.Options{Name: "x"}, []session.SlotID{1}); err == nil {
		t.Error("a game declaring no input devices was accepted")
	}
}

// TestFinalizeRejectsAnIncompleteRoster proves gathering has to finish
// before a session exists (concept:match-lifecycle).
func TestFinalizeRejectsAnIncompleteRoster(t *testing.T) {
	r := newRoster(t, options())
	if _, err := r.JoinLocal("me"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Finalize(config("x", 1)); err == nil {
		t.Error("a match started with an empty seat")
	}
}

// TestLocalAgentsAreDrivenByTheWrapper is the core of Match: a realtime
// session never calls Decide itself, so a seated agent only plays if the
// wrapper pumps it. If this passes, a bot seat works; if it hangs at
// zero, nothing does.
func TestLocalAgentsAreDrivenByTheWrapper(t *testing.T) {
	r := newRoster(t, options())
	if err := r.FillBots(newAgent); err != nil {
		t.Fatal(err)
	}
	watch := run.Watch[state, action, observation](nil)
	cfg := config("driven", 1)
	cfg.Recorder = watch

	match, err := r.Finalize(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := match.Run(context.Background(), session.Unlimited); err != nil {
		t.Fatal(err)
	}
	if match.Tick() == 0 {
		t.Fatal("the match committed no ticks: nobody was driving the agents")
	}
	sig, ok := matchOutcome(watch.Outcomes(), 1)
	if !ok || sig.Terminal != session.Win {
		t.Errorf("the faster seat did not win: %+v", sig)
	}
}

// TestLocalHumanSeatTakesInputFromSubmit covers the intake half of
// api:tick-hooks: a human seat is detached, and what reaches the session
// is whatever Submit queued — the same path a remote peer uses.
func TestLocalHumanSeatTakesInputFromSubmit(t *testing.T) {
	r := newRoster(t, options())
	slot, err := r.JoinLocal("me")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FillBots(newAgent); err != nil {
		t.Fatal(err)
	}
	if seats := r.Seats(); seats[0].Kind != run.LocalHuman {
		t.Fatalf("the joined seat is %v", seats[0].Kind)
	}

	cfg := config("submitting", 1)
	// The human seat submits a huge step from the broadcast seam, which
	// stands in for a key being held every frame.
	var match *run.Match[state, action, observation]
	cfg.Broadcast = func(_ session.Tick, _ *state) {
		if match != nil {
			match.Submit(slot, action{Step: 5})
		}
	}
	watch := run.Watch[state, action, observation](nil)
	cfg.Recorder = watch

	match, err = r.Finalize(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := match.Run(context.Background(), session.Unlimited); err != nil {
		t.Fatal(err)
	}
	sig, ok := matchOutcome(watch.Outcomes(), slot)
	if !ok || sig.Terminal != session.Win {
		t.Errorf("submitted input did not reach the session: %+v", sig)
	}
}

// TestServeWritesACorpus is the headless half of the AI development
// cycle: matches nobody watched, each leaving an episode directory the
// analysis and distillation tools can read.
func TestServeWritesACorpus(t *testing.T) {
	root := t.TempDir()
	var results []run.MatchResult
	err := run.Serve(context.Background(), options(), binding(), run.ServeOptions{
		Matches: 3,
		Seed:    7,
		Time:    session.Unlimited,
		Record:  run.RecordOptions{Root: root, Mode: episode.ReplayComplete},
		OnMatch: func(res run.MatchResult) { results = append(results, res) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("played %d matches, wanted 3", len(results))
	}
	for i, res := range results {
		if res.Seed != uint64(7+i) {
			t.Errorf("match %d ran with seed %d", i, res.Seed)
		}
		if res.Ticks == 0 {
			t.Errorf("match %d committed no ticks", i)
		}
		if len(res.Outcomes) != 2 {
			t.Errorf("match %d reported %d outcomes", i, len(res.Outcomes))
		}
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("corpus holds %d episodes, wanted 3", len(entries))
	}
	// One episode is enough to prove the streams are there and carry the
	// decisions: what the corpus is for is a decision per seat per tick.
	dir := filepath.Join(root, entries[0].Name())
	for _, name := range []string{"decisions.jsonl", "events.jsonl", "outcomes.jsonl", "world.jsonl"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("episode is missing %s: %v", name, err)
		}
	}
	decisions, err := os.ReadFile(filepath.Join(dir, "decisions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(decisions)
	if !strings.Contains(body, `"agent_kind":"bot"`) {
		t.Error("decisions carry no agent kind, so a corpus cannot be filtered by who decided")
	}
	if lines := strings.Count(strings.TrimSpace(body), "\n") + 1; lines < 4 {
		t.Errorf("decisions stream holds %d lines; an episode of several ticks should hold more", lines)
	}
}

// TestServeRecordsNothingWithoutARoot keeps recording opt-in: the loop
// still runs, and nothing is written.
func TestServeRecordsNothingWithoutARoot(t *testing.T) {
	root := t.TempDir()
	err := run.Serve(context.Background(), options(), binding(), run.ServeOptions{
		Matches: 1,
		Time:    session.Unlimited,
	})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("a run with no corpus root wrote %d entries", len(entries))
	}
}

// TestServeStopsWhenCancelled covers the operator-stop path a server
// needs: cancel the context and the loop returns rather than playing on.
func TestServeStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := run.Serve(ctx, options(), binding(), run.ServeOptions{Matches: 0, Time: session.Unlimited}); err != nil {
		t.Fatalf("a cancelled run reported %v", err)
	}
}

func matchOutcome(outcomes []session.SlotOutcome, slot session.SlotID) (session.EvaluationSignal, bool) {
	for _, o := range outcomes {
		if o.Slot == slot {
			return o.Signal, true
		}
	}
	return session.EvaluationSignal{}, false
}
