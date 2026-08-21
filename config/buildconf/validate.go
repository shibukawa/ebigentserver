package buildconf

import (
	"errors"
	"fmt"
	"slices"
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
