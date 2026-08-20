// Command pong runs sample:pong bot vs bot over the in-process loopback:
// authoritative tick loop, snapshot/delta downstream, inputs upstream.
// A third, receive-only hub attachment feeds the score display — the
// spectator path in miniature.
//
//	pong                  # 60Hz match, up to -duration
//	pong -duration=10s -record=./episodes
//
// A human-controlled client arrives with the rendering entry point in a
// later phase; the terminal is a poor paddle controller.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/samples/pong/msg"
	"github.com/shibukawa/ebigentserver/samples/pong/pong"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
)

// spectatorID is a hub attachment that is not a player slot: it receives
// the state stream and sends nothing.
const spectatorID session.SlotID = 99

func main() {
	duration := flag.Duration("duration", 30*time.Second, "maximum match length")
	record := flag.String("record", "", "directory to write the episode log into")
	flag.Parse()

	tuning := session.TuningProfile{TickRate: 60, SendRate: 20, HistoryDepth: 8, SnapshotEvery: 30}
	hub, err := statesync.NewHub(pong.Codec(), tuning)
	if err != nil {
		fatal(err)
	}

	cfg := session.Config[pong.State, pong.Input, pong.Observation]{
		ID:        "pong-cli",
		Slots:     pong.Slots(),
		Game:      pong.Game{},
		Validator: pong.Validator{},
		Canonical: pong.Canonical,
		Tuning:    &tuning,
		Broadcast: hub.Broadcast,
	}
	if *record != "" {
		streams, closeAll, err := openStreams(*record)
		if err != nil {
			fatal(err)
		}
		defer closeAll()
		w := episode.NewWriter[pong.State, pong.Input, pong.Observation](
			streams, episode.ReplayComplete,
			episode.Meta{ProtocolVersion: msg.CBORProtocolVersion,
				AgentKinds: map[session.SlotID]string{pong.SlotLeft: "bot", pong.SlotRight: "bot"}},
		)
		cfg.Recorder = w
		defer func() {
			if err := w.Err(); err != nil {
				fatal(fmt.Errorf("recording: %w", err))
			}
			fmt.Println("episode recorded to", *record)
		}()
	}

	s, err := session.New(cfg)
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

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()
	var wg sync.WaitGroup

	for _, slot := range pong.Slots() {
		down, err := hub.Attach(slot)
		if err != nil {
			fatal(err)
		}
		inbox, err := s.Inbox(slot)
		if err != nil {
			fatal(err)
		}
		client := &pong.Client{Slot: slot, Agent: &pong.Bot{}, Inbox: inbox, Hub: hub, Down: down, Tuning: tuning}
		wg.Add(1)
		go client.Run(ctx, &wg)
	}

	spectate, err := hub.Attach(spectatorID)
	if err != nil {
		fatal(err)
	}
	wg.Add(1)
	go scoreboard(ctx, hub, spectate, tuning, &wg)

	fmt.Printf("pong: bot vs bot, %dHz, first to %d (max %v)\n", tuning.TickRate, pong.WinScore, *duration)
	if err := s.RunRealtime(ctx, session.Paced); err != nil {
		fatal(err)
	}
	hub.Close()
	cancel()
	wg.Wait()
	fmt.Printf("done after %d ticks\n", s.Tick())
}

// scoreboard renders the receive-only view.
func scoreboard(ctx context.Context, hub *statesync.Hub[pong.State, msg.PongStateDelta], down <-chan statesync.Packet, tuning session.TuningProfile, wg *sync.WaitGroup) {
	defer wg.Done()
	receiver, err := statesync.NewReceiver(pong.Codec(), tuning)
	if err != nil {
		panic(err)
	}
	var lastL, lastR uint8
	for {
		select {
		case <-ctx.Done():
			return
		case pkt, ok := <-down:
			if !ok {
				return
			}
			if err := receiver.Apply(pkt); err != nil {
				hub.RequestResync(spectatorID)
				continue
			}
			w, _, ok := receiver.State()
			if !ok {
				continue
			}
			if w.ScoreL != lastL || w.ScoreR != lastR || w.Over {
				lastL, lastR = w.ScoreL, w.ScoreR
				fmt.Printf("  %d - %d (tick %d)\n", w.ScoreL, w.ScoreR, w.Tick)
				if w.Over {
					fmt.Printf("  winner: slot %d\n", w.Winner)
					return
				}
			}
		}
	}
}

func openStreams(dir string) (episode.Streams, func(), error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return episode.Streams{}, nil, err
	}
	var files []*os.File
	open := func(name string) (*os.File, error) {
		f, err := os.Create(filepath.Join(dir, name))
		if err == nil {
			files = append(files, f)
		}
		return f, err
	}
	var s episode.Streams
	var err error
	if s.Decisions, err = open("decisions.jsonl"); err == nil {
		if s.Events, err = open("events.jsonl"); err == nil {
			if s.Outcomes, err = open("outcomes.jsonl"); err == nil {
				s.World, err = open("world.jsonl")
			}
		}
	}
	closeAll := func() {
		for _, f := range files {
			f.Close()
		}
	}
	if err != nil {
		closeAll()
		return episode.Streams{}, nil, err
	}
	return s, closeAll, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "pong:", err)
	os.Exit(1)
}
