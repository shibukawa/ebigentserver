// Package cli is the ebigent command: one binary carrying every
// development task (requirement:unified-toolchain-binary).
//
// Each verb is a configbind SubCommand, so its options, positional
// arguments, and usage text are generated from a struct rather than
// assembled by hand — which is what keeps seventeen verbs from drifting
// into seventeen dialects of flag parsing (decision:one-ebigent-binary).
//
// Verb fields never read the configuration file or the environment.
// Settings come from ebigent.toml through data:build-config and
// data:run-config; a verb option is a choice about this invocation.
//
// The two option sets sit on opposite sides of the verb, because a
// subcommand consumes every argument after its own name:
//
//	ebigent --run-topology dedicated build server   # a configuration key
//	ebigent build server --output ./bin/server      # a verb option
//
// Putting a configuration option after the verb is a usage error rather
// than a silent no-op, which is the one place this arrangement can
// surprise someone.
package cli

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -openapi=false

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/shibukawa/ebigentserver/config/buildconf"
	"github.com/shibukawa/ebigentserver/config/confload"
	"github.com/shibukawa/ebigentserver/config/runconf"
	"github.com/shibukawa/tinybind-go/configbind"
)

// Version is the toolchain version, set at build time.
var Version = "dev"

// InitOptions scaffolds a new project (flow:project-init).
type InitOptions struct {
	Dir    string `arg:"optional" help:"project directory, created if missing (default: the current directory)"`
	Module string `default:"" help:"go module path"`
	Name   string `default:"" help:"game name; defaults to the directory name"`
	Style  string `default:"" help:"play style: solo, duo, or multi"`
	Agent  string `default:"" help:"agentic environment: claude or other; decides where the analysis skill is written"`
	Seats  int    `default:"0" help:"maximum players, for the multi style; 0 asks"`
	// A string rather than a bool: a wizard has to tell "said no" apart
	// from "did not say", and a bool cannot.
	SharedScreen  string `default:"" help:"do all players read the same screen content: yes or no; empty asks"`
	Sync          string `default:"" help:"synchronization mode; a shared surface has none"`
	Yes           bool   `default:"false" help:"take the default for every unanswered question instead of prompting"`
	FrameworkPath string `default:"" help:"path to a local framework checkout, added as a replace directive"`
	SkipTidy      bool   `default:"false" help:"do not run go mod tidy or the verification build"`
}

// BuildOptions compiles one declared concept:build-target.
type BuildOptions struct {
	Target string `arg:"optional" help:"target name from ebigent.toml; defaults to the dev target"`
	Output string `default:"" help:"output path; defaults to ./bin/<target>"`
}

// GenerateOptions emits the code the configuration settles: the protocol
// constants of requirement:config-codegen. It takes nothing — what to
// generate is the whole of ebigent.toml, and a partial run would only
// leave a tree half describing a file that has one answer.
type GenerateOptions struct{}

// ConfigOptions renders or explains the configuration.
type ConfigOptions struct {
	Action string `arg:"optional" default:"show" help:"scaffold, env, or show"`
}

// AnalyzeOptions aggregates a recorded corpus (metric:balance-signals).
type AnalyzeOptions struct {
	Corpus string `default:"" help:"corpus root; defaults to the behavior.corpus setting"`
	SQL    string `default:"" help:"also write a DuckDB report script to this path"`
}

// MergeOptions folds analyzer proposals into a chip library.
type MergeOptions struct {
	Request   string `arg:"required" help:"analysis-request.json the proposals answer"`
	Proposals string `arg:"required" help:"proposals JSON from the analyzer"`
	Library   string `default:"" help:"chip library; defaults to the behavior.library setting"`
	Diff      string `default:"" help:"write the reviewable diff JSON to this path"`
}

// DoctorOptions reports environment problems.
type DoctorOptions struct{}

// VersionOptions prints the toolchain version.
type VersionOptions struct{}

// pendingVerbs are declared so the help tree shows the shape of the
// toolchain, and so each one can say what it is waiting on rather than
// failing as an unknown command.
type (
	// DevOptions is flow:dev-rebuild-loop and ui:dev-console.
	DevOptions struct{}
	// RunOptions starts a built target with a data:run-config.
	RunOptions struct{}
	// SimulateOptions runs concept:training-farm workloads.
	SimulateOptions struct{}
	// ReplayOptions plays an episode back through actor:replay-agent.
	ReplayOptions struct{}
	// EditOptions opens the authoring tabs of ui:dev-console.
	EditOptions struct{}
)

// Run parses the command line, loads the configuration once, and
// dispatches. It returns the process exit code.
func Run(stdout, stderr io.Writer) int {
	initOpts := configbind.SubCommand[InitOptions]("init", "scaffold a new project")
	buildOpts := configbind.SubCommand[BuildOptions]("build", "compile a declared build target")
	configOpts := configbind.SubCommand[ConfigOptions]("config", "render or explain the configuration")
	generateOpts := configbind.SubCommand[GenerateOptions]("generate", "emit the code the configuration settles")
	analyzeOpts := configbind.SubCommand[AnalyzeOptions]("analyze", "aggregate a recorded episode corpus")
	mergeOpts := configbind.SubCommand[MergeOptions]("merge", "fold analyzer proposals into a chip library")
	doctorOpts := configbind.SubCommand[DoctorOptions]("doctor", "report environment problems")
	versionOpts := configbind.SubCommand[VersionOptions]("version", "print the toolchain version")

	devOpts := configbind.SubCommand[DevOptions]("dev", "build, run, and restart on change, with the dev console")
	runOpts := configbind.SubCommand[RunOptions]("run", "start a built target")
	simulateOpts := configbind.SubCommand[SimulateOptions]("simulate", "run headless simulation workloads")
	replayOpts := configbind.SubCommand[ReplayOptions]("replay", "play back a recorded episode")
	editOpts := configbind.SubCommand[EditOptions]("edit", "open the behavior authoring tabs")

	// The tool owns every prefix in the file: it binds the toolchain
	// sections and the run sections it passes down to a child process.
	// A project is optional here because init runs outside one.
	build := buildconf.Bind()
	run := runconf.Bind()
	res, err := confload.Load(confload.Options{
		Owned:               append(buildconf.Prefixes(), runconf.Prefixes()...),
		AllowMissingProject: true,
	})
	if err != nil {
		var usage *configbind.UsageError
		if errors.As(err, &usage) {
			// An empty Message is a help request; anything else is a
			// parse failure, and a failure must not exit 0 just
			// because usage is the most useful thing to print.
			if usage.Message == "" {
				fmt.Fprintln(stdout, usage.Usage)
				return 0
			}
			fmt.Fprintln(stderr, "ebigent:", usage.Message)
			fmt.Fprintln(stderr, usage.Usage)
			return 2
		}
		fmt.Fprintln(stderr, "ebigent:", err)
		return 1
	}

	ctx := &context{
		stdout: stdout,
		stderr: stderr,
		build:  build,
		run:    run,
		res:    res,
	}

	switch {
	case initOpts != nil:
		return ctx.report(runInit(ctx, initOpts))
	case buildOpts != nil:
		return ctx.report(runBuild(ctx, buildOpts))
	case generateOpts != nil:
		return ctx.report(runGenerate(ctx))
	case configOpts != nil:
		return ctx.report(runConfig(ctx, configOpts))
	case analyzeOpts != nil:
		return ctx.report(runAnalyze(ctx, analyzeOpts))
	case mergeOpts != nil:
		return ctx.report(runMerge(ctx, mergeOpts))
	case doctorOpts != nil:
		return ctx.report(runDoctor(ctx))
	case versionOpts != nil:
		fmt.Fprintf(stdout, "ebigent %s\n", Version)
		return 0
	case devOpts != nil:
		return ctx.report(pending("dev", "the watch loop and ui:dev-console"))
	case runOpts != nil:
		return ctx.report(pending("run", "launching a built artifact"))
	case simulateOpts != nil:
		return ctx.report(pending("simulate", "concept:training-farm workloads"))
	case replayOpts != nil:
		return ctx.report(pending("replay", "actor:replay-agent playback"))
	case editOpts != nil:
		return ctx.report(pending("edit", "the authoring tabs of ui:dev-console"))
	}

	fmt.Fprintln(stderr, "ebigent: no command given; try ebigent --help")
	return 2
}

// context carries what every verb needs: where to write, the loaded
// configuration, and where the project is.
type context struct {
	stdout, stderr io.Writer
	build          *buildconf.Config
	run            *runconf.Run
	res            *confload.Result
}

func (c *context) report(err error) int {
	if err == nil {
		return 0
	}
	fmt.Fprintln(c.stderr, "ebigent:", err)
	return 1
}

// requireProject rejects a verb that needs a project when none was found.
// The message names the file rather than the condition, because the fix
// is to be somewhere else or to run init.
func (c *context) requireProject() error {
	if c.res.ProjectRoot == "" {
		return fmt.Errorf("no %s in this directory or any parent; run `ebigent init` to make one", confload.FileName)
	}
	if err := c.build.Validate(); err != nil {
		return err
	}
	return nil
}

// path resolves a project-relative path against the project root, so a
// verb works the same from any subdirectory.
func (c *context) path(p string) string {
	if p == "" || filepath.IsAbs(p) || c.res.ProjectRoot == "" {
		return p
	}
	return filepath.Join(c.res.ProjectRoot, p)
}

func pending(verb, what string) error {
	return fmt.Errorf("%s is declared but not implemented yet: it lands with %s", verb, what)
}

// Main is the entry point wrapper, kept here so cmd/ebigent stays a
// three-line main package.
func Main() { os.Exit(Run(os.Stdout, os.Stderr)) }
