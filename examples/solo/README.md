# solo — ソロゲームでフレームワークを使う最小の一周

一人が二体の敵に追われ、時間切れまで捕まらなければ勝ち。それだけのゲームだが、**敵が席(`concept:player-slot`)に座っている**。

ソロゲームにセッションフレームワークを使う理由はそこにある。敵は遠隔プレイヤーと同じ `api:agent-interface` を通して決める。したがって敵の判断は一つ残らず、**判断の材料になった観測と一緒に** `data:episode-log` に残る。これは手書きの敵更新関数が決して残さないものであり、`flow:behavior-tree-synthesis` が必要とするただ一つの入力でもある。

`api:tick-hooks` の3つのフックのうち、非プレイヤーでも **入力(intake)** と **取り出し(apply)** は動く。中央で動くのは調停(arbitrate)だけで、ソロならそれも同じプロセスの中にある。

## 一周させる

```bash
go run ./examples/solo/cmd/solo-client
```

窓が開き、ロビーが出る。キーを押すと着席して即開始(`AutoStart`)。矢印キーかWASDで逃げる。`-record DIR` を付ければ人間の対局もコーパスになる。

```bash
go run ./examples/solo/cmd/solo-sim -matches 20 -record /tmp/solo
```

誰も見ていない対局。プレイヤー席も含め全席がエージェントで埋まり、`session.Unlimited` で数秒で終わる。ルールは窓版と同一のものがリンクされている。

```bash
go run ./cmd/ebigent analyze --corpus /tmp/solo
```

席ごとの勝敗、決定行数、rejection、ストリームのバイト数。

```bash
go run ./examples/solo/cmd/solo-distill
```

コーパスを録り、敵の種別ごとに決定を採掘し、承認済みチップをGoに落とす:

```
chaser   3739 decisions → 4 chips → examples/solo/distill/gen/chaser
    quarry_left_on_wide_axis    → move_left    coverage 1180
    quarry_below_on_wide_axis   → move_down    coverage 1098
    quarry_right_on_wide_axis   → move_right   coverage 838
    quarry_above_on_wide_axis   → move_up      coverage 623
flanker  3739 decisions → 8 chips → examples/solo/distill/gen/flanker
    quarry_left_on_narrow_axis  → move_left    coverage 1133
    ...
```

生成物は [distill/gen/chaser/agent_gen.go](distill/gen/chaser/agent_gen.go) にコミットされている。読めるGoであり、diffできる。

## 何が証明されているか

| 主張 | テスト |
|---|---|
| 同じシードは同じ試合を produce する(チェックポイントをピン留め) | `TestMatchIsDeterministic` |
| コーパスに勝ちと負けの両方が入る(片方だけでは学習素材にならない) | `TestCorpusCarriesBothOutcomes` |
| 敵2種は本当に別物 | `TestEnemiesDisagree` |
| 全敵の判断が観測つきで記録され、`analysis` が読める形になる | `TestSoloProducesATrainableCorpus` |
| 記録された観測が蒸留に必要な中身を持っている | `TestRecordedDecisionCarriesItsObservation` |
| 記録された判断が一つ残らず語彙で説明できる | `TestEveryDecisionIsCovered` |
| **同じ語彙・同じコーパスから、2種の敵が別の決定リストになる** | `TestKindsDistillDifferently` |
| コミット済み生成コードが古びていない | `TestGeneratedCodeMatchesTheCorpus` |
| **蒸留した敵を座らせると、手書きの敵と1tickも違わない試合になる** | `TestDistilledEnemiesPlayTheSameMatch` |

最後の2つが一周の閉じ目だ。語彙は両種で共有されている——述語は「獲物がどちらにいるか」「どちらの軸で離れているか」という**世界についての事実**でしかない。それでも採掘結果は別物になる。もし同じ決定リストが出てきたら、決めているのはコーパスではなく語彙だったということで、`TestKindsDistillDifferently` はそれを落とす。

## 構成

```
game/          ルール。エンジンをimportしない。両方のエントリが同じものをリンクする
  game.go      世界・行動・観測・評価・validator・Canonical
  agent.go     Chaser / Flanker / Runner — 意図的に最小
  bind.go      run.Options と run.Binding。宣言はここ1箇所
cmd/solo-client/   窓。ebitengineをimportする唯一のパッケージ
cmd/solo-sim/      ヘッドレス。エンジンをimportしない(タグではなく、ただ書いていない)
cmd/solo-distill/  コーパス → チップ → Go
distill/       語彙と蒸留パイプライン
  gen/chaser/  生成された敵(コミット済み)
  gen/flanker/
```

`ebigent init` が solo で生成するのはこの形だ。違いは、`init` の雛形が遊べるプレースホルダである一方、こちらは**一周が閉じていることをテストで示す**ところにある。

## 手を入れるとき

- 敵の性格を変える → `agent.go` を書き換え、`solo-distill` を再実行し、生成コードのdiffを読む
- 敵を賢くする → 語彙(`distill/distill.go` の `Vocabulary`)に述語を足す。足りなければ `Synthesize` が「被覆できなかった」と言って止まる
- 人間の対局から学ぶ → `solo-client -record DIR` で録り、`distill.Records(DIR, game.Player)` で自分の席を採掘する。フレームワークから見れば人間もエージェントなので、追加の機構は要らない
- ネットワーク対応にする → ルールは1行も変わらない。席の埋め方(`api:roster`)が変わるだけ
