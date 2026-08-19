# ebigentserver

汎用ゲームサーバーフレームワーク。人間・ボット・リプレイを同一の `agent` 抽象で着席させるセッションランタイム（`vision:agent-session-runtime`）。

要件と設計判断は [.knowledge/](.knowledge/) に概念として記録されている。実装順は [plan.md](plan.md) を参照。

## 状態: Phase 0 — 数値と生成の土台

| 項目 | 状態 | 実装 |
|---|---|---|
| `api:fixed-point-math` | 済（ローカル） | [github.com/shibukawa/fixmath](https://github.com/shibukawa/fixmath) — F64 (s32.32)、BAM Angle、Vec2、宣言スケール変換。**未push・未tagのため `go.mod` は `replace => ../fixmath` を持つ。push と v1.0.0 tag 後に replace を外すこと** |
| CBOR 生成（wire / world プロファイル、スケール対応） | 済 | tinybind-go v0.5.17 + tinygodriver v1.2.6 |
| `rule:codegen-rejects-nondeterministic-types` | 済 | tinybind の生成時検査（[検証テスト](examples/phase0/msg/gencheck_test.go)） |
| `data:protocol-version` | 済 | 生成コードの `CBORProtocolVersion` / `CBORSchema` |
| `decision:entry-points-over-build-tags` の cmd/ 構成 | 済 | [examples/phase0/cmd/](examples/phase0/cmd/) |
| `rule:engine-import-confined-to-client-entry` | 済 | [importcheck/](importcheck/) |

### 完了条件の対応

- **固定小数点フィールドを持つ構造体が CBOR で往復できる** — [examples/phase0/msg/roundtrip_test.go](examples/phase0/msg/roundtrip_test.go)。wire プロファイルの生バイト列も固定している。
- **構造体に float を足すとビルドが落ちる** — 生成パスが float / 素の int / map / interface / time.Time を拒否する。[examples/phase0/msg/gencheck_test.go](examples/phase0/msg/gencheck_test.go) が検証。
- **ゲームルールのパッケージから Ebitengine を import するとビルドが落ちる** — [importcheck](importcheck/) が import グラフを検査し、違反パッケージと import 連鎖を named error で報告する。ルート [boundary_test.go](boundary_test.go) が本モジュール自身に適用。

## パッケージ

- `importcheck` — 依存の閉じ込め検査。ゲームモジュールは `importcheck.Enforce(t, ".", importcheck.Default())` を 1 テスト持つ。
- `examples/phase0` — Phase 0 の証明: 固定小数点型 + 生成 CBOR コーデック + delta、build target ごとの cmd エントリポイント。
- `examples/phase0/sim` — Phase 0 スタック全体（fixmath の Sin/Atan2/Sqrt → 宣言スケール量子化 → 生成 delta エンコード）を通した決定的エピソード。digest をテストで固定しており、これが Phase 2 のクロスアーキテクチャ検証の種になる。

## コード生成

```bash
go generate ./...
```

メッセージパッケージには `//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -openapi=false` を置く。生成物 `cborbind_gen.go` はコミットする。

## ビルドターゲット

`concept:build-target` ごとに 1 つの main パッケージ（build tag は使わない）:

```bash
go run ./examples/phase0/cmd/phase0-server   # dedicated server（headless）
go run ./examples/phase0/cmd/phase0-sim      # simulation（決定的、digest を出力）
go run ./examples/phase0/cmd/phase0-client   # client（描画は Phase 1 から）
```

全パッケージが `js/wasm`, `wasip1/wasm`, `linux/386` でビルドできることを維持する（`requirement:native-and-wasm-targets`）。
