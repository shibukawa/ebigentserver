# step 4 — 人の指し手からボットを作る

step 3 で、遊んだ分だけ対局が記録として貯まるようになりました。このステップの終わりには、
**その記録から作ったボット**が相手をしています——手で書いたのではなく、記録から掘り出して
Go に落としたものが。

```bash
go run ./tutorial/step4-distill
```

step 3 と同じように遊べます。違うのは、相手をしているボットを**誰も書いていない**ことです。

本命は、**あなた自身の指し手**です。このステップの最後には、同じ道具をあなたの記録に向けて、
自分の写しと対戦します。先にボットの写しから始めるのは、ソースコードと突き合わせて
答え合わせできる教師がボットだけだからです。

[`distill/gen/agent_gen.go`](distill/gen/agent_gen.go) がその中身で、生成された Go です。

```go
func Decide(obs game.Sight) (msg.Move, bool) {
	switch {
	case pred.PreferredCellIs(obs, 4):
		// chip preferred_cell_is_4 → play_4 (coverage 800, counterexamples 0)
		return msg.Move{Cell: 4}, true
	case pred.PreferredCellIs(obs, 0):
		// chip preferred_cell_is_0 → play_0 (coverage 706, counterexamples 0)
		return msg.Move{Cell: 0}, true
	case pred.WinningMoveIs(obs, 8):
		// chip winning_move_is_8 → play_8 (coverage 516, counterexamples 0)
		return msg.Move{Cell: 8}, true
	...
```

switch そのものが**決定リスト**です。上から順に読んで、最初に条件が当たったものを採る。
1ケースが**チップ**1つ——人が承認した規則1つ——にあたり、コメントの `coverage` は
その条件を支持した記録の数になります。読めるし、diff も取れます。

## 何をしたのか

step 3 のボットの対局を記録し、そこから条件→行動の規則を採掘し、反例のなかったものを承認し、
Go に落として、同じ席に座らせた。パイプライン自体は framework のもの（`behavior` パッケージ）で、
このディレクトリが供給しているのは2つだけです。

- **コーパスの作り方** — [`distill/distill.go`](distill/distill.go) の `Corpus`
- **規則が使ってよい言葉**（これを**語彙**と呼びます） — [`distill/pred/pred.go`](distill/pred/pred.go)

コーパスの作り方は難しくありません。難しいのは語彙のほうです。語の意味は
[用語の整理](../../website/src/content/docs/architecture/terms.mdx)にまとめてあります。

## 最初の語彙は失敗します

盤面から素直に思いつく**述語**——視界に対する、名前の付いた真偽の判断——は「セル k が
空いている」でしょう。9個並べ、コーパスから「この条件のときこう指していた」を**採掘**すると、
こうなります。

```
naive: 8 chips, 4 approved, 386/652 decisions covered (59.2%)
```

**エラーは出ません。** `Propose` は「説明できなかった決定」を0件と報告します。それでも承認まで
残ったチップが説明できるのは、ボットの決定の**6割弱**です。この割合を**被覆**と呼びます。

理由は**反例**——条件は当たっているのに、違う行動を取っていた記録——です。「セル8が空いている
→ 8に置く」という規則は、たいていの局面で正しい。ただし**勝てる手があるとき**と
**止めなければ負けるとき**は、ボットは別のセルを選びます。その1手ずつが反例に数えられ、
反例のあるチップは自動承認されません。承認されなければ生成にも入りません。

結果として出てくるのは、**間違ったボットではなく、4割の局面で黙るボット**です。

## 判断に名前を付ける

盤面が言っていない区別に名前を付けます。`pred` パッケージがそれで、3つあります。

| 述語 | 意味 |
|---|---|
| `winning_move_is_k` | この席は k に置けばラインが揃う |
| `blocking_move_is_k` | k を取らないと相手が次に揃える |
| `preferred_cell_is_k` | 急ぎがないとき取るのは k（中央→角→辺） |

同じコーパスで採掘し直すと閉じます。

```
judgement: 19 chips, all approved, 2632/2632 decisions covered
```

**単数形が効いています。** 勝てるセルが同時に2つあることはあり、両方に true を返す述語だと
2つのチップが同じ局面を主張して、記録の半分と食い違います。だから tie-break を**述語の中で**
決めてあります——`game.Lines` と同じ順で。オセロの `BestMoveIs` が argmax を述語に畳んだのと
同じ手です。

**採掘器は意図的に無力です。** 条件は述語1個で、連言は学習しません。表現力は全部語彙側にあり、
賢さを入れる場所もそこしかありません。

## 順序は誰も教えていない——そして人間の順序でもない

生成された19ケースの並びは、こうなっています。

```
 1  preferred_cell_is_4    coverage 800
 2  preferred_cell_is_0    coverage 706
 3  winning_move_is_8      coverage 516
 4  winning_move_is_6      coverage 122
 ...
12  blocking_move_is_7     coverage  40
13  preferred_cell_is_2    coverage 174
 ...
```

「勝ち → ブロック → 好み」を期待していたなら、**先頭の2つが違います**。好みの規則が勝ちより
上にいる。優先順位を誰も採掘器に教えていないので、これは採掘器が自分で決めた並びです。

そして**正しい**。X は必ず中央から入るので、「4 が最優先の空きマス」が成立するのは X の1手目
だけで、そこに勝ちもブロックも存在しません（どちらも自分か相手の2つ目のマークが要る）。
`preferred_cell_is_0` も同じで、相手のマークが1個しかない局面です。この2つは勝ち・ブロックと
**同時に成立しえない**ので、上に置いても何も奪いません。

13行目以降の好みの規則は、ちゃんと勝ちとブロックの下にいます。

`SequentialCovering` は「コーパスが一度も否定しなかった規則」を優先します。否定されなかった
以上、上に置いてよい。つまり採掘器が復元したのは**人間が書いた順序ではなく、到達可能な範囲で
等価な別の順序**です。

生成された switch を眺めて納得するのではなく、全数で突き合わせるテストを置いてあるのは
このためです。読んで「順番が違う」と思ったものが、正しいことがあります。

## コーパスは自分の穴を見られません

3つのサイズで採掘すると、こうなります。

| 局数 | チップ | 黙る盤面 | 間違える盤面 |
|---|---|---|---|
| 200 | 16 | 8 | 2 |
| **400** | **19** | 0 | **2** |
| 800 | 19 | 0 | 0 |

**どのサイズも、パイプラインの中からは同じに見えます。** 反例ゼロ、被覆100%、`uncovered` は0件。

見るべきは真ん中の行です。400局でチップ数が**打ち止まります**——次に倍にしても新しい規則は
1つも出てきません。完成したように見える最も説得力のある信号がここで出ます。それでも生成された
エージェントは、到達可能な盤面を2つ間違えます。規則は全部揃っていて、**並びだけが違います**。

```
400 games: same 19 chips as the full corpus, and 2 boards answered wrongly (e.g. [1 0 0 0 1 2 1 2 2])
```

その盤面では、X は 3 に置けば勝ちで、同時に 2 を取らないと相手が次に揃えます。ボットは勝ちを
取ります。生成されたエージェントは**ブロックします**——勝てるのに止めに行く、という人間にも
一目で分かる間違いです。

原因は上位2件の並びだけです。

```
400局         800局
 6  blocking_move_is_2  cov 22      6  winning_move_is_3   cov 46
 7  winning_move_is_3   cov 14      7  blocking_move_is_2  cov 38
```

ここが居心地の悪いところです。**倍増して直った理由は、証拠が増えたからではありません。**
`SequentialCovering` は反例のない規則のうち被覆の大きいものから採ります。400局では
`blocking_move_is_2` のほうが支持記録が多く、800局では逆転した。それだけです。

どちらのサイズでも、`blocking_move_is_2` は反例ゼロです。つまり**どちらのコーパスも、
この2つのどちらが上かを知りません。** 800局の並びが正しいのは、頻度がたまたまそう転んだ
からで、何かが検証したからではありません。

決定リストの順序は、コーパスが述べている情報ではありません。**「反例なし」はコーパスに
ついての言明であって、ゲームについての言明ではありません。** そして違いを教えてくれたのは
被覆率でもチップ数でもなく、到達可能な局面を全部指してみたテストだけでした。

framework が承認を人間に通し、最後に `flow:automated-playtest` を置いているのは、
このためです。

## 確かめ方

| 主張 | テスト |
|---|---|
| ナイーブな語彙は、エラーなしにボットの大半を落とす | `TestTheNaiveVocabularyQuietlyLosesMostOfTheBot` |
| 判断語なら全決定が説明され、反例が1つも出ない | `TestJudgementVocabularyExplainsEveryDecision` |
| 生成エージェントは、**到達可能な123局面すべて**で手書きと一致する | `TestGeneratedAgentPlaysExactlyLikeTheBot` |
| 1/4のコーパスは規則を取りこぼす | `TestASmallCorpusIsMissingRules` |
| 半分のコーパスは規則が揃っていて、なお間違える | `TestAHalfCorpusHasEveryRuleAndStillPlaysWrong` |
| 採掘時の述語と実行時の述語が食い違っていない | `TestMinerAndRuntimeAgree` |
| 生成エージェントは実セッションで同じ試合をする | `TestTheDistilledAgentPlaysTheSameMatches` |
| コミット済みの生成物が古びていない | `TestGeneratedSourcesAreCurrent` |
| curate で減らしたコーパスの採掘を、holdout が数まで一致して裏付ける | `TestCurateThenMineMeasuresTheHoldout` |
| コインは同じ局面で違う手を打ち、curate はそれを一覧にする | `TestTheCoinIsPolicyMixingMadeVisible` |
| `agent_kind: human` の行が curate を通り、`genhuman.You` として生成される | `TestYourCopyMinesFromHumanRows` |
| 写しが黙った局面は代打が指し、その回数が数えられる | `TestUnderstudyAnswersWhereTheCopyIsSilent` |

3つ目が一番強い主張です。コーパスは到達可能局面の一部しか踏まないので、**全数で一致する**のは
被覆率が言っている以上のことです。

6つ目は地味ですが、外すと痛い。述語は2回書かれます——採掘時に記録 JSON を読む `Eval` と、
生成コードが実行時に呼ぶ `GoExpr` です。両者を一致させる仕組みは何もありません。ずれると、
ライブラリは検証を通り、コードはビルドでき、エージェントは**コーパスに存在しない方策**を
指します。このパイプラインで唯一、どこにも痕跡が残らない失敗です。

## 一周させる

このステップの成果物は、コマンド2つで最初から作り直せます。

```bash
cd tutorial/step4-distill && ebigent simulate
```

```bash
cd tutorial/step4-distill && ebigent distill
```

```
simulate: simulation, 800 matches into corpus
2632 decisions from corpus, 19 chips → distill/gen
100.0% of the recorded decisions are explained by an approved chip
```

**前半がコーパスを埋め、後半がそれを Go にします。** 2つは `corpus/` で出会うだけで、互いを
知りません。`simulate` は `kind = "simulation"` のターゲットをビルドして走らせ、局数・シード・
記録先を渡します。`distill` は `behavior.distill` が指すエントリを走らせ、そこがコーパスを読みます。

どちらも掘る処理や遊ぶ処理そのものは持ちません。述語は視界を受け取る Go の関数であって値では
ないので、ゲームをリンクしていないバイナリには受け取りようがない——`ebigent build` が
`go build` を呼ぶのと同じ理由です。

**局数とシードは `ebigent.toml` にあります。**

```toml
[run.episode]
root = "corpus"
matches = 800
seed = 0
```

引数なしの `ebigent simulate` がこの2つの数字で走るので、上の2コマンドが
**コミット済みの `distill/gen` をそのまま再現します**。`RUN_EPISODE_MATCHES=200 ./bin/simulation`
と `ebigent simulate --matches 200` は同じ実行で、渡す道は設定の環境変数層です。

**誰がどの席に座るかは、コマンドの引数ではありません。** ボットが X なのは、step 4 が掘るのが
その判断だからで、コインが O なのは、決定的な相手だと800局が「1局を800回書いたもの」に
なるからです。どちらもゲームの事実なので `cmd/simulation` の中にあります。

古びていないかを見るのはテストの側です。

```bash
cd tutorial/step4-distill && go test ./distill
```

書き出す側と比較する側は**同じ呼び出しを通ります**。コマンドが `Compiled.Write` を呼び、
テストが `Compiled.Sources` を committed と比べる。別々にコーパスを作っていたら、再生成しても
テストが赤いままで、どちらを走らせても閉じない——という状態が起こりえます。

## 同じ局面の800回は、800票です

被覆の先頭2行をもう一度見てください。`preferred_cell_is_4` が 800、`preferred_cell_is_0` が
706。この数字の正体は何でしょうか。X は必ず中央から入るので、**初手の盤面は800局すべてに
現れます**。`SequentialCovering` は1レコードを1票と数えるだけで、同じ局面かどうかは
見ません。つまり先頭の800は「この判断が800回重要だった」ではなく、「この盤面が800回
出てきた」です。被覆の順位は、重要度ではなく出現回数の順位でした。

蒸留の前にコーパスを**整える**——対象を絞り、同じ局面をまとめ、答え合わせ用を取り分ける——
のが `ebigent curate` です。

```bash
cd tutorial/step4-distill && ebigent curate --agent_kind tactic --cap 3 --holdout 20 --seed 1
```

```
curate: 6064 decisions in 800 episodes from corpus
filter: 2632 rows kept (1832 filtered out, 1600 sight-only)
split: 637 train / 163 holdout episodes (holdout 0.20, seed 1)
situations: 65 distinct among 2093 training rows
most repeated situation: 637 rows, 3 kept
cap 3: 185 of 2093 training rows kept (1908 dropped)
conflicts: 0 situations answered with more than one action
curated corpus written: corpus-curated
```

`--agent_kind tactic` はボットの行だけを残します（記録の各行には**誰が**決めたかを示す
`agent_kind` 列があり、人の行なら `human` です）。`--holdout 20` は2割のエピソードを
検証用に取り分け、`--cap 3` は同一の局面・同一の行動を3行までに制限します。残った学習用
2093行の中身は**65局面**でした。最頻の局面は637回——ほぼ全局に現れた、あの初手です。

整えた側を掘ると、こうなります。

```bash
cd tutorial/step4-distill && ebigent distill --corpus corpus-curated/train
```

```
185 decisions from corpus-curated/train, 19 chips → distill/gen
100.0% of the recorded decisions are explained by an approved chip
holdout: 539 decisions answered as recorded, 0 answered differently, 0 silent
silent situations written to corpus-curated/gaps.jsonl
```

2093行が185行になって、出てくるのは**同じ19本のチップ**です。1908行の重複は、規則を
1本も足していませんでした。ただし、並びが変わります。

```
 1  winning_move_is_8      coverage 45
 2  winning_move_is_6      coverage 24
 3  winning_move_is_1      coverage 23
```

`preferred_cell_is_4` は1位から**15位**に落ち、勝ち手が先頭に来ました。800票の正体が
1局面だと分かった途端、「好みが勝ちより上」というあの並びは消えます。あの並びを
決めていたのは、重複だったわけです。どちらの並びも正しい——それは次の数字が言います。

出力の `holdout` 行が、その答え合わせです。採掘が一度も見ていない163局・539決定を
生成された決定リストに指させて、記録と突き合わせる。一致539、違う手0、**沈黙0**。沈黙した
局面があれば `gaps.jsonl` に並び、それが「次の対局で集めるべき局面」の一覧になります。
「コーパスは自分の穴を見られません」の表は、到達可能な123局面を全数で指せるから書けました。
全数走査が現実的でないゲームでは、この3つの数字が表の代わりになります。

もうひとつ、この出力で0だったものがあります。conflicts——同じ局面に違う行動が記録されて
いる数です。ボットは決定論なので0ですが、コインの側を整えると景色が変わります。

```bash
cd tutorial/step4-distill && ebigent curate --agent_kind coin --out corpus-coin
```

```
situations: 19 distinct among 1832 training rows
conflicts: 19 situations answered with more than one action
```

**19局面すべてで手が割れています。** 採掘器は決定論なので、この揺れを個性ではなく反例として
数えます——コインの記録から蒸留すれば、局面ごとの多数派だけが規則になり、少数派は
拒否の山になります。そして人の記録は、コインほどではないにせよ同じ形をしています。
同じ局面で違う手を打つのは、人なら普通のことだからです。curate はこの混在を**解決しません**。
一覧にして `report.json` に置くだけです。プレイヤーで絞るのか、多数派を受け入れるのかは
判断であって、判断は承認と同じ場所——あなたの側にあります。

試したあとは、正典から作り直しておいてください。

```bash
cd tutorial/step4-distill && ebigent distill
```

`--corpus` 付きの蒸留も `distill/gen` に書くので、コミット済みの生成物とはズレた状態に
なっています。引数なしの1コマンドで戻ります——再生成がテストで守られているのは、
こういうときのためです。

## 自分を蒸留する

ここまでの道具は、全部あなたの記録にも向きます。このアプリは人の対局も `corpus/` に
記録していて（`agent_kind: human` の行として、シミュレーションの800局と同じ置き場です）、
curate の `--agent_kind` はまさにその行を取り出すためにあります。

まず、遊びます。何局でも——ただし step 3 の結論を思い出してください。数十局は「少ない」側です。

```bash
cd tutorial/step4-distill && go run .
```

次に、自分の行だけを整えます。出力先を分けるのは、ボット用の `corpus-curated` と
混ざらないようにするためです。

```bash
cd tutorial/step4-distill && ebigent curate --agent_kind human --cap 3 --holdout 20 --seed 1 --out corpus-human
```

報告の読み方はもう知っています。conflicts に並ぶのは、あなたが同じ局面で違う手を打った
回数です。ボットのときは0でしたが、ここでは0にならないはずです——それは記録の不備では
なく、あなたが人だという事実です。

そして蒸留します。`--target you` が、ボットの写し（`distill/gen`）とは別の置き場
`distill/genhuman` に書きます。コミット済みの生成物には触れません。

```bash
cd tutorial/step4-distill && go run ./cmd/distill --corpus corpus-human/train --target you
```

`ebigent distill` を通らず entry を直接呼ぶのは、`--target` がこのゲームの事情
（写しが2種類ある）であって、toolchain が知る話ではないからです。出力の holdout 行と
`corpus-human/gaps.jsonl` の読み方も、前の節のままです。

最後に、自分と打ちます。

```bash
cd tutorial/step4-distill && go run . -opponent you
```

席に着くのは [`distill/genhuman`](distill/genhuman/agent_gen.go) の `You` です。蒸留する前は
**何も知らない placeholder** で、代打（コイン）が全部指します。蒸留したあとは、規則の条件が
当たる局面ではあなたの手を、どの承認チップも当たらない局面では代打を指し、写しが黙った回数を
対局の終わりに端末へ報告します。その回数が、`gaps.jsonl` の生きた姿です。記録の照合表では
ないことに注意してください——写しは記録から掘った**規則**なので、一度も打っていない盤面でも
述語が当たればあなたの手を返します。空の写しに戻すのは `git checkout` です。

ここには、ボットの写しにあった答え合わせがありません。`TestGeneratedAgentPlaysExactlyLikeTheBot`
は教師のソースと突き合わせましたが、**あなたのソースコードは存在しません**。写しの確からしさを
言えるのは holdout の3つの数字だけで、その数字を良くする方法は記録を増やすことだけです。
そして人が800局打つのは——現実的ではありません。それが次のステップです。

## 構成

```
step4-distill/
├── cmd/simulation/  ebigent simulate が走らせるエントリ。800局を corpus/ に
├── cmd/distill/     ebigent distill が走らせるエントリ。corpus/ を掘る
├── main.go            ウィンドウ。生成エージェントを席に着けるのはここ
├── game/              ルール。step 3 から1文字も変えていない
├── msg/               ワイヤ型と生成物（step 2 と同一）
└── distill/
    ├── distill.go     語彙2つ、コーパス、採掘、playtest
    ├── pred/pred.go   判断語。人が書き、人がレビューする
    ├── distill_test.go
    ├── gen/           生成物（コミット済み）
    │   ├── agent_gen.go      決定リスト
    │   ├── agent_gen_test.go 記録された局面からの fixture
    │   └── chips.json        承認済みライブラリ
    └── genhuman/      あなたの写しの置き場。蒸留するまでは空の placeholder
```

`game` が `distill/gen` を import していないことに注意してください。生成エージェントは
`Sight` を判定する述語を呼ぶので、`gen` は `game` に依存します。ルール側がそれを名指すと
**自分の記録から蒸留されたものに依存する**ことになり、コンパイラが import cycle として
拒否します。だから席の選択は entry point に移りました——トランスポートと同じ理由、同じ場所です。

## まだないもの

**写しのレベルは、記録の相手で頭打ちになります。** あなたの記録は、勝ち・ブロック・好みの
三手しか知らないボットの写し（と、その代打のコイン）との対局から来ています。相手が
仕掛けてこなければ、追い詰められた局面のあなたの判断は**記録に現れようがなく**、
`gaps.jsonl` はそれを不足として数えるだけです。学習データの質を上げるには、
**もう一周**が要ります。蒸留した写しを相手に据えて（`-opponent you`）遊び直せば、次の
記録は一段違う相手との対局になり、再 curate・再蒸留で写しが厚くなる。人が相手なら、
このアプリは [step 2](../step2-lobby/) と同じ LAN マッチングを積んでいるので、もう一人に
接続してもらえば**人対人の記録**が同じコーパスに貯まります。どの道も
「相手を替えて、記録に現れる局面を変える」ことで——その一周を人手ではなく機械で回し、
効果を測るのが [step 5](../step5-simulate/) です。`gaps.jsonl` は、そこへ持っていく
買い物リストになります。
