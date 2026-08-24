# step 5 — 相手を回す、そして限界を測る

step 4 で、記録からボットを作れるようになりました。ただし2つ引っかかっています——コーパスの
並びが正しかったのは運だったこと、そして教師より強くはならないこと。このステップの終わりには、
**片方が直り、もう片方は直らないことが分かっています**。

```bash
go run ./tutorial/step5-simulate
```

step 4 と同じように遊べます。相手も同じく生成された Go です。違うのは**その裏側**で、
このステップの成果物は半分がコードで、半分が測定結果です。

宿題はこの2つでした。

- 800局の並びが正しかったのは**運**だった。もっと良い相手を回せば直るのか
- 教師より強くはならない。**もっと強い教師**を使えばどうなるのか

順に測りました。

## 前段 — 相手を回すと、量の半分で足りる

`concept:continuous-match-loop` は相手の組み合わせ方を4つ挙げています。round_robin、
ランダムサンプリング、自己対戦、リーグ。

framework 側の [`matchloop`](../../matchloop/matchloop.go) はシードと集計だけを持ち、
**誰が誰と当たるかは呼び出し側に委ねます**。それはゲームの事実で、ループが推測できるもの
ではないからです。

3体を回します。コイン、step 3 の手書きボット、そして完璧な打ち手。

**強さのために選んでいません。違うことのために選んでいます。** コインはまともな打ち手が
辿り着かない局面をうろつき、手書きボットはコインが滅多に作らない「勝ちとブロック」を量産し、
完璧な打ち手は「好みが緊急事態に譲る」局面を確実に作ります。

```
rotated 200 games: 77.3% explained; coin 800 games: 72.5%
```

**回した200局が、コインの800局より説明できています。** だから step 5 の正式なコーパスは
400局で、step 4 の半分です。

## 後段 — 完璧な教師は蒸留できない

[`distill/teacher.go`](distill/teacher.go) の `Perfect` は全幅探索で、負けません。
三目並べは小さいので探索は網羅的かつ厳密です。これを**教師**——記録を残す側——と呼び、
蒸留して出てくる方策のほうを**生徒**と呼びます。

この教師で12通り測ると、こうなります。

```
judgement random       200 games  70.7% explained
judgement random       400 games  71.7% explained
judgement random       800 games  72.5% explained
judgement round_robin  200 games  77.3% explained
judgement round_robin  400 games  77.6% explained
judgement round_robin  800 games  77.8% explained
fork      random       200 games  77.0% explained
fork      random       400 games  77.5% explained
fork      random       800 games  78.3% explained
fork      round_robin  200 games  79.2% explained
fork      round_robin  400 games  79.4% explained
fork      round_robin  800 games  79.7% explained
best of twelve runs: 79.7%
```

**どのノブを回しても閉じません。** 局数を4倍にして +1.8pt、語彙を増やして +7pt、
相手を回して +2pt。手書きボットなら 100% だったところが、最良で 79.7% です。

### 原因は賢さではありません

三目並べの初手は、完全に打てばどれも引き分けです。だから探索は9通りに**同じ値**を返し、
何か恣意的なものが1つを選びます。`Perfect.Principled` を落とすとループの走査順が選び、
立てると中央→角→辺の好みが選ぶ。**強さは同一**です。どの最適手を選んでも最適だからです。

述語は局面についての事実しか言えません。そして局面は、その選択を決定していない。
**理由のないところに、蒸留するものはありません。**

step 4 が成立したのは、手書きボットが良かったからではありませんでした。**指し手のすべてに、
述語が名指せる理由があった**からです。

### 語彙を増やすと、黙る代わりに堂々と間違える

fork 述語（`creates_fork_at_k` / `blocks_fork_at_k`）を足したときの内訳です。

```
judgement: 57 silent, 12 wrong.  fork: 3 silent, 36 wrong
```

沈黙は 57 → 3 に減り、**間違いは 12 → 36 に増えます**。語彙が豊かになると、採掘器は
「コーパスが一度も否定しなかった規則」をより多く見つけられるようになる。そして
**コーパスが否定しなかった規則は、それだけでは正しくありません。**

### そもそも探索は座らせられます。ただし高い

`Perfect` はごく普通の `session.Agent` です。蒸留せずにそのまま席に着けられます。
値段だけが問題になります。

```
search 614.166µs per decision, list 55ns — 11167x
```

alpha-beta で枝刈りした上での数字です。枝刈りしない探索と比べるのはフェアではないので、
実際の探索と比べて**なお4桁**あります。30tps なら tick 予算は 33ms で、3×3 の盤の1手に
その 1.8% を1席が使う。

`rule:generated-agent-code-is-deterministic` には `cost_bound` という項があります。述語は
エージェントごと・tickごとに走るので、生成は無制限のスキャンを拒否する——言っているのは
このことです。

**生徒が教師と同じコストを払うなら、蒸留していないのと同じ**になります。

## これは完全ゲームの限界です

12通り試して届かなかったのだから、蒸留は強い教師に効かない——そう読めます。ですが測ったのは
そこではありません。**解けている完全情報ゲームの限界**であって、蒸留の限界ではない。

三目並べやオセロには最強の打ち手が存在し、その打ち手は同値な最適手の中から理由なく選びます。
だから「教師を強くする」戦略が効きません。

この framework が想定しているゲームでは、事情が逆になります。

- **格闘ゲームやアクションゲームの敵は、完璧だと面白くありません。** 適度に弱いことが仕様です
- **敵に求められるのは、深い思考ではなくパターン**です。読み合いではなく、読める動き
- そして教師は探索ではなく**人**になります。開発者が敵を操作して動きを付け、それを記録し、
  ゲーム内で再生する

最後のケースでは、恣意性の問題が起きません。人が付けた動きは**それ自体が仕様**であって、
「同値な選択肢の中の任意の1つ」ではないからです。step 3 が「自分で打った記録」から始めたのは、
最終的にそこへ行くためでした。

その形は別のチュートリアルで扱います。ボードゲーム編はここで終わりです。

## 確かめ方

| 主張 | テスト |
|---|---|
| 相手を回した200局が、コインの800局より説明できる | `TestRotatingTheOpponentBeatsPlayingMore` |
| 完璧な教師は、12通りどれでも8割に届かない | `TestAPerfectTeacherCannotBeDistilled` |
| fork 述語は沈黙を減らし、間違いを増やす | `TestForkWordsTradeSilenceForConfidence` |
| 探索は決定リストの1万倍以上かかる（枝刈り後） | `TestTheSearchCostsWhatTheListDoesNot` |
| 生成エージェントは到達可能な123局面すべてで手書きと一致する | `TestGeneratedAgentPlaysExactlyLikeTheBot` |
| コミット済みの生成物が古びていない | `TestGeneratedSourcesAreCurrent` |

2つ目は**通る否定的結果**です。「閉じなかった」を主張として固定してあるので、誰かが
語彙や採掘器を改良して8割を超えたら、このテストが赤くなって教えてくれます。

```bash
go test ./tutorial/step5-simulate/distill -update   # 再生成
go test -short ./tutorial/step5-simulate/...        # 重い測定を飛ばす
```

## 構成

```
step5-simulate/
├── main.go              ウィンドウ。step 4 から変わらない
├── game/                ルール。step 3 から1文字も変えていない
├── msg/                 ワイヤ型と生成物
└── distill/
    ├── distill.go       語彙3つ（naive / judgement / fork）
    ├── pairing.go       Pairing と matchloop 経由のコーパス生成
    ├── teacher.go       minimax。alpha-beta つき
    ├── pred/pred.go     判断語。fork 2語が増えた
    └── gen/             生成物（コミット済み、round_robin 400局から）
```

## まだないもの

**`ebigent simulate` は pending のままです。** ここで書いた `Pairing` はチュートリアルの中に
あり、CLI の verb にはなっていません。`matchloop` が汎用なのに対し、誰と当たるかはゲームごとの
話なので、そのまま verb にできるものではありません。

**リーグがありません。** `SelfPlay` は定義してありますが、過去バージョンを相手に残す
リーグ運用は入れていません。三目並べでは意味のある差が出ないためで、それを示すのに
1ステップ使う価値がないと判断しました。
