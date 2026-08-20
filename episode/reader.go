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

// ReadReplay parses a decisions stream for replay. It fails with
// ErrNotReplayable unless the header declares replay_complete.
func ReadReplay[A any](decisions io.Reader) (*Replay[A], error) {
	sc := bufio.NewScanner(decisions)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("episode: empty decisions stream")
	}
	rep := &Replay[A]{Actions: map[session.SlotID][]A{}}
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
		slot := session.SlotID(row.Slot)
		rep.Actions[slot] = append(rep.Actions[slot], a)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return rep, nil
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
