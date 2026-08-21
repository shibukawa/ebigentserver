// Package importcheck enforces rule:engine-import-confined-to-client-entry:
// only client entry points may depend on the rendering engine (or any other
// confined dependency). Game rules, session, and server packages must stay
// free of it, so a headless target cannot acquire a rendering or cgo
// dependency through an unrelated package.
//
// The check inspects the module's import graph via `go list -deps -json` and
// fails with the offending package and the exact import chain, mirroring the
// build-error style of rule:codegen-rejects-nondeterministic-types.
package importcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path"
	"sort"
	"strings"
)

// Rule confines one dependency to a set of entry-point packages.
type Rule struct {
	// Prefix is the import path prefix being confined, for example
	// "github.com/hajimehoshi/ebiten". A package matches when its import
	// path equals the prefix or starts with prefix + "/".
	Prefix string
	// Reason is included in the violation message.
	Reason string
	// AllowedEntries are path.Match patterns, matched against the package
	// path relative to the module root ("cmd/pong-client", "internal/render").
	// A package matching any pattern may depend on Prefix.
	AllowedEntries []string
}

// Config selects the rules to enforce over one module.
type Config struct {
	Rules []Rule
	// ForbidCgo reports module-local packages built with cgo that do not
	// match AllowedCgoEntries. Cgo breaks the wasm targets of
	// requirement:native-and-wasm-targets, so the same pass covers it.
	ForbidCgo         bool
	AllowedCgoEntries []string
	// BuildTags selects which build the import graph is read from.
	//
	// It matters wherever a build tag decides linkage
	// (rule:build-tag-only-for-linkage): the untagged graph says nothing
	// about the tagged one, and it is usually the tagged build — the
	// headless artifact that ships — whose freedom from a renderer is
	// worth proving. Check both, one Config each.
	BuildTags []string
}

// EbitengineRule confines Ebitengine to client entry points. The default
// entry patterns accept cmd directories whose name marks a rendering target
// (client, listen server, static bundle) and packages under a directory
// named "presentation".
func EbitengineRule() Rule {
	return Rule{
		Prefix: "github.com/hajimehoshi/ebiten",
		Reason: "rule:engine-import-confined-to-client-entry — game rules and session packages must not import the engine",
		AllowedEntries: []string{
			"cmd/*client*",
			"cmd/*listen*",
			"cmd/*static*",
			"presentation", "presentation/*", "*/presentation", "*/presentation/*",
		},
	}
}

// Default is the configuration the framework applies to a game module.
func Default() Config {
	r := EbitengineRule()
	return Config{
		Rules:             []Rule{r},
		ForbidCgo:         true,
		AllowedCgoEntries: r.AllowedEntries,
	}
}

// Violation is one package that reaches a confined dependency.
type Violation struct {
	// Pkg is the module-local package that violates the rule.
	Pkg string
	// Target is the confined package actually reached.
	Target string
	// Chain is the import path from Pkg to Target, inclusive.
	Chain []string
	// Reason restates the violated rule.
	Reason string
}

func (v Violation) String() string {
	return fmt.Sprintf("package %s must not depend on %s (%s)\n\timport chain: %s",
		v.Pkg, v.Target, v.Reason, strings.Join(v.Chain, " -> "))
}

type listedPackage struct {
	ImportPath string
	Standard   bool
	Imports    []string
	Deps       []string
	CgoFiles   []string
	Module     *struct {
		Path string
		Main bool
	}
	ForTest string
}

// Check runs the import graph inspection over the module rooted at dir.
// It returns the violations found; a non-nil error means the inspection
// itself could not run.
func Check(ctx context.Context, dir string, cfg Config) ([]Violation, error) {
	pkgs, err := listPackages(ctx, dir, cfg.BuildTags)
	if err != nil {
		return nil, err
	}

	byPath := make(map[string]*listedPackage, len(pkgs))
	for _, p := range pkgs {
		if p.ForTest != "" {
			continue // test variants duplicate the graph; the base package covers the rule
		}
		byPath[p.ImportPath] = p
	}

	var violations []Violation
	for _, p := range byPath {
		if p.Module == nil || !p.Module.Main {
			continue // only module-local packages are held to the rule
		}
		rel := relPath(p.ImportPath, p.Module.Path)

		for _, rule := range cfg.Rules {
			if matchAny(rel, rule.AllowedEntries) {
				continue
			}
			target := firstMatch(p, rule.Prefix)
			if target == "" {
				continue
			}
			violations = append(violations, Violation{
				Pkg:    p.ImportPath,
				Target: target,
				Chain:  importChain(byPath, p.ImportPath, target),
				Reason: rule.Reason,
			})
		}

		if cfg.ForbidCgo && len(p.CgoFiles) > 0 && !matchAny(rel, cfg.AllowedCgoEntries) {
			violations = append(violations, Violation{
				Pkg:    p.ImportPath,
				Target: "C",
				Chain:  []string{p.ImportPath, "C"},
				Reason: "cgo breaks the wasm targets of requirement:native-and-wasm-targets",
			})
		}
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Pkg != violations[j].Pkg {
			return violations[i].Pkg < violations[j].Pkg
		}
		return violations[i].Target < violations[j].Target
	})
	return violations, nil
}

func listPackages(ctx context.Context, dir string, tags []string) ([]*listedPackage, error) {
	args := []string{"list", "-deps", "-json=ImportPath,Standard,Imports,Deps,CgoFiles,Module,ForTest"}
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
	}
	args = append(args, "./...")
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(out)
	var pkgs []*listedPackage
	for {
		p := new(listedPackage)
		if err := dec.Decode(p); err == io.EOF {
			break
		} else if err != nil {
			_ = cmd.Wait()
			return nil, fmt.Errorf("importcheck: parsing go list output: %w", err)
		}
		pkgs = append(pkgs, p)
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("importcheck: go list failed: %w\n%s", err, stderr.String())
	}
	return pkgs, nil
}

func relPath(importPath, modulePath string) string {
	if importPath == modulePath {
		return "."
	}
	return strings.TrimPrefix(importPath, modulePath+"/")
}

func matchAny(rel string, patterns []string) bool {
	for _, pat := range patterns {
		if ok, _ := path.Match(pat, rel); ok {
			return true
		}
	}
	return false
}

func hasPrefix(importPath, prefix string) bool {
	return importPath == prefix || strings.HasPrefix(importPath, prefix+"/")
}

// firstMatch returns a dependency of p (direct or transitive) under prefix.
func firstMatch(p *listedPackage, prefix string) string {
	if hasPrefix(p.ImportPath, prefix) {
		return p.ImportPath
	}
	for _, d := range p.Deps {
		if hasPrefix(d, prefix) {
			return d
		}
	}
	return ""
}

// importChain finds a shortest path from one package to another over the
// direct-import edges, so the violation names who introduced the dependency.
func importChain(byPath map[string]*listedPackage, from, to string) []string {
	type node struct {
		path string
		prev *node
	}
	visited := map[string]bool{from: true}
	queue := []*node{{path: from}}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if n.path == to {
			var chain []string
			for ; n != nil; n = n.prev {
				chain = append([]string{n.path}, chain...)
			}
			return chain
		}
		p := byPath[n.path]
		if p == nil {
			continue
		}
		for _, imp := range p.Imports {
			if !visited[imp] {
				visited[imp] = true
				queue = append(queue, &node{path: imp, prev: n})
			}
		}
	}
	return []string{from, to}
}
