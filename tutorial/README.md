# チュートリアル — ボードゲーム編

三目並べを、フレームワークなしの Ebitengine アプリから、記録して学習して育てるところまで
段階的に育てる。各ステップは独立したディレクトリで、前のステップを壊さずに残す。

`samples/tictactoe/` とは別物であることに注意。あちらは framework の回帰ハーネス
（`decision:samples-as-test-infrastructure`）で、意図的に端末ベースのまま置いてある。
こちらは読み進めるためのもの。

## ステップ

| ステップ | 内容 | 状態 |
|---|---|---|
| [step1-hotseat](step1-hotseat/) | マウス1つで交互に指す。framework を一切使わない | 済 |
| [step2-lobby](step2-lobby/) | ルールを `StageRuleSet` に。ロビーが相手を待ち、LAN 上の2インスタンスで対人戦 | 済 |
| [step3-record](step3-record/) | `if` 4本の CPU を人間と同じ席に着ける。その判断が記録に残る | 済 |
| [step4-distill](step4-distill/) | 記録から決定リストを蒸留し、人の指し手から自分の写しを作って対戦する | 済 |
| [step5-simulate](step5-simulate/) | 相手を回して corpus を広げる。そして完全な教師の限界を測る | 済 |

ここで**ボードゲーム編は完結**する。

三目並べは解けているゲームなので、最強の相手が存在し、その相手は同値な最適手の中から
理由なく1つを選ぶ。step5 が測っているのはその限界で、**蒸留一般の限界ではない**。
アクションゲームの敵のように「開発者が操作して動きを付け、それを再生する」使い方は
問題の形がまるで違うので、別のチュートリアルで扱う。

## 進め方

各ステップの `README.md` に、そのステップで何が増えたか、そして次に何が引っかかるかを書く。
ステップ間の差分がそのまま教材になるので、コードは前のステップからコピーして育てる。

各ステップは**それ自身の Go モジュール**でもある。step2 は `ebigent init` を実際に走らせた
結果を持っていて、`init` は `go.mod` があるかどうかで仕事を変えるので、読者が走らせるのと
同じものをここで走らせるには `go.mod` が要った。ルートの `go.work` が5つをまとめているため、
`go run ./tutorial/...` はこれまでどおり動く。各ステップの `replace` はこのチェックアウトを
指すので、リリース版ではなく手元のフレームワークを試すことになる。

```bash
go run ./tutorial/step1-hotseat
```

```bash
go run ./tutorial/step2-lobby
```

step2 は2つ起動する。片方がホストになり、もう片方が空席に着く。

```bash
go run ./tutorial/step3-record
```

step3 は1つでよい。待っている間にクリックすると、空席にボットが座る。対局は `./corpus` に残る。

```bash
go run ./tutorial/step4-distill
```

step4 も1つでよい。相手をするのは、step3 の記録から生成されたボットになる。

```bash
go run ./tutorial/step5-simulate
```

step5 は step4 と同じように遊べる。違いは corpus の作り方と、そこで測ったことにある。
