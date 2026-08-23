# 実装計画

汎用ゲームサーバーフレームワークの実装作業順。

要件と設計判断は `.knowledge/` に 201 概念として記録済み。本書はそれらを **どの順で実装するか** だけを扱う。個々の判断の根拠は概念側を参照すること。

```bash
python3 ~/.claude/skills/knowledge-compiler/scripts/concept.py show --project . concept:agent --related
```

## 全体方針

`concept:sample-progression` の7サンプルは、そのまま能力の依存順になっている。したがって **サンプルの梯子を実装順として使う**。

各サンプルは `rule:sample-adds-one-capability` により1つの能力だけを追加し、`decision:samples-as-test-infrastructure` により自身の回帰テストを連れてくる。前段が緑である限り、後段の失敗は新しい能力に局在する。

この梯子の前に基盤フェーズ（Phase 0）を、後にAI育成と代替経路（Phase 7〜8）を置く。

---

## Phase 0 — 数値と生成の土台

サンプルなし。**ここだけは後から入れると全書き直しになる。**

### 実装項目

- `api:fixed-point-math` — FixPointCS 移植。sqrt / rcp / exp / log / sin / cos / atan2。角度は BAM（全周を整数域に対応させる）
- `system:tinybind` の CBOR 生成 — `concept:cbor-wire-profile` と `concept:cbor-world-profile`、スケール対応の encode/decode
- `rule:codegen-rejects-nondeterministic-types` の検査パス — float / map / interface / time.Time / スケール未宣言フィールドをビルドエラーにする
- `data:protocol-version` — 生成スキーマからの導出
- `decision:entry-points-over-build-tags` の `cmd/` 構成
- `rule:engine-import-confined-to-client-entry` の import グラフ検査

### 完了条件

- 固定小数点フィールドを持つ構造体が CBOR で往復できる
- 構造体に float を足すとビルドが落ちる
- ゲームルールのパッケージから Ebitengine を import するとビルドが落ちる

### 注意

FixPointCS に Go 版は存在しない。移植は自前で、ライセンスは vendoring 前に確認すること（`system:fixpointcs`）。

---

## Phase 1 — セッション核 + Tic-Tac-Toe

### 実装項目

- `concept:session` と `concept:player-slot`
- `api:agent-interface` — step pacing（セッションが `decide` の返却を待つ）
- `concept:session-lifecycle` の状態機械（created / admitting / running / draining / ended / aborted）
- ローカル（プロセス内）トランスポート
- `rule:deterministic-tick-commit` — スロットID順の安定した順序付けコミット
- `decision:owner-namespaced-entity-ids`
- `concept:sight`（この段階では global scope）
- `data:evaluation-signal`（勝敗のみ）
- `sample:tic-tac-toe`

### 完了条件

- 人間とボットが同じ `api:agent-interface` で着席し、1局が終わる
- `decision:no-ai-game-mode` のレビュー基準を満たす。ゲームロジック内に操作者種別の条件分岐が1つもないこと

---

## Phase 2 — 記録と決定性の検証 + Reversi

### 実装項目

- 合法手列挙を `concept:sight` に載せる。ボットが独自のルールエンジンを持たないための土台
- 探索AIと AI vs AI 対戦
- `data:episode-log` — JSONL 4ストリーム（decisions / events / outcomes / world）
- `concept:episode-recording-mode` — replay_complete と analysis_sampled の区別
- `data:decision-record`
- `actor:replay-agent`
- `rule:shared-rng-seed`
- `data:state-checkpoint`

### 完了条件

- 記録した対局の再生がビット一致する
- **arm64 と amd64 で同じエピソードを再生し、ダイジェストが一致する**

Phase 0 の決定性がここで初めて実証される。ここが赤いまま先へ進まないこと。

---

## Phase 3 — リアルタイムとネットワーク + Pong

最大のフェーズ。2つに割って進める。

### 3a. ローカルリアルタイム

- tick ループと権威シミュレーション
- `data:player-input`、`data:snapshot`、`data:state-delta`
- `decision:framework-side-delta-generation` — 保持スナップショットとの差分生成
- `data:session-tuning-profile` — tick rate、送信レート、帯域予算などの宣言
- `sample:pong` をループバックで動かす

### 3b. ネットワーク

- `api:transport-interface` と WebTransport / WebSocket 実装
- `api:message-framing` — 分割・再構成・backpressure
- `api:sequence-ack-layer` — シーケンス、ack、沈黙検出
- `concept:delta-baseline-policy` と `concept:ack-transmission-policy`
- `flow:session-admission` — JWT チケット、`rule:asymmetric-ticket-signature`、`rule:ticket-bound-to-connection`
- `policy:protocol-rollout`

### 完了条件

- 損失と遅延を注入した状態で Pong が破綻しない
- バージョン不一致の接続が handshake で明示的に拒否される

---

## Phase 4 — 多人数と運用堅牢性 + Tron

### 実装項目

- 多人数フィールド（人間とボットの混在）
- `permission:spectator-receive-only` — 観戦者。専用 ack 経路の実証も兼ねる
- `concept:agent-departure-policy` と `concept:agent-proxy-designation`
- `decision:host-loss-ends-session`
- `api:action-validator` — legality（決定的、全ピア）と plausibility（権威側のみ、ヒューリスティック可）の2クラス
- `data:runtime-resource-budget`
- `policy:overload-handling`、`policy:realtime-abuse-protection`
- `api:runtime-observability`

### 完了条件

- 8人 + 観戦者で、切断・過負荷・不正入力を注入しても `requirement:production-runtime-safety` を満たす
- 8受信者での baseline 保持コストが実測でき、`data:session-tuning-profile` の history_depth の妥当性が確認できる

---

## Phase 5 — 投影と非対称性 + Cooperative Maze → Dungeon Master

### 実装項目

- `concept:agent-view` — スロットごとの保持と増分更新
- `concept:visibility-scope` — self / team / role / spectator / global
- `data:visibility-annotation` — アプリ側からの明示出力
- `rule:sight-content-owned-by-game`
- 役割とチーム（Scout / Engineer / Carrier / Navigator）
- `rule:evaluation-respects-visibility-scope`
- `sample:cooperative-maze` → `sample:dungeon-master`

### 完了条件

- DM とパーティが種類の違う世界像を受け取る
- **隠れた情報が送信されていないことをテストで検証できる**。表示上隠れているのではなく、送っていないこと
- 人間/AI の4通りの組み合わせすべてで DM サンプルが成立する

---

## Phase 6 — ハイブリッド同期 + RTS-lite

### 実装項目

- `concept:hybrid-synchronization` — コマンド上り、ワールド下り
- fog of war と interest management（`concept:agent-view` の可視性述語として）
- 2つの CBOR プロファイルの併用
- 1スロットが多数のユニットへ指示を出す構造

### 完了条件

- 大規模ワールドで `concept:delta-baseline-policy` の各モードの差が実測できる

---

## Phase 7 — AI育成パイプライン

意味のある corpus には Phase 4 以降の複雑さが要る。ただし **記録の口は Phase 2 で開けておくこと。**

### 実装項目

- `system:duckdb` による分析、JSONL → Parquet 変換
- `metric:balance-signals`、`metric:episode-outcome`
- `concept:continuous-match-loop` — round_robin / random / self_play / league
- `flow:behavior-tree-synthesis` の2段合成
  1. `data:derived-predicate` の提案（語彙層）
  2. 述語の上で条件を書く（`data:behavior-candidate`）
- `decision:shared-chip-library` — 承認済み候補は `data:behavior-chip` として共有ライブラリに入る。AI 1体 = `data:agent-loadout`（チップ選択 + `concept:behavior-profile`）
- `concept:tactic-selector` — ルートの作戦切り替え層。視界駆動・決定的、味方への作戦指示は普通の action
- `decision:behavior-tree-compiled-to-go` — loadout ごとに Go ソース生成
- `rule:predicate-tests-generated-from-episodes` — エピソードからのテスト自動生成
- `ui:behavior-tree-editor` — 4ペイン（ツリー / 候補 / エビデンス / 述語）+ レベル行列 + 差分
- `ui:chip-benchmark` — loadout 総当たり行列、チップ寄与、ablation、作戦頻度
- `concept:skill-level-gating` — chip タグの1次元として
- `policy:episode-data-governance`

### 完了条件

- 承認したツリーから生成した Go コードのエージェントが `flow:automated-playtest` を通る
- 再生成が承認済みノードを破壊せず差分として出る

---

## Phase 8 — サーバーレス経路

代替シグナリング経路であってコア能力ではない。Phase 3b のトランスポート抽象が正しければ差し込むだけ。

### 実装項目

- `system:webrtc` — 信頼順序チャネルと非順序非再送チャネルの2本立て
- `api:manual-signaling-token` — non-trickle ICE、宣言長つきトークン、fragment の除去
- `concept:static-host-mode` — 静的HTML配信
- `api:lan-discovery` — UDPブロードキャスト（ネイティブのみ）
- `flow:peer-authentication`、`decision:no-auth-on-lan`
- `rule:unauthenticated-admission-requires-scope-or-capability`

---

## 最初から継ぎ目だけ空けておくもの

**本計画で最も重要な項目。** 実装は自明でも、型として分けておかないと後で全面改修になる。いずれも Phase 1 では中身が数行で済む。

| 継ぎ目 | Phase 1 での実装 | 分けなかった場合 |
|---|---|---|
| `Sight` と `WorldState` を別型にする | 投影は恒等関数でよい | Phase 5 で全ゲームコードを書き換え |
| `data:evaluation-signal` のフィールドを置く | 勝敗だけでよい | Phase 7 で信用割当ができず corpus が無価値になる |
| `api:action-validator` のフック位置を作る | 常に通す実装でよい | Phase 4 で決定ループに手を入れることになる |
| `data:progress-report` の発行点を作る | terminal 1件だけでよい | Control Plane 連携が後付けの横断改修になる |

## 並行して継続するもの

以下は各フェーズの完了条件に含める。まとめて後半に回すと、どのフェーズで壊れたか分からなくなる。

- `api:runtime-observability` — そのフェーズで増えたメトリクスとイベント
- `data:runtime-resource-budget` — そのフェーズで増えた資源の上限宣言
- `concept:sample-acceptance-matrix` — 該当行の追加
- `policy:protocol-rollout` — スキーマ変更時のバージョン運用

## 順序を変える場合の判断基準

- **前倒しは自由** — Phase 8 のトランスポートは Phase 3b の後ならいつでもよい
- **後ろ倒しが危険なもの** — Phase 0 の固定小数点、Phase 2 の決定性検証、上表の4つの継ぎ目。いずれも「後から入れる」が「作り直す」と同義になる
- **飛ばせるサンプル** — `sample:tron` は Bomberman 系で代替可能（`concept:sample-progression` の backlog 参照）。他は能力の依存関係上、飛ばせない
