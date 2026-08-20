package session

// VisibilityAnnotation is data:visibility-annotation: the game's explicit
// declaration of what a slot could perceive at a decision point — emitted
// by the game, never inferred by the framework, because only the game
// knows why something was visible. Games embed it in their observation
// and view types, so it records and transmits with them.
//
// Without it an analyzer cannot tell a hidden field from an unused one,
// and flow:behavior-tree-synthesis would propose conditions the runtime
// agent can never evaluate (rule:analysis-restricted-to-visible-fields).
type VisibilityAnnotation struct {
	// Scope names the applied concept:visibility-scope: "self", "team",
	// "role", "spectator", or "global".
	Scope string `json:"scope"`
	// Schema names which observation shape this agent received —
	// asymmetric games ship views that differ in kind, not just radius.
	Schema string `json:"schema"`
	// VisibleEntities lists ids the agent could currently perceive.
	VisibleEntities []uint32 `json:"visible_entities,omitempty"`
	// Stale lists ids last seen earlier: remembered, no longer
	// confirmed.
	Stale []uint32 `json:"stale,omitempty"`
	// Derived names values the agent computed rather than received.
	Derived []string `json:"derived,omitempty"`
	// Affordances names the options the interface actually offered,
	// bounding what the player could have chosen.
	Affordances []string `json:"affordances,omitempty"`
	// EvaluationScope declares how the slot's data:evaluation-signal
	// was computed (rule:evaluation-respects-visibility-scope):
	// "scoped" (from this projection alone) or "privileged" (ground
	// truth; must then be withheld from agents during play).
	EvaluationScope string `json:"evaluation_scope,omitempty"`
}
