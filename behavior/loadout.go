package behavior

import (
	"fmt"
	"go/format"
	"strings"
)

// Loadout is data:agent-loadout: one AI personality assembled from the
// shared chip library — which chips it knows, grouped into tactics, plus
// the behavior-profile metadata the execution layer reads. Many loadouts
// select from one library, which is the anti-drift point of
// decision:shared-chip-library.
type Loadout struct {
	Name string `json:"name"`
	// Tactics are evaluated in order by the generated root selector
	// (concept:tactic-selector); the first whose condition holds is
	// active this decision. A tactic with an empty Condition is the
	// fallback and must come last.
	Tactics []Tactic `json:"tactics"`
	// Profile carries the concept:behavior-profile vector as metadata;
	// codegen does not consume it (execution quality is the runtime
	// layer's axis, knowledge is this one's).
	Profile map[string]string `json:"profile,omitempty"`
}

// Tactic is one chip group of the loadout.
type Tactic struct {
	Name string `json:"name"`
	// Condition names the vocabulary Feature that activates the
	// tactic; empty means always (the fallback group). Because the
	// condition is an observation predicate, switching is
	// deterministic and replays exactly — and a player's order to an
	// allied agent can drive it by simply being part of the
	// observation.
	Condition string `json:"condition,omitempty"`
	// ChipKeys select library chips (Chip.Key() strings) in decision
	// order within this tactic.
	ChipKeys []string `json:"chip_keys"`
}

// Validate checks the loadout against a library: every key must resolve
// to an approved chip, and only the last tactic may be unconditional.
func (l *Loadout) Validate(lib *Library) error {
	if len(l.Tactics) == 0 {
		return fmt.Errorf("behavior: loadout %q has no tactics", l.Name)
	}
	for ti, t := range l.Tactics {
		if t.Condition == "" && ti != len(l.Tactics)-1 {
			return fmt.Errorf("behavior: loadout %q: unconditional tactic %q must be last", l.Name, t.Name)
		}
		if len(t.ChipKeys) == 0 {
			return fmt.Errorf("behavior: loadout %q: tactic %q selects no chips", l.Name, t.Name)
		}
		for _, key := range t.ChipKeys {
			chip := lib.find(key)
			if chip == nil {
				return fmt.Errorf("behavior: loadout %q: unknown chip %q", l.Name, key)
			}
			if !chip.Approved || chip.Rejected {
				return fmt.Errorf("behavior: loadout %q: chip %q is not approved (rule:generated-behavior-requires-approval)", l.Name, key)
			}
		}
	}
	return nil
}

// GenerateLoadoutAgent compiles a loadout to Go: the root switch is the
// tactic selector, each case the tactic's chip decision list — static
// code, nothing rebuilt at runtime (concept:tactic-selector under
// decision:behavior-tree-compiled-to-go). funcName and agentName let
// several loadouts coexist in one generated package.
func GenerateLoadoutAgent(spec CodegenSpec, v *Vocabulary, lib *Library, l *Loadout, funcName, agentName string) ([]byte, error) {
	if err := l.Validate(lib); err != nil {
		return nil, err
	}
	features := map[string]Feature{}
	for _, f := range v.Features {
		features[f.Name] = f
	}
	actions := map[string]ActionDef{}
	for _, a := range v.Actions {
		actions[a.Name] = a
	}

	var b strings.Builder
	fmt.Fprintf(&b, "// Package %s: generated loadout %q — a tactic selector over chips\n", spec.Package, l.Name)
	b.WriteString("// from the shared library (data:agent-loadout, concept:tactic-selector).\n")
	b.WriteString("// Edits belong upstream in the loadout or the chips.\n")
	fmt.Fprintf(&b, "package %s\n\nimport (\n\t\"context\"\n\n\t%q\n", spec.Package, spec.SessionImport)
	for _, imp := range spec.Imports {
		fmt.Fprintf(&b, "\t%q\n", imp)
	}
	b.WriteString(")\n\n")

	fmt.Fprintf(&b, "// %s selects a tactic from the observation, then runs its chips.\n", funcName)
	fmt.Fprintf(&b, "func %s(obs %s) (%s, bool) {\n\tswitch {\n", funcName, spec.ObsType, spec.ActionType)
	for _, t := range l.Tactics {
		cond := "true"
		note := "fallback tactic"
		if t.Condition != "" {
			f, ok := features[t.Condition]
			if !ok {
				return nil, fmt.Errorf("behavior: tactic %q names unknown predicate %q", t.Name, t.Condition)
			}
			cond = f.GoExpr
			note = "tactic " + t.Name
		} else {
			note = "fallback tactic " + t.Name
		}
		fmt.Fprintf(&b, "\tcase %s:\n\t\t// %s\n\t\tswitch {\n", cond, note)
		for _, key := range t.ChipKeys {
			chip := lib.find(key)
			f, ok := features[chip.Condition]
			if !ok {
				return nil, fmt.Errorf("behavior: chip %q names unknown predicate %q", key, chip.Condition)
			}
			a, ok := actions[chip.Action]
			if !ok {
				return nil, fmt.Errorf("behavior: chip %q names unknown action %q", key, chip.Action)
			}
			fmt.Fprintf(&b, "\t\tcase %s:\n\t\t\t// chip %s (coverage %d)\n\t\t\treturn %s, true\n",
				f.GoExpr, chip.Key(), chip.Coverage, a.GoExpr)
		}
		b.WriteString("\t\t}\n")
	}
	fmt.Fprintf(&b, "\t}\n\tvar zero %s\n\treturn zero, false\n}\n\n", spec.ActionType)

	fmt.Fprintf(&b, "// %s seats the loadout behind api:agent-interface.\n", agentName)
	fmt.Fprintf(&b, "type %s struct {\n\tlast %s\n\thas  bool\n}\n\n", agentName, spec.ObsType)
	fmt.Fprintf(&b, "func (*%s) Guest(session.SlotID) {}\n\n", agentName)
	fmt.Fprintf(&b, "func (a *%s) Observe(obs %s) { a.last, a.has = obs, true }\n\n", agentName, spec.ObsType)
	fmt.Fprintf(&b, "func (a *%s) Decide(context.Context) (%s, bool) {\n\tif !a.has {\n\t\tvar zero %s\n\t\treturn zero, false\n\t}\n\treturn %s(a.last)\n}\n\n",
		agentName, spec.ActionType, spec.ActionType, funcName)
	fmt.Fprintf(&b, "func (*%s) Ended(session.Result) {}\n", agentName)

	return format.Source([]byte(b.String()))
}
