package buildconf

import (
	"strings"
	"testing"
	"time"
)

func valid() *Config {
	return &Config{
		Project:  &Project{Module: "example.com/game", Rules: "./game"},
		Build:    &Build{Target: []Target{{Name: "server", Kind: "dedicated", Entry: "./cmd/server"}}},
		Generate: &Generate{},
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
		{"no rules package", func(c *Config) { c.Project.Rules = "" }, "project.rules"},
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
