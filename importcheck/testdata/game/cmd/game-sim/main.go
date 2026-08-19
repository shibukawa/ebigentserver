// The simulation entry point reaches the engine transitively through the
// rules package: the check must name the chain.
package main

import "example.com/game/rules"

func main() { rules.Step() }
