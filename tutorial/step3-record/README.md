# step 3 — シンプルなアルゴリズムの CPU を実装する

step 2 で LAN 上の相手と対戦できるようになりました。ただし相手が来なければ何も始まりません。
このステップの終わりには、空席に座る **CPU** ができています——`if` 4本のごく普通の
アルゴリズムを、**人間の席と同じインターフェース**に実装したものが。

ついでにもう一つ手に入ります。CPU がフレームワークの席に座っている以上、その判断は人間のものと
同じ形で記録に残ります。**遊べば遊ぶほど貯まる**ようになるのは、そのおまけのほうです。

```bash
go run ./tutorial/step3-record
```

step 2 を一人で起動すると、ロビーが `waiting for another player to join...` と言ったまま
止まります。誰も来ない。それだけの理由で、そこから先が一度も動きませんでした。

step 3 のロビーには、同じ行に続きがあります。

```
waiting for another player - click to play the bot instead
```

クリックすると空席にボットが座り、対局が始まります。終わると、起動したディレクトリに
`corpus/` ができています。

## CPU も席に座る

CPU を足すのに、ゲームのループへ分岐を書く場所はありません。書くのは `session.Agent` の
実装で、これは**人間の席と同じインターフェース**です。

```go
type Agent[S, A any] interface {
	Joined(slot SlotID)
	Observe(obs S)
	Decide(ctx context.Context) (action A, ok bool)
	Ended(result Result)
}
```

人が座っている席には `session.Detached` という何もしないスタブが入り、セッションは代わりに
その席の Inbox を読みます。セッションから見れば、**人間もボットも同じ型の穴に嵌まっている**。
ルールがどちらか聞かないのは、聞く口がないからです。

この4メソッドのうち、**判断が入るのは `Decide` だけ**です。残りの3つと、型パラメータ2つと、
`session.Agent` を満たしているという表明は、どれも決めることがありません。しかも型パラメータを
決めているのはこのファイルではなくルールセットの宣言で、片方は別のパッケージにあります。

```bash
cd tutorial/step3-record && ebigent add agent tactic --type Bot --file bot.go
```

step 2 で書いた `ebigent.toml` があるので、`add` は `var _ session.TickStageRuleSet[...]` を
読んで視界と行動の型を取り、`game/bot.go` に型・表明・ファクトリ・4メソッドを書きます。最後に
`NewAgent: NewBot,` と印刷して終わります。

`agent` の後ろは全部**質問**です。渡したオプションはその初期値になるだけなので、`ebigent add
agent` だけで起動しても3問答えれば同じところに着きます。名前は**方策の名前**で、`--type` と
`--file` は、このゲームがすでに `Bot` と呼んでいるものに合わせるためのものです。

残るのは `Decide` の中身だけです。三目並べの CPU は、4メソッドのうち2つしか使いません。
[`game/bot.go`](game/bot.go) の全部がこれです。

```go
type Bot struct {
	sight Sight
}

// Observe は視界を持っておくだけ。判断は Decide でやる。
func (b *Bot) Observe(sight Sight) { b.sight = sight }

func (b *Bot) Decide(context.Context) (msg.Move, bool) {
	s := b.sight

	// 手番でないか、対局が終わっている。視界がそう言っている。
	if len(s.Legal) == 0 {
		return msg.Move{}, false
	}
	// 1. 勝てるなら勝つ
	if cell, ok := completes(s.Cells, s.Mark); ok {
		return msg.Move{Cell: cell}, true
	}
	// 2. 負けるなら止める
	if cell, ok := completes(s.Cells, rival(s.Mark)); ok {
		return msg.Move{Cell: cell}, true
	}
	// 3. 中央 → 角 → 辺
	for _, cell := range preference {
		if slices.Contains(s.Legal, int(cell)) {
			return msg.Move{Cell: cell}, true
		}
	}
	return msg.Move{Cell: uint8(s.Legal[0])}, true
}
```

探索も、状態も、乱数もありません。読むべきところが3つあります。

**`b.sight` しか読んでいません。** `msg.TTTWorld` への参照がどこにもない。三目並べでは両者が
同じ盤を運ぶので差が出ませんが、隠し情報を持つゲームでこの同じボットがそのまま通るのは、
この一点のためです。

**ルールエンジンを持っていません。** 「どこが合法か」を自前で計算していない。`Sight.Legal` が
運んできたものを使うだけで、これは step 2 で視界に合法手を載せた見返りです。

**手番でないとき黙ります。** ローカルのエージェントはコミットされた世界ごとに叩かれるので、
毎回答えるボットは答えのほとんどを validator に撥ねられ、その全部が events ストリームに
rejection として残ります。`len(s.Legal) == 0` の1行がそれを防いでいます。

そして**この CPU は、記録のために何もしていません。** 席に座っているだけです。それでも
`Decide` が返した1手ずつが、そのとき見えていた視界とともに残ります。`Update` の中に書いた
敵の更新関数は、これを決して残しません。

## 空席は誰のものか

step 2 のホストは何も押さずに座りました。**選択肢が1つしかない問いを出しても仕方がない**
からで、そこには他に誰もおらず、空いているのは自分の席だけでした。

いまは二つあります。待つか、ボットと打つか。だから押す操作に意味が戻りました。

```go
Lobby: eb.LobbyOptions{
	NoBots:  true, // 空席は人のもの。勝手に埋めない
	StandIn: true, // ただし待つのをやめる権利は本人にある
},
```

`NoBots` はそのままです。空席の持ち主は依然として人で、その人が来れば対局はそちらで始まります。
`StandIn` が変えたのは、**待ち続けることが唯一の選択肢ではなくなった**という一点だけです。

## 席に着けるのは Binding の1フィールド

ルールをフレームワークに手渡す `Binding` には、「空席をどう埋めるか」を書く欄があります。
step 2 ではそこが意図的に空でした。埋めるのは `add` が最後に印刷した1行です。

```go
NewAgent: NewBot,
```

これで全部です。ルールは1行も変わっていません。`RuleSet` はボットという言葉を知らないし、
`main.go` はこの関数の存在を知りません。step 2 の冒頭で「`Update` の中に『いま O の番なら
ボットに考えさせる』と書きたくなる」と言った、その1行はどこにも現れませんでした。

同じ関数が、性質のまったく違う2つの呼び出しに答えています。ロビーのクリックは**来なかった人の
代わり**を1席に座らせ、`run.Serve` は**全席**をここから埋めて誰も見ていない対局を回します。
どちらも「席にエージェントを置く」という同じ動作なので、関数は1つで足ります。

### ボットはわざと弱い

三目並べを完璧に打つコードのほうが、上の4本より短く書けます。そうしなかったのは、**完璧な相手からは
引き分けと負けしか記録されない**からです。勝ちが1局も入っていないコーパスは、step 4 で
「勝つとはどういうことか」を一度も見せられません。

`TestTheBotCanBeBeaten` が勝ち筋を1本固定しています。誰かがボットを賢くしてこのテストが
赤くなったら、相手は強くなって記録は悪くなった、ということになります。

## 記録は Options の1フィールド

```go
Record: run.RecordOptions{Root: *corpus},
```

1局が1ディレクトリになります。

```
corpus/
├── tictactoe-0000/
│   ├── decisions.jsonl  誰が何を見て何をしたか
│   ├── events.jsonl     状態遷移と、validator が撥ねた入力
│   └── outcomes.jsonl   席ごとの決着
└── tictactoe-0001/
```

`world.jsonl` はここにはありません。既定の記録モードは `analysis_sampled` で、世界そのものは
落とします。ビット一致の再生まで残したいときは `Record.Mode` に `episode.ReplayComplete` を
書くと、世界ストリームとチェックポイントが増えます。

**起動しなおしても上書きされません。** 番号はコーパスにすでにあるエピソードの続きから
振られるので、今日3局、明日3局打てば6局になります。

これは名前の話だけではありません。**番号はシードでもあります。** 0番から振り直す実装は、
前回の記録を消したうえで同じシードをもう一度録ることになります。

## 何が記録されているのか

盤面の履歴ではありません。**その席に見えていたものと、そこで取った行動の対**です。

```json
{"tick":1,"slot":1,"agent_kind":"human",
 "sight":{"you":1,"mark":"X","cells":["-","-","-","-","-","-","-","-","-"],
          "turn":1,"legal":[0,1,2,3,4,5,6,7,8],"winner":0,"over":false,
          "signal":{"score":0,"progress":0,"evaluation":0,"reward_delta":0,
                    "terminal":"not_terminal"}},
 "action":{"cell":0},
 "evaluation":{"score":0,"progress":0,"evaluation":0,"reward_delta":0}}
```

盤面の履歴なら「何が起きたか」しか言えません。この行は「**選んだ人に何が見えていたか**」を
言っています。差はそのまま値段の差になります。9マス空いている盤で 0 を選んだのと、1マスしか
空いていない盤で 0 を選んだのとでは、同じ `{"cell":0}` でも意味がまるで違います。前者は判断で、
後者は消去法です。`legal` がその区別を持っています。

`agent_kind` が `human` なのは、その席に人が座っていたからです。ボットの行は同じ形で、
この列だけが `tactic` になります——`NewBot` が返した id が、そのまま列になっています。
**人とボットは記録の上で1つのコーパスに混ざります**——step 4 でどちらを蒸留するかは、
後から列で選ぶ話です。

この列が「ボットかどうか」ではなく**どのボットか**を持っているのは、ボットが2種類になった
瞬間に効いてきます。席番号で見分ける方法もありますが、それは種類が席を移した途端に壊れます。

`sight` の中の `signal` と、行の `evaluation` 列は同じ評価シグナルです。列のほうは常にあり、
中のほうは**このゲームが視界に載せると決めた**からあります。載せなければ列だけが残ります。

## 見る

```bash
go run ./cmd/ebigent analyze --corpus ./corpus
```

3局打ったあとの出力です。

```
episodes: 3
by protocol version:
  0464a0ebb7358018: 3
by recording mode:
  analysis_sampled: 3

outcomes by slot:
  slot 1: win=1 lose=1 draw=1
  slot 2: win=1 lose=1 draw=1
outcomes by agent kind:
  bot: win=1 lose=1 draw=1
  human: win=1 lose=1 draw=1

duration ticks: min=7 mean=8.33 max=10
decision rows: 28 (mean 9.33 per episode)
  with action: 22
  sight-only: 6
rejection reasons:
  (none)
```

`with action` と `sight-only` が分かれているのは、決着した局面が全席に配られ、誰も指さないまま
記録されるからです。3局なら6行。そこに勝敗が入っています。

`rejection reasons` が空なのは、ボットが自分の手番でないとき黙っているからです。ローカルの
エージェントはコミットされた世界ごとに叩かれるので、毎回答えるボットは答えのほとんどを
validator に撥ねられ、その全部がこのファイルに残ります。`TestBotPassesWhenTheSeatIsNotItsTurn`
がそれを止めています。

## 視界が「文書」になった

step 2 まで、`Sight` は1 tick だけプロセス内に存在する値でした。フィールドを何と呼ぼうと誰の
関知するところでもなかった。記録を始めた瞬間に、その名前は**列の名前**になります。

`json` タグが付いたのはそのためです。もう一つ、`Mark` に `MarshalText` が付きました。

```go
func (m Mark) MarshalText() ([]byte, error) { return []byte(m.String()), nil }
```

これがないと、`encoding/json` は `[]Mark` をバイト列とみなして base64 で書きます。

```json
"cells":"AAAAAAAAAAAA"
```

正しいし、往復もします。ただ、ログを読む人にも、そこに述語を書く人にも何の役にも立ちません。
`Legal` が `[]uint8` ではなく `[]int` なのも同じ理由です。ワイヤは通ればいい小ささを求め、
ログは選択できる形を求める。両者は違っていて構いません。

同じ罠に framework 側の型もかかっていました。`signal` の `terminal` が `3` ではなく `"draw"` と
書かれるのは、`session.Terminal` が同じ `MarshalText` を持ったからです。それが付いたのは、
このステップを書いている最中でした。

古いコーパスは数字のまま読めます。記録の形を変えることが、記録そのものを失うことに
なってはいけないからです。

## で、これをどうAIにするのか

コーパスから先へ進む道は、だいたい4本あります。

| 道 | 要るもの | 出るもの | なぜそう決めたか説明できるか |
|---|---|---|---|
| 手で書く | コーパスは要らない | 思ったとおりのボット | できる。ただしバランスを触るたび全部書き直し |
| 探索（minimax など） | コーパスは要らない | ほぼ最適な手 | 手は出るが、理由は木の中にしかない |
| **記録から蒸留** | 数百の決定 | 読める決定リスト、そこから Go | できる。しかも差分が出る |
| 学習（NN など） | 数万から | 強い方策 | できない |

このフレームワークが3番目を選んでいる理由は、右端の列にあります。生成された相手が理不尽に
感じられたとき、直せなければ意味がありません。蒸留パイプライン（`website` の「ログが AI に
なるまで」）は、承認を必ず人間に通す形になっています。

そして本当の分かれ目は道ではなく、**そのコーパスを誰が打ったか**です。

- **自分で打つ** — 数十局が限界です。ただし人がやりたがることが入っています
- **ボット同士を回す** — いくらでも回ります。ただし決定的なボット2体なら、100局回しても中身は
  1局分です。`TestTwoCopiesOfTheBotRecordTheSameGameEveryTime` がそれを固定しています
- **自己対戦・リーグ** — 違いを意図的に持ち込む。最終的にはここに行きます

順番はこの並びのままです。まず自分で打ち、蒸留して座らせ、座らせたものを相手に回す。
シミュレーションによる自動収集が効くのは3周目からで、1周目にそれをやると、種がないので何も
出てきません。step 3 が人の対局から始まるのは、それが一番簡単だからというより、**それ以外に
最初の種がない**からです。

## 確かめ方

| 主張 | テスト |
|---|---|
| 人が打った手が、そのとき見えていた視界つきで記録される | `TestYourOwnPlayIsWhatGetsRecorded` |
| 記録された行動は隣にある視界の上で合法だった（局面がずれていない） | 同上 |
| 人の行とボットの行が、ラベル1列だけ違う同じ形で1つのログに入る | 同上 |
| 窓なし・人なしでも同じルールがコーパスを産む | `TestPlayingWithNobodyWatchingProducesACorpus` |
| 同じボット2体を何局回しても記録は1局分にしかならない | `TestTwoCopiesOfTheBotRecordTheSameGameEveryTime` |
| ボットは勝てるとき勝ち、負けるとき止める | `TestBotTakesTheWin` / `TestBotBlocksTheLoss` |
| ボットは自分の手番でないとき黙る（events を汚さない） | `TestBotPassesWhenTheSeatIsNotItsTurn` |
| ボットは人間に負けうる | `TestTheBotCanBeBeaten` |
| step 2 の LAN 対戦は壊れていない | `TestTwoInstancesPlayOneBoard` |

## 構成

```
step3-record/
├── main.go            ウィンドウ・マウス・描画・ネットワーク・記録先の宣言
├── game/
│   ├── game.go        ルール。Sight に json タグ、Mark に text 形が付いた
│   ├── bot.go         決定リスト4段のボット
│   ├── bind.go        Options / Binding。NewAgent が埋まった
│   ├── game_test.go   ルールとコーデック
│   ├── bot_test.go    ボットの4規則と、負けうること
│   ├── corpus_test.go 記録されたものが step 4 の入力になっているか
│   └── net_test.go    2インスタンスの通し（step 2 から変わらず）
└── msg/               ワイヤ型と生成物（step 2 と同一）
```

## まだないもの

**ゲスト側は記録しません。** セッションを走らせているのはホストだけで、ゲストの手元には
決定を書き出す元になる視界の生成がありません。LAN で対局した記録は、ホストになった側の
マシンにだけ残ります。

**蒸留がありません。** `corpus/` を語彙に通して条件→行動の候補を出し、承認したものを Go に
落とし、生成されたエージェントを同じ席に座らせる。step 4 がそれになります。

いま座っているボットは、その比較対象として手で書いてあります。生成されたほうが強いかどうかは、
実のところ興味がありません。知りたいのは、**記録だけから、人が書いたものと同じ方策に戻れるか**
です。
