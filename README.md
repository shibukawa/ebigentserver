# ebigentserver

汎用ゲームサーバーフレームワーク。人間・ボット・リプレイを同一の `agent` 抽象で着席させるセッションランタイム（`vision:agent-session-runtime`）。

要件と設計判断は [.knowledge/](.knowledge/) に概念として記録されている。実装順は [plan.md](plan.md) を参照。

## 状態: 全フェーズ完了(Phase 0〜8)

### Phase 8 — サーバーレス経路

| 項目 | 状態 | 実装 |
|---|---|---|
| `system:webrtc` | 済 | [transport/rtc/](transport/rtc/) — pion/webrtc/v4。事前交渉済み2チャネル(ordered reliable + unordered/no-retransmit)、non-trickle ICE(有界gathering待ち)、リモートDTLS fingerprint公開(`rule:ticket-bound-to-connection`用)。js/wasm はwtと同じstubパターン |
| `api:manual-signaling-token` | 済 | [signaltoken/](signaltoken/) — version/type/expiry/宣言長ヘッダ(チャットが付け足すゴミは宣言長より先を無視)、SDP辞書置換+flate+base64url、単回償還`Redeemer`(`rule:invitation-is-single-use-and-expiring`)。URLフラグメント運搬、TURN資格情報は決して含まない |
| `api:lan-discovery` | 済 | [discovery/](discovery/) — UDPブロードキャストビーコン+受動リスナ、プロトコル版フィルタ、TTL失効。ネイティブのみ |
| `rule:unauthenticated-admission-requires-scope-or-capability` | 済 | `admission.GuardUnauthenticated` — loopback/private/link-local以外はfail closed。WebRTCのrendezvous能力側はlistening portが存在しないためガード不要(`decision:no-auth-on-lan`) |
| `concept:static-host-mode` / `flow:peer-authentication` | 部分 | 構成要素(rtc+token+fingerprint)は全部揃い、`TestPongOverWebRTC`が招待トークン貼り付け→償還→実WebRTC対局を通す。ブラウザ側WebRTC APIブリッジと相互チケット検証の実配線は未 |

### Phase 7 — AI育成パイプライン

| 項目 | 状態 | 実装 |
|---|---|---|
| `flow:behavior-tree-synthesis` 2段合成 | 済(貫通) | [behavior/](behavior/) — segment(エピソード→決定点) → 語彙(`data:derived-predicate`: 採掘用評価器+生成用Go式の対) → `Analyzer`が候補提案(`data:behavior-candidate`: coverage/反例/根拠つき) |
| 分析ステップ | 差替可能 | `Analyzer`はインターフェース。本来はLLM(`actor:llm-agent`)、現在は決定的な逐次被覆(決定リスト貪欲採掘)の`SequentialCovering`がベースライン。成果物の形は同一 |
| `decision:shared-chip-library` | 済 | 承認済み候補→`data:behavior-chip`(JSONライブラリ、タグ=level/style)。承認ゲートは`rule:generated-behavior-requires-approval`のとおり必須(現在はスクリプトが門番の代行) |
| `decision:behavior-tree-compiled-to-go` | 済 | [behavior/codegen.go](behavior/codegen.go) — approved chips→決定リストのswitch+`api:agent-interface`実装のGoソース。古びた述語はビルドエラー |
| `rule:predicate-tests-generated-from-episodes` | 済 | 記録済み局面をフィクスチャにしたテストを生成、episode+tickで失敗箇所を名指し |
| `rule:regeneration-preserves-approved-nodes` | 済 | [behavior/merge.go](behavior/merge.go) — new / unchanged / changed_metrics / matches_rejected(却下理由再提示) / conflicts_with_approved の差分。approved/rejectedは絶対に無言上書きしない |
| `concept:continuous-match-loop` | 済(最小) | [matchloop/](matchloop/) — fresh seed/試合 + `metric:episode-outcome`集計。pairing方針はplay関数側 |
| `data:agent-loadout` / `concept:tactic-selector` 生成 | 済 | [behavior/loadout.go](behavior/loadout.go) — 作戦セレクタ+チップ決定リストの静的Go生成。TTTで同一ライブラリから別個性(center-first)を組み立て |
| Reversi蒸留(判断語の語彙) | 済 | [samples/reversi/distill/](samples/reversi/distill/) — `best_move_is_k`(argmax判断)を[dpred](samples/reversi/distill/dpred/)としてレビュー可能なGoに。61チップ(到達可能最大)、GreedyBotとビット一致 |
| 分析 / `metric:balance-signals` | 済 | [analysis/](analysis/) + `ebigent analyze` — 純Go集計(kind別勝率・duration分布・拒否ランキング等)+ DuckDB用SQL生成(実duckdbで検証済み)。Parquet変換は未 |
| `ui:behavior-tree-editor` / `ui:chip-benchmark` | 済(初版) | [cmd/behavior-editor](cmd/behavior-editor/) — `ebigent edit` に統合予定。チップ承認/却下(理由付き)、証拠ペイン(episode+tickの実局面)、述語使用状況、levelタグ行列、再生成diff、ベンチ表。`-library chips.json`で起動 |
| LLM Analyzer | 済(skill) | [skills/behavior-analyze/](skills/behavior-analyze/) — LLMは開発者自身のagentic環境(Claude Code等)でskillとして分析する。導入=skillフォルダ共有+duckdb CLI(任意)。scriptは純Python stdlib。`ttt-export`がanalysis-request.json(語彙+bit列化済み決定)を出力し、提案は`validate_proposals.py`とGo側`behavior.ValidateProposals`が二重に検証(語彙外条件の拒否・coverage再計算・証拠実在確認)してから`ebigent merge`でdiffマージ。**提案は助言であって権威ではない** |

### 完了条件の対応

- **承認したツリーから生成したGoコードのエージェントが automated-playtest を通る** — [distill_test.go](samples/tictactoe/distill/distill_test.go): TTTのfirst-emptyボットの対局200戦(686決定)を蒸留→9チップ承認→[生成されたagent](samples/tictactoe/distill/gen/agent_gen.go)が50試合のplaytestを完走。**同一シード・同一相手で元ボットと勝敗・所要tickが完全一致**(方策の等価性の実証)。生成フィクスチャテスト24件も同梱。
- **再生成が承認済みノードを破壊せず差分として出る** — 同一コーパスの再合成→全て`unchanged`でライブラリファイルはバイト不変。却下チップは`matches_rejected`で旧理由つき再提示、approvedと矛盾する提案は`conflicts_with_approved`で明示判断待ち。

## 状態: Phase 6 — ハイブリッド同期 + RTS-lite

| 項目 | 状態 | 実装 |
|---|---|---|
| `concept:hybrid-synchronization` | 済 | [samples/rtslite/](samples/rtslite/) — コマンド上り(wire profile datagram)、fog投影の delta 下り。`flow:hybrid-sync-exchange` を損失リンク上のフルスタックで検証 |
| コマンドストリーム(1スロット→多数ユニット) | 済 | [session/tuning.go](session/tuning.go) の `InputIntake`: IntakeNewest(連続操作) / IntakeAll(命令列、slot内FIFO=`rule:deterministic-tick-commit`のper-slot sequence)。`decision:input-timing-owned-by-sync-mode`の実装点 |
| fog of war と interest management | 済 | `ProjectPlayer` — 自軍全量+視界内の敵のみ。Phase 5の`ProjectedSender`の述語を書くだけで成立(基盤の正しさの証明) |
| 2つのCBORプロファイル併用 | 済 | コマンド13B(wire profile固定配列) vs view snapshot 600B+(world profile map)、テストで実測 |
| unit ID | 済 | owner slotを上位byteに詰めた`decision:owner-namespaced-entity-ids`のワイヤ簡約形。所有権validatorの根拠 |

### 完了条件の対応

- **大規模ワールドで delta-baseline-policy 各モードの差が実測できる** — [baseline_test.go](samples/rtslite/rtslite/baseline_test.go): 128ユニットの4人戦をfog投影senderに流し、ack遅延6send+RTTスパイクを注入して3モードを計測。**speculative 11068B < bounded(16) 13053B < confirmed_only 15809B(+43%)**。deltaサイズ・snapshot回数の内訳もログに出る。
- 複数命令/tickの記録→再生ビット一致(decisions/outcomes/world全一致+checkpoint鎖一致。eventsは原走行のみvalidator拒否を含むため意図的に除外)、最終checkpointのクロスアーキ固定(tick 101)も継続。

## 状態: Phase 5 — 投影と非対称性 + Dungeon

| 項目 | 状態 | 実装 |
|---|---|---|
| `concept:agent-view` | 済 | [statesync/view.go](statesync/view.go) — `ProjectedSender`: world→**slot別view**→保持→diff→送信。deltaの機構をそのまま再利用 |
| `concept:visibility-scope` | 済 | self / team / role を dungeon サンプルで実証(global は既存サンプル)。**投影がシリアライズより前**なので、隠れた状態はエンコードすらされない |
| `data:visibility-annotation` | 済 | [session/visibility.go](session/visibility.go) — scope / schema / visible_entities / derived / affordances / evaluation_scope。ゲームが明示発行し、観測に埋めて記録される |
| `rule:observation-content-owned-by-game` | 済 | 可視性述語とフィールド選択は [project.go](samples/dungeon/dungeon/project.go)(ゲーム側)、保持・diff・配送は framework |
| 役割とチーム | 済 | scout(視界広) / engineer(罠解除) / carrier(宝運搬) / navigator(出口既知)。行動も情報も役割でゲート |
| `rule:evaluation-respects-visibility-scope` | 済 | パーティのsignalはチーム可視の事実のみから計算(隠し罠を置いても不変、テストで検証)。DMはprivilegedをannotationで宣言 |
| `sample:cooperative-maze` → `sample:dungeon-master` | 済(統合) | [samples/dungeon/](samples/dungeon/) — 両サンプルの能力(チーム協力+役割 / 種類の違うview)を1本に統合。梯子の粒度からの意図的な逸脱 |

### 完了条件の対応

- **DMとパーティが種類の違う世界像を受け取る** — `DMView`(全マップ)と`AdventurerView`(探索済みセル+発見済み罠のみ)は**別の生成struct**。netplayクライアントも別のview型でインスタンス化される。
- **隠れた情報が送信されていないことをテストで検証** — `TestHiddenInfoNeverOnTheWire`: scoutクライアントに届いた**全バイト列**を捕捉し、独立したdecode鎖で再構成。180パケット全てで「未発見罠なし・未探索セルの壁情報なし・出口座標なし」を検証(その間サーバ側にはDMの罠が8個存在)。表示フィルタではなく、ワイヤに載っていない。
- **人間/AIの4通りの組み合わせ** — `TestAllFourControllerCombos`: DM{bot, scripted human} × party{bot, scripted human} の4セッション全てが完走し、両側が各自のview型を受信。

## 状態: Phase 4 — 多人数と運用堅牢性 + Tron

| 項目 | 状態 | 実装 |
|---|---|---|
| 多人数フィールド(人間とボットの混在) | 済 | [samples/tron/](samples/tron/) — 2〜8スロット、trail は append-only identity collection で delta が snapshot より明確に安い形 |
| `permission:spectator-receive-only` | 済 | [netplay/](netplay/) — spectator ロールは inbox を持たず、入力datagram は違反として計数。クライアント側は自動で dedicated ack |
| `concept:agent-departure-policy` / `concept:agent-proxy-designation` | 済 | 離脱検出(transport close + 沈黙deadline) → `OnDeparture` コールバック。ai_takeover は「新しい admission でbotを着席」= 追加機構ゼロ(`decision:agent-as-central-abstraction`の実証) |
| `decision:host-loss-ends-session` | 済 | クライアントは `ErrSessionLost` を報告して戻るのみ。移行なし |
| `api:action-validator` 2クラス | 済 | legality(決定的) + plausibility(権威側ヒューリスティック、[session/validator.go](session/validator.go))。rejection計数 → tuning宣言のしきい値で `OnSuspect` → netplay が切断 |
| `data:runtime-resource-budget` | 済 | [budget/](budget/) — 宣言必須・起動時検証。接続容量(handshake中も計上)、入力レート(token bucket)を enforcement |
| `policy:overload-handling` / `policy:realtime-abuse-protection` | 済 | 容量超過は割り当て前に fail closed、不正入力/レート超過/観戦者入力は計数→しきい値切断、全てに証拠 |
| `api:runtime-observability` | 済 | [observe/](observe/) — 有界カウンタ + 構造化イベント(session/conn/tick/reason付き、credential禁止) |

### 完了条件の対応

- **8人+観戦者で、切断・過負荷・不正入力を注入しても production-runtime-safety を満たす** — [net_test.go](samples/tron/tron/net_test.go) `TestEightPlayersSpectatorsAndInjectedFailures`: 損失10%+遅延の8人対戦に、①途中切断→AI takeover(slot 3)、②途中切断→continue_without(slot 5)、③不正データ洪水プレイヤー→しきい値切断、④入力を送る観戦者→違反計数→切断、を注入。セッションは正常終了し、正直なクライアントと観戦者は同期を維持、全違反に observability 証拠が残る。
- **8受信者での baseline 保持コスト実測** — 同テストが試合中に実測: 7受信者 × history_depth 16版 × スナップショット約11.6KB ≈ 1.3MiB。trail が伸びる状態での history_depth 選定材料がそのまま出る。

## 状態: Phase 3b — ネットワーク

| 項目 | 状態 | 実装 |
|---|---|---|
| `api:transport-interface` | 済 | [transport/](transport/transport.go) — capability宣言つきConn。実装: [pipe](transport/pipe/pipe.go)(プロセス内+障害注入)、[ws](transport/ws/ws.go)(WebSocket、reliable-onlyフォールバック)、[wt](transport/wt/wt.go)(WebTransport: QUIC datagram+reliable stream、`decision:webtransport-primary-for-wasm`。ネイティブのみ、js側はブラウザAPIブリッジ待ち) |
| `api:message-framing` | 済 | [transport/framing/](transport/framing/framing.go) — 12KB分割・再構成・不正フレーム破棄・部分フラッド上限 |
| `api:sequence-ack-layer` | 済 | [transport/seqack/](transport/seqack/seqack.go) — seq番号+ack bitfield、confirmed tag、RTT/loss推定、沈黙検出素材 |
| `concept:delta-baseline-policy` | 済 | statesyncに speculative / confirmed_only / bounded_speculation。tuning profileで宣言 |
| `concept:ack-transmission-policy` | 済 | piggyback_only / dedicated / delayed_piggyback |
| `flow:session-admission` | 済 | [admission/](admission/ticket.go) — Ed25519 JWT(`rule:asymmetric-ticket-signature`)、ローカル検証、jti一回償還、kid複数受理。handshakeはバージョン照合が最初 |
| `policy:protocol-rollout` | 概念のみ | エンドポイント分離は運用事項。バージョン照合とチケットaudienceが機構側の担保 |

### 完了条件の対応

- **損失と遅延を注入した状態でPongが破綻しない** — `TestPongSurvivesLossAndLatency`: 20%損失+25ms遅延+jitter+並べ替えの両方向注入でフルスタック(admission→seqack→statesync bounded_speculation→resync)を3秒走行。全クライアントが終局tickの7割以上まで再構成を維持し、RTT/loss計測も生きている。
- **バージョン不一致の接続がhandshakeで明示的に拒否される** — `TestVersionMismatchIsRejectedExplicitly`: 双方のバージョンを名指しする明示エラーで拒否(`rule:protocol-version-must-match`、交渉はしない)。
- 実WebSocket(`TestPongOverWebSocket`)と実WebTransport(`TestPongOverWebTransport`、QUIC datagramで状態ストリーム)でも同スタックがlocalhost実対局で走る。

## 状態: Phase 3a — ローカルリアルタイム + Pong

| 項目 | 状態 | 実装 |
|---|---|---|
| tickループと権威シミュレーション | 済 | [session/realtime.go](session/realtime.go) — `TickGame`(Apply+Advance)、`RunRealtime`。入力は`Inbox`(非ブロッキング、tickごとに最新1件)、遅延agentはそのtick無入力 |
| `data:player-input` / `data:snapshot` / `data:state-delta` | 済 | [samples/pong/msg](samples/pong/msg/types.go) — wire/worldプロファイル生成、状態=wire形そのもの |
| `decision:framework-side-delta-generation` | 済 | [statesync/](statesync/) — 受信者ごとのSender(保持リング+speculative baseline)、Receiver、loopback Hub。baseline喪失→snapshot fallback |
| `data:session-tuning-profile` | 済 | [session/tuning.go](session/tuning.go) — 宣言必須(デフォルトなし)、整合検査つき |
| `sample:pong` ループバック | 済 | [samples/pong/](samples/pong/) — 固定小数点物理、hub経由のsnapshot/delta、bot対bot |
| `concept:game-time-control` | 済(一部) | Paced / Unlimited。step は Phase 1 実装済み、slowed は未対応 |

### 完了条件の対応(3a分)

- **記録したリアルタイム対局の再生がビット一致** — `TestRealtimeRecordReplaysBitIdentical`: 記録から`ReadReplaySchedule`で「どのtickにどの入力が受理されたか」を復元し、再実行すると4ストリーム全てバイト一致。
- **クロスアーキテクチャ** — `TestScriptedMatchDigestPinned`: スクリプト入力の585tick対局の最終checkpointを定数固定(arm64開発機 vs amd64 CI)。
- 損失・遅延注入(3bの完了条件)はネットワーク実装後。ただしHubの輻輳ドロップ→resyncの経路は`TestLossForcesResync`で検証済み。

## 状態: Phase 2 — 記録と決定性の検証 + Reversi

| 項目 | 状態 | 実装 |
|---|---|---|
| 合法手列挙を observation に載せる | 済 | [reversi.go](samples/reversi/reversi/reversi.go) — `Observation.Legal`（flip数=affordance付き）。botはルールエンジンを持たない |
| 探索AIと AI vs AI 対戦 | 済 | [bot.go](samples/reversi/reversi/bot.go) — GreedyBot(1-ply) vs FirstBot。方針: AIの深さはBT蒸留(Phase 7)で得るため意図的に最小 |
| `data:episode-log`（JSONL 4ストリーム） | 済 | [episode/](episode/) — decisions / events / outcomes / world、全ストリーム共通ヘッダ行 |
| `concept:episode-recording-mode` | 済 | replay_complete / analysis_sampled。sampledはworld+checkpointを落とし、replay readerが拒否 |
| `data:decision-record` | 済 | 配信された観測そのものを記録、action/evaluation/agent_kind/latency付き |
| `actor:replay-agent` | 済 | [session/replay.go](session/replay.go) + [episode/reader.go](episode/reader.go) — 記録から着席する通常のagent |
| `rule:shared-rng-seed` | 済 | `Config.Seed` → `Game.Start(seed)`、ヘッダに記録 |
| `data:state-checkpoint` | 済 | [session/record.go](session/record.go) — tick + world hash + accepted action hash、毎tick発行 |

### 完了条件の対応

- **記録した対局の再生がビット一致する** — [record_test.go](samples/reversi/reversi/record_test.go) `TestRecordedMatchReplaysBitIdentical`: ログだけからreplay agentを着席させ、4ストリーム全てがバイト単位で一致。
- **arm64とamd64で同じエピソードのダイジェストが一致する** — `TestFinalCheckpointIsPinnedAcrossArchitectures`: 正準対局(greedy vs first、64手)の最終checkpointを定数で固定。開発機(darwin/arm64)とCI(linux/amd64)の両方で走ることで一致を実証。

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

- `session` — セッション核。lifecycle 状態機械、slot 順の決定的コミット、agent interface、evaluation signal、action validator フック、progress report 発行点、記録フック(Recorder)とcheckpoint。
- `episode` — `data:episode-log` の JSONL 読み書き。session.Recorder 実装の Writer と、replay 用 Reader。
- `entity` — owner 名前空間つきエンティティ ID と決定的 allocator。
- `samples/tictactoe` — 最小サンプル兼回帰ハーネス（`decision:samples-as-test-infrastructure`）。`go run ./samples/tictactoe/cmd/ttt` で人間対ボット。
- `behavior` — 蒸留パイプライン: 語彙・分析(Analyzer差替可)・チップライブラリ・再生成マージ・Goコード生成。
- `matchloop` — 無人連続対局と結果集計。
- `analysis` — corpus集計とDuckDB SQL生成(ゲームプロセス外の分析ツール)。
- `config/buildconf`, `config/runconf`, `config/confload` — `ebigent.toml` 1ファイルを prefix でセクション分けして bind。既定 < ファイル < 環境変数 < オプションの順で上書き。
- `scaffold` — `ebigent init` が書き出すプロジェクト雛形。既定は Ebitengine の Flappy Bird 風(2羽が同じパイプ列を飛ぶ、操作はflapのみ)で、リアルタイムsession・固定小数点物理・シード付きRNG・engineをclientエントリに閉じ込める構成が最初から動く。生成物がビルドでき自身のテスト(境界テスト含む)が通ることをテストで担保している。
- `cli`, `cmd/ebigent` — ツールチェーン本体。
- `signaltoken` — 帯域外シグナリングトークン(WebRTC招待/応答)。
- `discovery` — LANセッション発見ビーコン。
- `netplay` — セッションを実トランスポートに接続する汎用層。admission→peer、view別状態配信(`MakeSender`ファクトリで席・役割ごとに投影を選択)、観戦者enforcement、離脱検出、abuse対策。
- `budget` — `data:runtime-resource-budget` の宣言と起動時検証。
- `observe` — 有界カウンタと構造化イベント(`api:runtime-observability`)。
- `transport` — トランスポート抽象とその実装(pipe/ws/wt)、framing、sequence/ack層。
- `admission` — Ed25519署名のsession ticketとhandshake(バージョン照合→ローカル検証→着席)。
- `statesync` — framework側delta生成。生成コーデックを差し込む`Codec`、受信者ごとの`Sender`/`Receiver`(双方が履歴保持)、baseline mode 3種、ループバック`Hub`。
- `samples/reversi` — 合法手列挙つき観測と記録/再生の実証。`go run ./samples/reversi/cmd/reversi` で人間対greedy bot、`-record=DIR` でエピソード記録。
- `samples/pong` — 最小リアルタイム。`go run ./samples/pong/cmd/pong` でbot対bot(観戦チャネルがスコア表示)、`-record=DIR` 対応。`go run ./samples/pong/cmd/pong-client` でEbitengine描画クライアント(W/S・↑/↓で左パドル操作、`-left=bot`で観戦)。clientエントリだけがengineをimportでき(`samples/*/cmd/*client*`)、headlessターゲット(wasip1/386)のビルドマトリクスからは除外される。
- `importcheck` — 依存の閉じ込め検査。ゲームモジュールは `importcheck.Enforce(t, ".", importcheck.Default())` を 1 テスト持つ。
- `examples/phase0` — Phase 0 の証明: 固定小数点型 + 生成 CBOR コーデック + delta、build target ごとの cmd エントリポイント。
- `examples/phase0/sim` — Phase 0 スタック全体（fixmath の Sin/Atan2/Sqrt → 宣言スケール量子化 → 生成 delta エンコード）を通した決定的エピソード。digest をテストで固定しており、これが Phase 2 のクロスアーキテクチャ検証の種になる。

## ツールチェーン

```bash
go build -o bin/ebigent ./cmd/ebigent
```

`ebigent` 1つに統合済み(`decision:one-ebigent-binary`)。`corpus-report` は `ebigent analyze`、`behavior-merge` は `ebigent merge` になった。`behavior-editor` は `ebigent edit` として `ui:dev-console` に統合予定で、それまでは単体コマンドのまま残る。

```bash
ebigent init            # ウィザードでプロジェクト雛形を生成
ebigent build [target]  # 宣言済み build target をビルド
ebigent config show     # 実効値と、それを設定した層
ebigent doctor          # 動かない理由
ebigent --help          # 全 verb
```

設定オプションは verb の**前**に置く(verb は自分の名前より後ろの引数を全部取るため):

```bash
ebigent --run-topology dedicated build server
```

`dev` / `run` / `simulate` / `replay` / `edit` は宣言済みだが未実装で、実行すると何待ちかを表示する。

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
