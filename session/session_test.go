package session_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/shibukawa/fixmath"

	"github.com/shibukawa/ebigentserver/session"
)

// The test game: both slots act every step, each submitting an increment;
// the game ends when the shared total reaches Target. Illegal increments
// (<= 0) exercise the validator path.
const target = 6

type addState struct {
	Total int64
}

type addMove struct{ N int64 }

type addObs struct {
	You   session.SlotID
	Total int64
}

// addGame records every Apply into log: the commit-order witness, since
// agents themselves never see the world state.
type addGame struct{ log *[]string }

func (addGame) Start(uint64) addState { return addState{} }

func (addGame) ActingSlots(s *addState) []session.SlotID {
	if s.Total >= target {
		return nil
	}
	// Deliberately unsorted: the session must commit in slot order.
	return []session.SlotID{7, 3}
}

func (g addGame) Apply(s *addState, slot session.SlotID, m addMove) {
	s.Total += m.N
	if g.log != nil {
		*g.log = append(*g.log, fmt.Sprintf("slot%d+%d=%d", slot, m.N, s.Total))
	}
}

func (addGame) Project(s *addState, slot session.SlotID) addObs {
	return addObs{You: slot, Total: s.Total}
}

func (addGame) Evaluate(s *addState, slot session.SlotID) session.EvaluationSignal {
	sig := session.EvaluationSignal{Score: s.Total}
	if s.Total >= target {
		sig.Terminal = session.Draw
	}
	return sig
}

type addValidator struct{}

func (addValidator) Legal(s *addState, slot session.SlotID, m addMove) error {
	if m.N <= 0 {
		return fmt.Errorf("increment must be positive, got %d", m.N)
	}
	return nil
}

// addAgent submits scripted increments and records every callback, so
// tests can assert exact call order.
type addAgent struct {
	moves  []addMove
	events []string
	slot   session.SlotID
}

func (a *addAgent) Joined(slot session.SlotID) {
	a.slot = slot
	a.events = append(a.events, "joined")
}

func (a *addAgent) Observe(o addObs) {
	a.events = append(a.events, fmt.Sprintf("observe:%d", o.Total))
}

func (a *addAgent) Decide(context.Context) (addMove, bool) {
	if len(a.moves) == 0 {
		return addMove{}, false
	}
	m := a.moves[0]
	a.moves = a.moves[1:]
	a.events = append(a.events, fmt.Sprintf("decide:%d", m.N))
	return m, true
}

func (a *addAgent) Ended(r session.Result) {
	a.events = append(a.events, fmt.Sprintf("ended:%v:%v", r.State, r.Signal.Terminal))
}

type recordSink struct{ reports []session.Report }

func (r *recordSink) Report(rep session.Report) error {
	r.reports = append(r.reports, rep)
	return nil
}

func newAddSession(t *testing.T, a3, a7 *addAgent, sink session.ReportSink, log *[]string) *session.Session[addState, addMove, addObs] {
	t.Helper()
	s, err := session.New(session.Config[addState, addMove, addObs]{
		ID:        "add-test",
		Slots:     []session.SlotID{7, 3}, // unsorted on purpose
		RuleSet:   addGame{log: log},
		Validator: addValidator{},
		Reports:   sink,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OpenAdmission(); err != nil {
		t.Fatal(err)
	}
	// Admission order is the reverse of slot order; commit order must
	// not care.
	if err := s.Admit(7, a7); err != nil {
		t.Fatal(err)
	}
	if err := s.Admit(3, a3); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLifecycleGuards(t *testing.T) {
	s, err := session.New(session.Config[addState, addMove, addObs]{
		Slots: []session.SlotID{3, 7}, RuleSet: addGame{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.State() != session.StateCreated {
		t.Fatalf("state = %v, want created", s.State())
	}
	// Admission is forbidden in created.
	if err := s.Admit(3, &addAgent{}); err == nil {
		t.Fatal("Admit in created must fail")
	}
	// Run is forbidden in created.
	if err := s.Run(context.Background()); err == nil {
		t.Fatal("Run in created must fail")
	}
	if err := s.OpenAdmission(); err != nil {
		t.Fatal(err)
	}
	// Run with an empty slot is forbidden.
	if err := s.Admit(3, &addAgent{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(context.Background()); err == nil {
		t.Fatal("Run with empty slot must fail")
	}
	// Unknown slot and double admission.
	if err := s.Admit(5, &addAgent{}); err == nil {
		t.Fatal("Admit of unknown slot must fail")
	}
	if err := s.Admit(3, &addAgent{}); err == nil {
		t.Fatal("double Admit must fail")
	}
}

func TestConfigValidation(t *testing.T) {
	if _, err := session.New(session.Config[addState, addMove, addObs]{Slots: []session.SlotID{1}}); err == nil {
		t.Fatal("nil game must fail")
	}
	if _, err := session.New(session.Config[addState, addMove, addObs]{RuleSet: addGame{}}); err == nil {
		t.Fatal("empty slots must fail")
	}
	if _, err := session.New(session.Config[addState, addMove, addObs]{RuleSet: addGame{}, Slots: []session.SlotID{0, 1}}); err == nil {
		t.Fatal("slot 0 must fail")
	}
	if _, err := session.New(session.Config[addState, addMove, addObs]{RuleSet: addGame{}, Slots: []session.SlotID{2, 2}}); err == nil {
		t.Fatal("duplicate slot must fail")
	}
}

func TestFullRunCommitsInSlotOrder(t *testing.T) {
	a3 := &addAgent{moves: []addMove{{1}, {1}, {1}}}
	a7 := &addAgent{moves: []addMove{{1}, {1}, {1}}}
	sink := &recordSink{}
	var log []string
	s := newAddSession(t, a3, a7, sink, &log)
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.State() != session.StateEnded {
		t.Fatalf("state = %v, want ended", s.State())
	}
	if s.Tick() != 3 {
		t.Fatalf("tick = %d, want 3", s.Tick())
	}

	// Slot 3 commits before slot 7 every step, although the game lists
	// acting slots unsorted and slot 7 was admitted first
	// (rule:deterministic-tick-commit).
	wantLog := "slot3+1=1 slot7+1=2 slot3+1=3 slot7+1=4 slot3+1=5 slot7+1=6"
	if got := strings.Join(log, " "); got != wantLog {
		t.Errorf("commit log:\n got %s\nwant %s", got, wantLog)
	}

	// Each agent observed the tick-opening totals and its own callbacks
	// in contract order.
	want := "joined observe:0 decide:1 observe:2 decide:1 observe:4 decide:1 observe:6 ended:ended:draw"
	if got := strings.Join(a3.events, " "); got != want {
		t.Errorf("slot 3 transcript:\n got %s\nwant %s", got, want)
	}
	if got := strings.Join(a7.events, " "); got != want {
		t.Errorf("slot 7 transcript:\n got %s\nwant %s", got, want)
	}

	// Terminal reporting seam: slot outcomes in slot order, then the
	// terminal report, with a monotonic sequence.
	if len(sink.reports) != 3 {
		t.Fatalf("reports = %d, want 3", len(sink.reports))
	}
	for i, r := range sink.reports {
		if r.Seq != uint64(i+1) || r.SessionID != "add-test" {
			t.Errorf("report %d: seq=%d id=%q", i, r.Seq, r.SessionID)
		}
	}
	if sink.reports[0].Subject != 3 || sink.reports[1].Subject != 7 {
		t.Errorf("outcome subjects = %d, %d; want 3, 7", sink.reports[0].Subject, sink.reports[1].Subject)
	}
	last := sink.reports[2]
	if !last.Terminal || last.Kind != "session_ended" {
		t.Errorf("last report = %+v, want terminal session_ended", last)
	}
}

func TestRunIsDeterministic(t *testing.T) {
	run := func() []string {
		a3 := &addAgent{moves: []addMove{{1}, {1}, {1}}}
		a7 := &addAgent{moves: []addMove{{1}, {1}, {1}}}
		s := newAddSession(t, a3, a7, nil, nil)
		if err := s.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		return append(a3.events, a7.events...)
	}
	first, second := run(), run()
	if strings.Join(first, "|") != strings.Join(second, "|") {
		t.Fatalf("two identical runs diverged:\n%v\n%v", first, second)
	}
}

func TestIllegalActionRetriesThenSucceeds(t *testing.T) {
	// Slot 3's first submission is illegal; the retry consumes the next
	// scripted move without touching the state.
	a3 := &addAgent{moves: []addMove{{-5}, {1}, {1}, {1}}}
	a7 := &addAgent{moves: []addMove{{1}, {1}, {1}}}
	s := newAddSession(t, a3, a7, nil, nil)
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.State() != session.StateEnded {
		t.Fatalf("state = %v, want ended", s.State())
	}
	// The illegal -5 never reached Apply: totals still climb 2 per tick.
	want := "joined observe:0 decide:-5 decide:1 observe:2 decide:1 observe:4 decide:1 observe:6 ended:ended:draw"
	if got := strings.Join(a3.events, " "); got != want {
		t.Errorf("transcript:\n got %s\nwant %s", got, want)
	}
}

func TestIllegalActionsExhaustBudgetAndAbort(t *testing.T) {
	a3 := &addAgent{moves: []addMove{{-1}, {-1}, {-1}, {-1}, {-1}, {-1}}}
	a7 := &addAgent{moves: []addMove{{1}, {1}, {1}}}
	s := newAddSession(t, a3, a7, nil, nil)
	err := s.Run(context.Background())
	if err == nil {
		t.Fatal("Run must fail when the retry budget is exhausted")
	}
	if s.State() != session.StateAborted {
		t.Fatalf("state = %v, want aborted", s.State())
	}
	last := a3.events[len(a3.events)-1]
	if last != "ended:aborted:abandoned" {
		t.Errorf("final agent event = %q, want ended:aborted:abandoned", last)
	}
}

func TestNoActionDrainsAsAbandoned(t *testing.T) {
	// Slot 3 runs out of moves mid-game: the session drains and every
	// unfinished slot ends Abandoned.
	a3 := &addAgent{moves: []addMove{{1}}}
	a7 := &addAgent{moves: []addMove{{1}, {1}, {1}}}
	s := newAddSession(t, a3, a7, nil, nil)
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.State() != session.StateEnded {
		t.Fatalf("state = %v, want ended", s.State())
	}
	last := a3.events[len(a3.events)-1]
	if last != "ended:ended:abandoned" {
		t.Errorf("final agent event = %q, want ended:ended:abandoned", last)
	}
}

func TestCancelledContextDrainsAsAbandoned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a3 := &addAgent{moves: []addMove{{1}, {1}, {1}}}
	a7 := &addAgent{moves: []addMove{{1}, {1}, {1}}}
	s := newAddSession(t, a3, a7, nil, nil)
	if err := s.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if s.State() != session.StateEnded {
		t.Fatalf("state = %v, want ended (operator stop drains, not aborts)", s.State())
	}
	last := a3.events[len(a3.events)-1]
	if last != "ended:ended:abandoned" {
		t.Errorf("final agent event = %q, want ended:ended:abandoned", last)
	}
}

func TestStateTransitionTable(t *testing.T) {
	allowed := map[[2]session.State]bool{
		{session.StateCreated, session.StateAdmitting}: true,
		{session.StateAdmitting, session.StateRunning}: true,
		{session.StateRunning, session.StateDraining}:  true,
		{session.StateDraining, session.StateEnded}:    true,
		{session.StateCreated, session.StateAborted}:   true,
		{session.StateAdmitting, session.StateAborted}: true,
		{session.StateRunning, session.StateAborted}:   true,
		{session.StateDraining, session.StateAborted}:  true,
	}
	states := []session.State{
		session.StateCreated, session.StateAdmitting, session.StateRunning,
		session.StateDraining, session.StateEnded, session.StateAborted,
	}
	for _, from := range states {
		for _, to := range states {
			want := allowed[[2]session.State{from, to}]
			if got := from.CanTransition(to); got != want {
				t.Errorf("CanTransition(%v, %v) = %v, want %v", from, to, got, want)
			}
		}
	}
}

// TestTerminalReadsBothSpellings is what lets the recorded format change
// without the recorded data changing. Episodes written before the
// outcome had a name hold a bare number in that column; they are still
// evidence, and a reader that could not open them would turn a change to
// the format into a loss of the corpus.
func TestTerminalReadsBothSpellings(t *testing.T) {
	for _, tc := range []struct {
		json string
		want session.Terminal
	}{
		{`"not_terminal"`, session.NotTerminal},
		{`"win"`, session.Win},
		{`"lose"`, session.Lose},
		{`"draw"`, session.Draw},
		{`"abandoned"`, session.Abandoned},
		{`0`, session.NotTerminal},
		{`1`, session.Win},
		{`2`, session.Lose},
		{`3`, session.Draw},
		{`4`, session.Abandoned},
	} {
		var got session.Terminal
		if err := json.Unmarshal([]byte(tc.json), &got); err != nil {
			t.Fatalf("%s: %v", tc.json, err)
		}
		if got != tc.want {
			t.Fatalf("%s decoded to %v, want %v", tc.json, got, tc.want)
		}
	}
	for _, bad := range []string{`"winner"`, `9`, `"1"`, `{}`} {
		var got session.Terminal
		if err := json.Unmarshal([]byte(bad), &got); err == nil {
			t.Fatalf("%s decoded to %v instead of failing", bad, got)
		}
	}
	// null leaves the field alone rather than failing, which is what
	// every other stdlib decoder does with an absent value.
	got := session.Win
	if err := json.Unmarshal([]byte(`null`), &got); err != nil || got != session.Win {
		t.Fatalf("null gave %v, %v", got, err)
	}
}

// TestSignalRoundTripsThroughItsRecordedForm covers the whole struct: a
// signal written to a decision row and read back is the same signal, and
// the column names are the ones the rest of the log uses.
func TestSignalRoundTripsThroughItsRecordedForm(t *testing.T) {
	want := session.EvaluationSignal{
		Score:       7,
		Progress:    fixmath.FromInt32(1),
		Evaluation:  fixmath.FromInt32(-2),
		RewardDelta: fixmath.FromInt32(3),
		Terminal:    session.Draw,
	}
	body, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{`"score"`, `"progress"`, `"evaluation"`, `"reward_delta"`, `"terminal":"draw"`} {
		if !strings.Contains(string(body), column) {
			t.Fatalf("recorded signal %s is missing %s", body, column)
		}
	}
	var got session.EvaluationSignal
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round trip gave %+v, want %+v", got, want)
	}
}
