module github.com/shibukawa/ebigentserver/tutorial/step3-record

go 1.26.5

require (
	github.com/hajimehoshi/ebiten/v2 v2.9.10
	github.com/shibukawa/ebigentserver v0.0.0
	github.com/shibukawa/tinybind-go v0.5.23
	github.com/shibukawa/tinygodriver v1.2.7
)

require (
	github.com/ebitengine/gomobile v0.0.0-20250923094054-ea854a63cce1 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/purego v0.9.0 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	github.com/shibukawa/fixmath v0.9.0 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/tools v0.48.0 // indirect
)

replace github.com/shibukawa/ebigentserver => ../..

tool github.com/shibukawa/tinybind-go/cmd/tinybind-gen
