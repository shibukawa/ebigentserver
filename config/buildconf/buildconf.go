// Package buildconf declares data:build-config: the toolchain sections of
// ebigent.toml, the file whose presence marks a project root.
//
// Only the ebigent tool binds these sections. A built artifact registers
// none of these prefixes and so never reads them, even though it reads
// the same file (decision:one-config-file-many-sections).
package buildconf

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -openapi=false

import (
	"time"

	"github.com/shibukawa/tinybind-go/configbind"
)

// Project is the [project] table: what this repository is.
type Project struct {
	// Module is the go module path, used to resolve entry points and
	// generated import paths.
	Module string `default:"" help:"go module path of this project"`
	// GoToolchain pins the toolchain so a generated project does not
	// drift with the host. Empty follows the host.
	GoToolchain string `key:"go" default:"" help:"pinned go toolchain version; empty follows the host"`
}

// Build is the [build] table. Each [[build.target]] is one
// concept:build-target, realized as one entry point per
// decision:entry-points-over-build-tags.
type Build struct {
	// Target is the declared artifact set. An array of tables, so the
	// file is its only source.
	Target []Target `help:"Target is the declared artifact set. An array of tables, so the file is its only source"`
}

// Target is one [[build.target]] block. Element of an array of tables, so
// it carries no enum or dependon tag and no CLI or env surface; Validate
// checks Kind instead.
type Target struct {
	// Name identifies the target to build and dev.
	Name string `default:"" help:"target name used by ebigent build and ebigent dev"`
	// Kind is the concept:build-target row: client, listen, dedicated,
	// or simulation. It decides which linkage the import graph check
	// enforces, notably whether ebitengine may appear at all.
	Kind string `default:"client" help:"client, listen, dedicated, or simulation"`
	// Entry is the main package of this target.
	Entry string `default:"" help:"main package path, such as ./cmd/game"`
	// GOOS and GOARCH pin the artifact platform. Empty follows the
	// host, and js/wasm is what makes a target the wasm row.
	GOOS   string `key:"goos" default:"" help:"target GOOS; empty follows the host"`
	GOARCH string `key:"goarch" default:"" help:"target GOARCH; empty follows the host"`
	// Tags are build tags. Linkage only: rule:build-tag-only-for-linkage
	// keeps behavior out of tags, so nothing here may select a game
	// rule or a synchronization mode.
	Tags []string `help:"Tags are build tags. Linkage only: rule:build-tag-only-for-linkage keeps behavior out of tags, so nothing here may select a game rule or a synchronization mode"`
	// Dev marks a target as carrying api:dev-debug-endpoint. A target
	// with Dev set must never be shipped
	// (rule:debug-endpoint-excluded-from-release).
	Dev bool `default:"false" help:"this target links the dev debug endpoint and must not ship"`
}

// Dev is the [dev] table: how ebigent dev runs flow:dev-rebuild-loop.
type Dev struct {
	// Target names the [[build.target]] dev builds and runs. Empty
	// picks the only declared target, and is an error when there are
	// several.
	Target string `default:"" help:"target ebigent dev builds and runs; empty requires exactly one declared target"`
	// Watch are the roots watched for changes.
	Watch []string `help:"source roots watched for changes; empty watches the project root"`
	// Ignore are glob patterns never watched, beyond the always
	// ignored VCS and build output directories.
	Ignore []string `help:"glob patterns never watched"`
	// Debounce is the quiet period before a change triggers a rebuild,
	// so a multi-file save rebuilds once.
	Debounce time.Duration `default:"200ms" help:"quiet period before a rebuild"`
	// Console is the ui:dev-console listen address. Loopback by
	// default; empty runs dev without a console.
	Console string `default:"127.0.0.1:8930" help:"dev console listen address; empty disables the console"`
}

// Behavior is the [behavior] table: where the AI pipeline keeps its
// files. decision:ai-pipeline-always-scaffolded means these paths exist
// in every generated project, whether or not the pipeline is in use.
type Behavior struct {
	// Library is the chip library that ui:behavior-tree-editor writes
	// to under rule:generated-behavior-requires-approval.
	Library string `default:"behavior/chips.json" help:"chip library file"`
	// Corpus is the data:episode-log root that analysis and the
	// evidence pane read.
	Corpus string `default:"corpus" help:"episode corpus root"`
	// Distill is the package `ebigent distill` runs.
	//
	// It is a path rather than a set of options because the step it
	// names cannot be described in configuration: mining needs the
	// game's predicates, and a predicate is a Go function over a sight,
	// not a value. So the toolchain spawns the project's own entry the
	// way `ebigent build` spawns `go build`, and what happens inside is
	// the game's to write.
	Distill string `default:"./cmd/distill" help:"package ebigent distill runs"`
}

// Config is every toolchain section, bound in one call.
type Config struct {
	Project  *Project
	Protocol *Protocol
	Build    *Build
	Dev      *Dev
	Behavior *Behavior
}

// Bind registers the toolchain sections and returns the destinations the
// next configbind.Load fills. Call it before Load, never after.
func Bind() *Config {
	return &Config{
		Project:  configbind.Bind[Project]("project"),
		Protocol: configbind.Bind[Protocol]("protocol"),
		Build:    configbind.Bind[Build]("build"),
		Dev:      configbind.Bind[Dev]("dev"),
		Behavior: configbind.Bind[Behavior]("behavior"),
	}
}

// Prefixes reports the configuration prefixes Bind claims, for the
// prefix-scoped stray key check of decision:one-config-file-many-sections.
func Prefixes() []string {
	return []string{"project", "protocol", "build", "dev", "behavior"}
}
