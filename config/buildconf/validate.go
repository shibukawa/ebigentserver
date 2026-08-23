package buildconf

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Target kinds accepted in a [[build.target]] block, matching the
// concept:build-target matrix. An element of an array of tables cannot
// carry an enum tag, so the allowlist lives here.
var targetKinds = []string{"client", "listen", "dedicated", "simulation"}

// Validate checks the toolchain declaration. It is what ebigent runs
// before build, dev, or generate touches anything, so a malformed
// project fails at the command rather than inside a compiler error.
func (c *Config) Validate() error {
	var errs []error

	if c.Project.Module == "" {
		errs = append(errs, errors.New("project.module is required"))
	}

	errs = append(errs, c.validateProtocol()...)

	if len(c.Build.Target) == 0 {
		errs = append(errs, errors.New("declare at least one [[build.target]]"))
	}
	names := map[string]bool{}
	for i, t := range c.Build.Target {
		switch {
		case t.Name == "":
			errs = append(errs, fmt.Errorf("build.target[%d].name is required", i))
		case names[t.Name]:
			errs = append(errs, fmt.Errorf("build.target[%d] repeats the name %q", i, t.Name))
		}
		names[t.Name] = true
		if t.Entry == "" {
			errs = append(errs, fmt.Errorf("build.target[%d] (%s) needs an entry package", i, t.Name))
		}
		if !slices.Contains(targetKinds, t.Kind) {
			errs = append(errs, fmt.Errorf("build.target[%d] (%s) kind %q is not one of %v", i, t.Name, t.Kind, targetKinds))
		}
	}

	switch {
	case c.Dev.Target != "" && !names[c.Dev.Target]:
		errs = append(errs, fmt.Errorf("dev.target %q names no declared target", c.Dev.Target))
	case c.Dev.Target == "" && len(c.Build.Target) > 1:
		errs = append(errs, errors.New("dev.target is required when several targets are declared"))
	}
	if c.Dev.Debounce < 0 {
		errs = append(errs, errors.New("dev.debounce must not be negative"))
	}

	if len(errs) > 0 {
		return fmt.Errorf("buildconf: invalid project configuration: %w", errors.Join(errs...))
	}
	return nil
}

// DevTarget returns the target ebigent dev builds and runs. Validate has
// already established that the choice is unambiguous.
func (c *Config) DevTarget() (Target, bool) {
	if c.Dev.Target == "" {
		if len(c.Build.Target) == 1 {
			return c.Build.Target[0], true
		}
		return Target{}, false
	}
	for _, t := range c.Build.Target {
		if t.Name == c.Dev.Target {
			return t, true
		}
	}
	return Target{}, false
}

// Values accepted in a [protocol] field. configbind's enum tag reaches
// neither the generated code nor the loader (decision:configbind-for-all-config),
// so every allowlist is restated here and checked before a build runs.
var (
	protocolShapes    = []string{"solo", "duo", "multi"}
	protocolRealtime  = []string{"turn_based", "paced", "twitch"}
	protocolViews     = []string{"shared", "per_agent"}
	protocolSyncModes = []string{"server_authoritative", "delay", "rollback", "hybrid"}
	protocolDevices   = []string{"keyboard", "mouse", "gamepad"}
	seatFills         = []string{"bots", "empty"}
	seatOccupants     = []string{"any", "human", "bot"}
	conditionMatches  = []string{"exact", "band"}
)

// seatsForShape reports the seat count a concept:participant-shape fixes,
// and whether it fixes one at all. Only multi leaves the count open.
func seatsForShape(shape string) (int, bool) {
	switch shape {
	case "solo":
		return 1, true
	case "duo":
		return 2, true
	default:
		return 0, false
	}
}

// validateProtocol checks the game's own contract. Everything it rejects
// would otherwise reach ebigent generate and become a constant.
func (c *Config) validateProtocol() []error {
	p := c.Protocol
	var errs []error

	for _, check := range []struct {
		field, value string
		allowed      []string
	}{
		{"shape", p.Shape, protocolShapes},
		{"realtime", p.Realtime, protocolRealtime},
		{"view", p.View, protocolViews},
		{"sync", p.Sync, protocolSyncModes},
		{"seats.fill", p.Seats.Fill, seatFills},
		{"seats.occupant", p.Seats.Occupant, seatOccupants},
	} {
		if !slices.Contains(check.allowed, check.value) {
			errs = append(errs, fmt.Errorf("protocol.%s %q is not one of %v", check.field, check.value, check.allowed))
		}
	}
	for i, d := range p.Devices {
		if !slices.Contains(protocolDevices, d) {
			errs = append(errs, fmt.Errorf("protocol.devices[%d] %q is not one of %v", i, d, protocolDevices))
		}
	}
	if len(p.Devices) == 0 {
		errs = append(errs, errors.New("protocol.devices is required; a game nobody can control is a configuration error"))
	}

	// A shape that fixes its seat count may still say so, but not
	// differently: two answers to one question is worse than either.
	if want, fixed := seatsForShape(p.Shape); fixed && p.Seats.Count != want {
		errs = append(errs, fmt.Errorf("protocol.shape %q declares %d seat(s), not %d", p.Shape, want, p.Seats.Count))
	}
	switch {
	case p.Seats.Count < 1:
		errs = append(errs, fmt.Errorf("protocol.seats.count must be at least 1, not %d", p.Seats.Count))
	case p.Seats.Count > 8:
		errs = append(errs, fmt.Errorf("protocol.seats.count %d is past what the generated placeholder renders; declare the slots by hand instead", p.Seats.Count))
	}
	// A shape with no link has no mode to choose, so naming one would
	// promise consistency work that never runs.
	if p.Shape == "solo" && p.Sync != "server_authoritative" {
		errs = append(errs, fmt.Errorf("protocol.sync %q has no meaning for a solo game, which has no link", p.Sync))
	}
	// Past two seats every exchange is two hops through a host, which is
	// neither short nor symmetric, so the one-hop netcodes are unreachable.
	if p.Seats.Count > 2 && (p.Sync == "rollback" || p.Sync == "delay") {
		errs = append(errs, fmt.Errorf("protocol.sync %q needs a one-hop link, which %d seats cannot have", p.Sync, p.Seats.Count))
	}

	errs = append(errs, c.validateTeams()...)
	errs = append(errs, c.validateConditions()...)
	return errs
}

// validateTeams checks the seat division. Teams are optional; declaring
// some means they account for every seat, since a seat on no team has no
// answer to who it is playing with.
func (c *Config) validateTeams() []error {
	p := c.Protocol
	if len(p.Team) == 0 {
		return nil
	}
	var errs []error
	total := 0
	names := map[string]bool{}
	for i, t := range p.Team {
		switch {
		case t.Name == "":
			errs = append(errs, fmt.Errorf("protocol.team[%d].name is required", i))
		case names[t.Name]:
			errs = append(errs, fmt.Errorf("protocol.team[%d] repeats the name %q", i, t.Name))
		}
		names[t.Name] = true
		if t.Seats < 1 {
			errs = append(errs, fmt.Errorf("protocol.team[%d] (%s) needs at least one seat", i, t.Name))
		}
		total += t.Seats
		if t.Occupant != "" && !slices.Contains(seatOccupants, t.Occupant) {
			errs = append(errs, fmt.Errorf("protocol.team[%d] (%s) occupant %q is not one of %v", i, t.Name, t.Occupant, seatOccupants))
		}
	}
	if total != p.Seats.Count {
		errs = append(errs, fmt.Errorf("the teams account for %d seats and protocol.seats.count declares %d", total, p.Seats.Count))
	}
	return errs
}

// validateConditions checks the axes matchmaking filters on. An axis that
// cannot be satisfied by any room is worse than no axis: it refuses
// everybody and says the terms were not met.
func (c *Config) validateConditions() []error {
	var errs []error
	names := map[string]bool{}
	for i, a := range c.Protocol.Condition {
		switch {
		case a.Name == "":
			errs = append(errs, fmt.Errorf("protocol.condition[%d].name is required", i))
		case names[a.Name]:
			errs = append(errs, fmt.Errorf("protocol.condition[%d] repeats the axis %q", i, a.Name))
		}
		names[a.Name] = true
		if !slices.Contains(conditionMatches, a.Match) {
			errs = append(errs, fmt.Errorf("protocol.condition[%d] (%s) match %q is not one of %v", i, a.Name, a.Match, conditionMatches))
			continue
		}
		switch a.Match {
		case "exact":
			if a.Low != 0 || a.High != 0 {
				errs = append(errs, fmt.Errorf("protocol.condition[%d] (%s) is exact, so low and high mean nothing", i, a.Name))
			}
		case "band":
			if len(a.Values) > 0 {
				errs = append(errs, fmt.Errorf("protocol.condition[%d] (%s) is a band, so values mean nothing", i, a.Name))
			}
			if a.High <= a.Low {
				errs = append(errs, fmt.Errorf("protocol.condition[%d] (%s) spans %d..%d, which admits nothing", i, a.Name, a.Low, a.High))
			}
		}
	}
	return errs
}

// GamePackage is the import path identifying this game, half of the pair
// of decision:module-path-is-game-identity. It falls back to the module
// path, which is the whole path whenever one module holds one game.
func (c *Config) GamePackage() string {
	if c.Protocol.Package != "" {
		return c.Protocol.Package
	}
	return c.Project.Module
}

// GameTitle is what a browse list shows. It identifies nothing, so an
// empty one falls back to the last element of the package path rather
// than being an error.
func (c *Config) GameTitle() string {
	if c.Protocol.Title != "" {
		return c.Protocol.Title
	}
	pkg := c.GamePackage()
	if i := strings.LastIndex(pkg, "/"); i >= 0 {
		return pkg[i+1:]
	}
	return pkg
}

// SeatOccupant reports who may take one seat, counting from zero, after
// the team division narrows the table's own answer.
func (c *Config) SeatOccupant(seat int) string {
	base := c.Protocol.Seats.Occupant
	at := 0
	for _, t := range c.Protocol.Team {
		if seat < at+t.Seats {
			if t.Occupant != "" {
				return t.Occupant
			}
			return base
		}
		at += t.Seats
	}
	return base
}

// TeamOf reports the team a seat belongs to, counting from zero, and
// whether the game divides its seats at all.
func (c *Config) TeamOf(seat int) (string, bool) {
	at := 0
	for _, t := range c.Protocol.Team {
		if seat < at+t.Seats {
			return t.Name, true
		}
		at += t.Seats
	}
	return "", false
}
