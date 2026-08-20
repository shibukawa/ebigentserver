# ebigentserver

汎用ゲームサーバーフレームワーク。人間・ボット・リプレイを同一の `agent` 抽象で着席させるセッションランタイム（`vision:agent-session-runtime`）。

要件と設計判断は [.knowledge/](.knowledge/) に概念として記録されている。実装順は [plan.md](plan.md) を参照。

## 状態: Phase 1 — セッション核 + Tic-Tac-Toe

| 項目 | 状態 | 実装 |
|---|---|---|
| `concept:session` / `concept:player-slot` | 済 | [session/](session/) — generics でゲームの S / A / O 型を受ける |
| `api:agent-interface`（step pacing） | 済 | [session/agent.go](session/agent.go) — Joined / Observe / Decide / Ended。`decide` の返却をセッションが待つ |
| `concept:session-lifecycle` 状態機械 | 済 | [session/lifecycle.go](session/lifecycle.go) — created → admitting → running → draining → ended / aborted、遷移表をテストで全列挙 |
| `rule:deterministic-tick-commit` | 済 | acting slots をステップ冒頭でスナップショット、slot ID 順にコミット |
| `decision:owner-namespaced-entity-ids` | 済 | [entity/](entity/) — 上位16bit=owner、下位48bit=連番 |
| `concept:observation`（global scope） | 済 | `Game.Project` — WorldState とは別型（Phase 5 への継ぎ目） |
| `data:evaluation-signal` | 済 | [session/evaluation.go](session/evaluation.go) — 全フィールドを最初から確保、Phase 1 は terminal のみ使用 |
| `api:action-validator`（legality フック） | 済 | [session/validator.go](session/validator.go) — 位置を確保、retry budget 超過で abort |
| `data:progress-report` 発行点 | 済 | [session/report.go](session/report.go) — terminal 発行のみ、Seq が冪等キー |
| `sample:tic-tac-toe` | 済 | [samples/tictactoe/](samples/tictactoe/) — ゲームルール + trivial bot + CLI |

### 完了条件の対応

- **人間とボットが同じ `api:agent-interface` で着席し、1局が終わる** — [ttt_test.go](samples/tictactoe/ttt/ttt_test.go) の `TestScriptedHumanBeatsBot`（scripted human）。実際の人間は `go run ./samples/tictactoe/cmd/ttt`（stdin の console agent が同一 interface を実装）。
- **ゲームロジック内に操作者種別の条件分岐が1つもない** — [ttt.go](samples/tictactoe/ttt/ttt.go) は agent 実装型を一切参照しない。controller の選択は cmd の起動フラグのみ（`decision:no-ai-game-mode` の review criterion）。

## 状態: Phase 0 — 数値と生成の土台

| 項目 | 状態 | 実装 |
|---|---|---|
| `api:fixed-point-math` | 済 | [github.com/shibukawa/fixmath](https://github.com/shibukawa/fixmath) v0.9.0 — F64 (s32.32)、BAM Angle、Vec2、宣言スケール変換（`ToScaled` はフィールド幅で飽和）。v1.0.0 tag で出力ビットが凍結される |
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

- `session` — セッション核。lifecycle 状態機械、slot 順の決定的コミット、agent interface、evaluation signal、action validator フック、progress report 発行点。
- `entity` — owner 名前空間つきエンティティ ID と決定的 allocator。
- `samples/tictactoe` — 最小サンプル兼回帰ハーネス（`decision:samples-as-test-infrastructure`）。`go run ./samples/tictactoe/cmd/ttt` で人間対ボット。
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
