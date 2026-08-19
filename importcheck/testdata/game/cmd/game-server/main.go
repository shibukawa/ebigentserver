// The dedicated server entry point must stay engine-free.
package main

import "example.com/game/session"

func main() { session.Tick() }
