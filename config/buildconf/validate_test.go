package buildconf

import (
	"strings"
	"testing"
	"time"
)

func valid() *Config {
	return &Config{
		Project: &Project{Module: "example.com/game"},
		Protocol: &Protocol{
			Shape: "duo", Realtime: "paced", View: "shared", Sync: "rollback",
			Devices: []string{"keyboard"},
			Seats:   Seats{Count: 2, Fill: "bots", Occupant: "any"},
		},
		Build:    &Build{Target: []Target{{Name: "server", Kind: "dedicated", Entry: "./cmd/server"}}},
		Dev:      &Dev{Debounce: 200 * time.Millisecond, Console: "127.0.0.1:8930"},
		Behavior: &Behavior{Library: "behavior/chips.json", Corpus: "corpus"},
	}
}

func TestValidAndDevTargetResolves(t *testing.T) {
	c := valid()
	if err := c.Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	// One declared target needs no dev.target to be unambiguous.
	got, ok := c.DevTarget()
	if !ok || got.Name != "server" {
		t.Errorf("DevTarget = %+v, %v", got, ok)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"no module", func(c *Config) { c.Project.Module = "" }, "project.module"},
		{"no targets", func(c *Config) { c.Build.Target = nil }, "at least one"},
		{"target without a name", func(c *Config) { c.Build.Target[0].Name = "" }, "name is required"},
		{"target without an entry", func(c *Config) { c.Build.Target[0].Entry = "" }, "entry package"},
		{"unknown target kind", func(c *Config) { c.Build.Target[0].Kind = "headless" }, "kind"},
		{"duplicate target names", func(c *Config) {
			c.Build.Target = append(c.Build.Target, Target{Name: "server", Kind: "client", Entry: "./cmd/other"})
			c.Dev.Target = "server"
		}, "repeats the name"},
		{"dev target names nothing", func(c *Config) { c.Dev.Target = "client" }, "names no declared target"},
		{"ambiguous dev target", func(c *Config) {
			c.Build.Target = append(c.Build.Target, Target{Name: "client", Kind: "client", Entry: "./cmd/client"})
		}, "dev.target is required"},
		{"negative debounce", func(c *Config) { c.Dev.Debounce = -time.Second }, "debounce"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.mut(c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("want an error naming %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to name %q", err, tc.want)
			}
		})
	}
}

// A dev target links api:dev-debug-endpoint and must never ship
// (rule:debug-endpoint-excluded-from-release). Declaring it is legal; the
// linkage check that enforces it lives in the import graph, not here.
func TestDevTargetFlagIsCarriedThrough(t *testing.T) {
	c := valid()
	c.Build.Target[0].Dev = true
	if err := c.Validate(); err != nil {
		t.Fatalf("a dev target is a legal declaration: %v", err)
	}
	got, ok := c.DevTarget()
	if !ok || !got.Dev {
		t.Errorf("DevTarget = %+v, %v", got, ok)
	}
}

// TestValidateRejectsProtocol covers the game's own contract. Every case
// here would otherwise reach ebigent generate and be emitted as a
// constant, which is why they fail at the command instead.
func TestValidateRejectsProtocol(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"unknown shape", func(c *Config) { c.Protocol.Shape = "quartet" }, "protocol.shape"},
		{"unknown realtime tier", func(c *Config) { c.Protocol.Realtime = "brisk" }, "protocol.realtime"},
		{"unknown view", func(c *Config) { c.Protocol.View = "split" }, "protocol.view"},
		{"unknown sync mode", func(c *Config) { c.Protocol.Sync = "optimistic" }, "protocol.sync"},
		{"unknown device", func(c *Config) { c.Protocol.Devices = []string{"tongue"} }, "protocol.devices[0]"},
		{"no device at all", func(c *Config) { c.Protocol.Devices = nil }, "nobody can control"},
		{"unknown fill policy", func(c *Config) { c.Protocol.Seats.Fill = "maybe" }, "seats.fill"},
		{"unknown occupant", func(c *Config) { c.Protocol.Seats.Occupant = "ghost" }, "seats.occupant"},
		{"shape and count disagree", func(c *Config) { c.Protocol.Seats.Count = 5 }, "declares 2 seat"},
		{
			"no seats at all",
			func(c *Config) { c.Protocol.Shape, c.Protocol.Seats.Count = "multi", 0 },
			"at least 1",
		},
		{
			"more seats than the placeholder renders",
			func(c *Config) { c.Protocol.Shape, c.Protocol.Seats.Count = "multi", 9 },
			"declare the slots by hand",
		},
		{
			"a solo game naming a sync mode",
			func(c *Config) {
				c.Protocol.Shape, c.Protocol.Seats.Count = "solo", 1
				c.Protocol.Sync = "rollback"
			},
			"has no link",
		},
		{
			"a one-hop netcode past two seats",
			func(c *Config) {
				c.Protocol.Shape, c.Protocol.Seats.Count = "multi", 4
				c.Protocol.Sync = "rollback"
			},
			"needs a one-hop link",
		},
		{
			"a team with no name",
			func(c *Config) { c.Protocol.Team = []Team{{Seats: 2}} },
			"team[0].name",
		},
		{
			"two teams with one name",
			func(c *Config) {
				c.Protocol.Shape, c.Protocol.Seats.Count = "multi", 4
				c.Protocol.Sync = "server_authoritative"
				c.Protocol.Team = []Team{{Name: "red", Seats: 2}, {Name: "red", Seats: 2}}
			},
			"repeats the name",
		},
		{
			"a team holding nobody",
			func(c *Config) { c.Protocol.Team = []Team{{Name: "red", Seats: 2}, {Name: "blue", Seats: 0}} },
			"at least one seat",
		},
		{
			"teams that do not account for every seat",
			func(c *Config) { c.Protocol.Team = []Team{{Name: "red", Seats: 1}} },
			"account for 1 seats",
		},
		{
			"a team wanting an unknown occupant",
			func(c *Config) {
				c.Protocol.Team = []Team{{Name: "red", Seats: 1, Occupant: "ghost"}, {Name: "blue", Seats: 1}}
			},
			"occupant",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.mut(c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("accepted %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// TestProtocolDefaults covers the values a project may leave unsaid: the
// identity falls back to the module and the title to the package's last
// element, since a title identifies nothing and an absent one is no error.
func TestProtocolDefaults(t *testing.T) {
	c := valid()
	if got := c.GamePackage(); got != "example.com/game" {
		t.Errorf("GamePackage = %q, want the module path", got)
	}
	if got := c.GameTitle(); got != "game" {
		t.Errorf("GameTitle = %q, want the last path element", got)
	}

	// A monorepo names the game below the module, and the identity has
	// to follow the subpath or two games share one.
	c.Protocol.Package = "example.com/arcade/pong"
	if got := c.GamePackage(); got != "example.com/arcade/pong" {
		t.Errorf("GamePackage = %q, want the declared path", got)
	}
	if got := c.GameTitle(); got != "pong" {
		t.Errorf("GameTitle = %q, want the last path element", got)
	}
	c.Protocol.Title = "Pong!"
	if got := c.GameTitle(); got != "Pong!" {
		t.Errorf("GameTitle = %q, want the declared title", got)
	}
}

// TestSeatDivision covers what a seat inherits from its team. A team
// narrows the table's answer and never widens it, and a game with no
// teams answers the same for every seat.
func TestSeatDivision(t *testing.T) {
	c := valid()
	c.Protocol.Shape, c.Protocol.Seats.Count, c.Protocol.Sync = "multi", 4, "server_authoritative"
	c.Protocol.Seats.Occupant = "any"
	c.Protocol.Team = []Team{
		{Name: "red", Seats: 2, Occupant: "human"},
		{Name: "blue", Seats: 2},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid division: %v", err)
	}
	for seat, want := range map[int]string{0: "red", 1: "red", 2: "blue", 3: "blue"} {
		if got, ok := c.TeamOf(seat); !ok || got != want {
			t.Errorf("seat %d is on %q (%v), want %q", seat, got, ok, want)
		}
	}
	for seat, want := range map[int]string{0: "human", 1: "human", 2: "any", 3: "any"} {
		if got := c.SeatOccupant(seat); got != want {
			t.Errorf("seat %d admits %q, want %q", seat, got, want)
		}
	}

	// No teams means no division to report, and the table's answer
	// reaches every seat unchanged.
	c.Protocol.Team = nil
	c.Protocol.Seats.Occupant = "bot"
	if _, ok := c.TeamOf(0); ok {
		t.Error("a game with no teams reported one")
	}
	if got := c.SeatOccupant(3); got != "bot" {
		t.Errorf("seat 3 admits %q, want the table's bot", got)
	}
}
