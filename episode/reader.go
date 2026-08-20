package episode

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/shibukawa/ebigentserver/session"
)

// ErrNotReplayable rejects a log whose recording mode cannot back a
// replay (concept:episode-recording-mode: the replay consumer rejects
// incomplete or sampled logs when replay_complete is required).
var ErrNotReplayable = errors.New("episode: log is not replay_complete")

// Replay is a parsed decisions stream, ready to seat replay agents.
type Replay[A any] struct {
	Header Header
	// Actions holds each slot's accepted actions in commit order.
	Actions map[session.SlotID][]A
}

// actionRow is one accepted action parsed from the decisions stream.
type actionRow[A any] struct {
	tick   uint64
	slot   uint16
	action A
}

// parsedReplay carries the header and the accepted actions in commit
// order.
type parsedReplay[A any] struct {
	Header Header
	rows   []actionRow[A]
}

func parseReplay[A any](decisions io.Reader) (*parsedReplay[A], error) {
	sc := bufio.NewScanner(decisions)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("episode: empty decisions stream")
	}
	rep := &parsedReplay[A]{}
	if err := json.Unmarshal(sc.Bytes(), &rep.Header); err != nil {
		return nil, fmt.Errorf("episode: header: %w", err)
	}
	if rep.Header.Mode != ReplayComplete {
		return nil, fmt.Errorf("%w: mode %q", ErrNotReplayable, rep.Header.Mode)
	}
	for line := 2; sc.Scan(); line++ {
		var row Decision
		if err := json.Unmarshal(sc.Bytes(), &row); err != nil {
			return nil, fmt.Errorf("episode: decisions line %d: %w", line, err)
		}
		if len(row.Action) == 0 || string(row.Action) == "null" {
			continue // observation-only row
		}
		var a A
		if err := json.Unmarshal(row.Action, &a); err != nil {
			return nil, fmt.Errorf("episode: decisions line %d action: %w", line, err)
		}
		rep.rows = append(rep.rows, actionRow[A]{tick: row.Tick, slot: row.Slot, action: a})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return rep, nil
}

// ReadReplay parses a decisions stream for step-paced replay. It fails
// with ErrNotReplayable unless the header declares replay_complete.
func ReadReplay[A any](decisions io.Reader) (*Replay[A], error) {
	parsed, err := parseReplay[A](decisions)
	if err != nil {
		return nil, err
	}
	rep := &Replay[A]{Header: parsed.Header, Actions: map[session.SlotID][]A{}}
	for _, row := range parsed.rows {
		slot := session.SlotID(row.slot)
		rep.Actions[slot] = append(rep.Actions[slot], row.action)
	}
	return rep, nil
}

// ReadReplaySchedule parses a replay_complete decisions stream into a
// per-tick input schedule for realtime replay: the returned function has
// the session.Config.InputSource shape, feeding each recorded action back
// at the tick it was accepted on. Realtime intake accepts at most one
// input per slot per tick, so the (tick, slot) key is unique by
// construction.
func ReadReplaySchedule[A any](decisions io.Reader) (Header, func(tick session.Tick, slot session.SlotID) (A, bool), error) {
	rep, err := parseReplay[A](decisions)
	if err != nil {
		return Header{}, nil, err
	}
	type key struct {
		tick session.Tick
		slot session.SlotID
	}
	schedule := make(map[key]A, len(rep.rows))
	for _, row := range rep.rows {
		schedule[key{session.Tick(row.tick), session.SlotID(row.slot)}] = row.action
	}
	return rep.Header, func(tick session.Tick, slot session.SlotID) (A, bool) {
		a, ok := schedule[key{tick, slot}]
		return a, ok
	}, nil
}

// ReadCheckpoints extracts the checkpoint rows from an events stream, in
// order — the comparison material for replay verification.
func ReadCheckpoints(events io.Reader) ([]Event, error) {
	sc := bufio.NewScanner(events)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var cps []Event
	first := true
	for line := 1; sc.Scan(); line++ {
		if first {
			first = false // header row
			continue
		}
		var ev Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			return nil, fmt.Errorf("episode: events line %d: %w", line, err)
		}
		if ev.Kind == "checkpoint" {
			cps = append(cps, ev)
		}
	}
	return cps, sc.Err()
}
