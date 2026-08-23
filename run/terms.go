package run

import (
	"fmt"
	"sort"
	"strings"
)

// Axis is one thing a room may state about itself and a joiner may filter
// on (requirement:conditional-matchmaking).
//
// The axis set is the game's and settled at build; what a room says on
// each axis is chosen when the room opens. Two ends that disagree about
// which axes exist could not explain a refusal to each other, which is
// why only the values are a run value.
type Axis struct {
	// Name is the axis.
	Name string `json:"name"`
	// Band marks the asymmetric comparison: the room states a range and
	// the joiner brings their own value. A rank is an attribute of the
	// player rather than a filter they pick, so there is no unset case
	// on their side.
	Band bool `json:"band,omitempty"`
}

// Terms are what one room states about itself: a value per exact axis and
// a range per band axis. An axis a room says nothing about constrains
// nothing.
type Terms struct {
	// Exact maps an axis to the value a joiner must want, or want
	// nothing on.
	Exact map[string]string `json:"exact,omitempty"`
	// Low and High bound a band axis. Both are needed for the bound to
	// mean anything.
	Low  map[string]int `json:"low,omitempty"`
	High map[string]int `json:"high,omitempty"`
}

// Wants is what a joiner brings: the values they ask for on exact axes and
// the attributes they carry on band ones.
type Wants struct {
	// Exact is what this joiner is looking for. An axis absent here is
	// one they do not care about.
	Exact map[string]string `json:"exact,omitempty"`
	// Value is this joiner's own reading on each band axis — their rank,
	// not their preference.
	Value map[string]int `json:"value,omitempty"`
}

// Satisfies reports whether a joiner may sit in a room, and says why not
// when they may not.
//
// It is the conjunction of every axis and nothing else: no disjunction,
// no nesting, no expression language. A room states its terms once when
// it opens and judges nobody afterwards, so this is the whole of what
// happens on the host side when somebody arrives — a check against what
// was already declared.
func (t Terms) Satisfies(axes []Axis, w Wants) error {
	for _, a := range sortedAxes(axes) {
		if err := t.satisfiesAxis(a, w); err != nil {
			return err
		}
	}
	return nil
}

// satisfiesAxis is one axis of the conjunction.
func (t Terms) satisfiesAxis(a Axis, w Wants) error {
	if a.Band {
		low, hasLow := t.Low[a.Name]
		high, hasHigh := t.High[a.Name]
		if !hasLow || !hasHigh {
			return nil // the room bounded nothing on this axis
		}
		got, ok := w.Value[a.Name]
		if !ok {
			return fmt.Errorf("%s: the room admits %d..%d and this peer brought no reading", a.Name, low, high)
		}
		if got < low || got > high {
			return fmt.Errorf("%s: %d is outside the %d..%d this room admits", a.Name, got, low, high)
		}
		return nil
	}
	want, asked := w.Exact[a.Name]
	has, stated := t.Exact[a.Name]
	if !asked || !stated {
		return nil // either side saying nothing is no constraint
	}
	if want != has {
		return fmt.Errorf("%s: this room is %q and the peer asked for %q", a.Name, has, want)
	}
	return nil
}

// Describe renders the terms for a browse list, in axis order so two
// rooms read alike.
func (t Terms) Describe(axes []Axis) string {
	var parts []string
	for _, a := range sortedAxes(axes) {
		if a.Band {
			low, hasLow := t.Low[a.Name]
			high, hasHigh := t.High[a.Name]
			if hasLow && hasHigh {
				parts = append(parts, fmt.Sprintf("%s %d..%d", a.Name, low, high))
			}
			continue
		}
		if v, ok := t.Exact[a.Name]; ok {
			parts = append(parts, a.Name+" "+v)
		}
	}
	return strings.Join(parts, ", ")
}

// sortedAxes puts the axes in name order, so a refusal names the same one
// whichever end reports it.
func sortedAxes(axes []Axis) []Axis {
	out := append([]Axis(nil), axes...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
