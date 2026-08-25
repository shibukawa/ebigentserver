package run_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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

type sight struct {
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

func (rules) Project(s *state, slot session.SlotID) sight {
	return sight{You: slot, Mine: s.Count[slot-1], Theirs: s.Count[2-slot]}
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
func (*stepper) Observe(sight)                           {}
func (s *stepper) Decide(context.Context) (action, bool) { return action{Step: s.by}, true }
func (*stepper) Ended(session.Result)                    {}

func config(id string, seed uint64) session.Config[state, action, sight] {
	tuning := session.TuningProfile{TickRate: 60, SendRate: 60, HistoryDepth: 1}
	return session.Config[state, action, sight]{
		ID:        id,
		Slots:     []session.SlotID{1, 2},
		RuleSet:   rules{},
		Seed:      seed,
		Tuning:    &tuning,
		Canonical: func(s *state) []byte { b, _ := json.Marshal(s); return b },
	}
}

func newAgent(slot session.SlotID) (string, session.Agent[sight, action]) {
	if slot == 1 {
		return "fast", &stepper{by: 2}
	}
	return "slow", &stepper{by: 1}
}

func options() run.Options {
	return run.Options{Name: "countup", Devices: run.Keyboard}
}

func binding() run.Binding[state, action, sight] {
	return run.Binding[state, action, sight]{
		Slots:    []session.SlotID{1, 2},
		Config:   config,
		NewAgent: newAgent,
		// The same two kinds NewAgent chooses between, named so a run
		// can ask for one, plus a third nothing chooses by default.
		Agents: map[string]func(uint64) session.Agent[sight, action]{
			"fast":  func(uint64) session.Agent[sight, action] { return &stepper{by: 2} },
			"slow":  func(uint64) session.Agent[sight, action] { return &stepper{by: 1} },
			"crawl": func(uint64) session.Agent[sight, action] { return &stepper{by: 1} },
		},
		ProtocolVersion:   "countup-1",
		EvaluationVersion: 1,
	}
}

func newRoster(t *testing.T, opts run.Options) *run.Roster[state, action, sight] {
	t.Helper()
	r, err := run.NewRoster[state, action, sight](opts, []session.SlotID{1, 2})
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

	if _, err := r.SitLocal("me"); err != nil {
		t.Fatalf("first join: %v", err)
	}
	if _, err := r.SitLocal("also me"); err == nil {
		t.Error("a second person guest a game declaring one local seat")
	}
	if err := r.SitRemote(1, "someone"); err == nil {
		t.Error("an occupied seat was taken again")
	}
	if err := r.Sit(2, run.Bot, true, "no agent", nil); err == nil {
		t.Error("a bot seat was accepted without a controller")
	}
	if err := r.SitRemote(9, "elsewhere"); err == nil {
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

	// A person is "human" and never their name; a bot is the policy it
	// runs, because that is the axis a corpus is filtered on.
	kinds := r.AgentKinds()
	if kinds[1] != "human" || kinds[2] != "slow" {
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
		if _, err := r.SitLocal("p"); err != nil {
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
	if _, err := run.NewRoster[state, action, sight](run.Options{Name: "x"}, []session.SlotID{1}); err == nil {
		t.Error("a game declaring no input devices was accepted")
	}
}

// TestFinalizeRejectsAnIncompleteRoster proves gathering has to finish
// before a session exists (concept:match-lifecycle).
func TestFinalizeRejectsAnIncompleteRoster(t *testing.T) {
	r := newRoster(t, options())
	if _, err := r.SitLocal("me"); err != nil {
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
	watch := run.Watch[state, action, sight](nil)
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
	slot, err := r.SitLocal("me")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FillBots(newAgent); err != nil {
		t.Fatal(err)
	}
	if seats := r.Seats(); !seats[0].LocalHuman() {
		t.Fatalf("the guest seat is %v (local=%v)", seats[0].Kind, seats[0].Local)
	}

	cfg := config("submitting", 1)
	// The human seat submits a huge step from the broadcast seam, which
	// stands in for a key being held every frame.
	var match *run.Match[state, action, sight]
	cfg.Broadcast = func(_ session.Tick, _ *state) {
		if match != nil {
			match.Submit(slot, action{Step: 5})
		}
	}
	watch := run.Watch[state, action, sight](nil)
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
	// The label is the policy the seat ran, not the fact that it was a
	// bot: a corpus is filtered by who decided, and "bot" answers that
	// for a game with one kind and for no other.
	if !strings.Contains(body, `"agent_kind":"fast"`) || !strings.Contains(body, `"agent_kind":"slow"`) {
		t.Errorf("decisions do not carry the policy each seat ran:\n%s", body)
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

// TestResumeIndexSkipsWhatTheCorpusAlreadyHolds fixes the property that
// makes a corpus worth keeping across launches: a second run adds to it.
//
// Without this, the second launch of a game numbers its first match zero
// again and os.Create truncates — so playing three matches, quitting,
// and playing three more leaves three episodes rather than six, with the
// same seeds recorded on top of themselves.
func TestResumeIndexSkipsWhatTheCorpusAlreadyHolds(t *testing.T) {
	root := t.TempDir()
	if got := run.ResumeIndex(root, "race", 0); got != 0 {
		t.Fatalf("an empty corpus resumed at %d, want 0", got)
	}
	for _, id := range []string{"race-0000", "race-0001", "race-0002"} {
		if err := os.MkdirAll(filepath.Join(root, id), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := run.ResumeIndex(root, "race", 0); got != 3 {
		t.Fatalf("resumed at %d, want 3", got)
	}
	// A second game shares the root without colliding: the name is part
	// of the episode id, so one corpus can hold several of them.
	if got := run.ResumeIndex(root, "chase", 0); got != 0 {
		t.Fatalf("another game resumed at %d, want 0", got)
	}
	// Not recording is not a reason to go looking at the filesystem.
	if got := run.ResumeIndex("", "race", 7); got != 7 {
		t.Fatalf("a run with no corpus root resumed at %d, want 7", got)
	}
}

// TestRecordingsAccumulateAcrossRuns is the same claim through the calls
// a launch actually makes: open an episode, close it, resume, open the
// next. Three launches leave three episodes and none of them is a
// rewrite of an earlier one.
func TestRecordingsAccumulateAcrossRuns(t *testing.T) {
	root := t.TempDir()
	seen := map[string]bool{}
	for launch := 0; launch < 3; launch++ {
		index := run.ResumeIndex(root, "race", 0)
		rec, err := run.OpenRecording[state, action, sight](run.RecordOptions{
			Root:      root,
			EpisodeID: run.EpisodeID("race", index),
		})
		if err != nil {
			t.Fatalf("launch %d: %v", launch, err)
		}
		if seen[rec.Dir] {
			t.Fatalf("launch %d reopened %s, which an earlier launch had already recorded", launch, rec.Dir)
		}
		seen[rec.Dir] = true
		if err := rec.Close(); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("three launches left %d episodes, want 3", len(entries))
	}
}

// A game with several kinds of bot has to be able to record one of them
// at a time. Mixing three pursuit styles into one corpus distills into a
// policy none of them had, so which kind is playing is a property of the
// run rather than of the rules.
func TestServeSeatsTheAgentTheRunAsksFor(t *testing.T) {
	root := t.TempDir()
	agents, err := run.ParseAgents("crawl")
	if err != nil {
		t.Fatal(err)
	}
	if err := run.Serve(t.Context(), options(), binding(), run.ServeOptions{
		Agents:  agents,
		Matches: 1,
		Seed:    1,
		Time:    session.Unlimited,
		Record:  run.RecordOptions{Root: root, Mode: episode.ReplayComplete},
	}); err != nil {
		t.Fatal(err)
	}
	// The name is what labels the seat in the log, which is the whole
	// reason a corpus can be split by kind later.
	kinds := recordedKinds(t, filepath.Join(root, "countup-0000", "decisions.jsonl"))
	for _, slot := range []string{"1", "2"} {
		if kinds[slot] != "crawl" {
			t.Errorf("slot %s recorded as %q, want crawl; kinds = %v", slot, kinds[slot], kinds)
		}
	}
}

// Naming one seat names one seat. Every other bot seat is still the
// game's own choice, because an assignment seeds a roster rather than
// replacing what fills it.
func TestNamingOneSeatLeavesTheRestToTheGame(t *testing.T) {
	root := t.TempDir()
	agents, err := run.ParseAgents("2=crawl")
	if err != nil {
		t.Fatal(err)
	}
	if err := run.Serve(t.Context(), options(), binding(), run.ServeOptions{
		Agents:  agents,
		Matches: 1,
		Seed:    1,
		Time:    session.Unlimited,
		Record:  run.RecordOptions{Root: root, Mode: episode.ReplayComplete},
	}); err != nil {
		t.Fatal(err)
	}
	kinds := recordedKinds(t, filepath.Join(root, "countup-0000", "decisions.jsonl"))
	if kinds["1"] != "fast" {
		t.Errorf("slot 1 = %q, want the game's own choice of fast", kinds["1"])
	}
	if kinds["2"] != "crawl" {
		t.Errorf("slot 2 = %q, want the crawl the run asked for", kinds["2"])
	}
}

// A run asking for an agent the game does not have would record a corpus
// under a label nothing in it earned, so it fails instead — naming what
// there was to ask for.
func TestAnUnknownAgentIsRefusedWithWhatIsDeclared(t *testing.T) {
	err := run.Serve(t.Context(), options(), binding(), run.ServeOptions{
		Agents:  map[session.SlotID]string{1: "sprint"},
		Matches: 1,
		Seed:    1,
		Time:    session.Unlimited,
	})
	if err == nil {
		t.Fatal("want an error for an agent the binding does not name")
	}
	for _, want := range []string{"sprint", "crawl, fast, slow"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to mention %q", err, want)
		}
	}
}

// The assignment travels as one value because an array of tables in
// data:run-config has the file as its only source, so these are the
// forms that have to survive a trip through an environment variable.
func TestParseAgentsReadsBothForms(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want map[session.SlotID]string
		fail bool
	}{
		{name: "empty", in: "  "},
		{name: "one name is every bot seat", in: "chaser", want: map[session.SlotID]string{run.AnySlot: "chaser"}},
		{name: "per seat", in: "2=chaser, 3=flanker", want: map[session.SlotID]string{2: "chaser", 3: "flanker"}},
		{name: "a default with one seat against it", in: "chaser,1=runner", want: map[session.SlotID]string{run.AnySlot: "chaser", 1: "runner"}},
		{name: "no name", in: "2=", fail: true},
		{name: "no slot", in: "x=chaser", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := run.ParseAgents(tc.in)
			if tc.fail {
				if err == nil {
					t.Fatalf("ParseAgents(%q) = %v, want an error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ParseAgents(%q) = %v, want %v", tc.in, got, tc.want)
			}
			for slot, name := range tc.want {
				if got[slot] != name {
					t.Errorf("slot %d = %q, want %q", slot, got[slot], name)
				}
			}
		})
	}
}

// recordedKinds reads the agent_kind column back out of a decisions
// stream, keyed by slot.
func recordedKinds(t *testing.T, path string) map[string]string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		var row struct {
			Slot      int    `json:"slot"`
			AgentKind string `json:"agent_kind"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil || row.AgentKind == "" {
			continue
		}
		out[strconv.Itoa(row.Slot)] = row.AgentKind
	}
	return out
}
