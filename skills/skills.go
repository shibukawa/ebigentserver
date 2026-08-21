// Package skills embeds the framework's shareable agent skills so the
// toolchain can copy them into a generated project.
//
// The behavior-analyze skill is the LLM half of flow:behavior-tree-synthesis
// and runs in the developer's own agentic environment, not in a game
// process (rule:analysis-tooling-outside-game-process). It is written
// into every project because decision:ai-pipeline-always-scaffolded never
// asks whether the pipeline is wanted.
package skills

import "embed"

// BehaviorAnalyze holds the behavior-analyze skill directory, rooted at
// "behavior-analyze".
//
//go:embed behavior-analyze
var BehaviorAnalyze embed.FS
