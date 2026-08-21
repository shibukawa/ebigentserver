// Command ebigent is the framework toolchain: one binary carrying every
// development task (api:game-cli).
//
// It replaces the separate behavior-editor, behavior-merge, and
// corpus-report commands, which became the edit, merge, and analyze
// verbs (decision:one-ebigent-binary).
//
//	ebigent init          scaffold a project
//	ebigent build         compile a declared target
//	ebigent config show   what is configured, and which layer set it
//	ebigent doctor        why the last thing failed
//	ebigent --help        every verb
package main

import "github.com/shibukawa/ebigentserver/cli"

func main() { cli.Main() }
