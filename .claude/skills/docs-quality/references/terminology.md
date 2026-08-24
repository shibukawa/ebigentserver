# Fixed terminology

Each entry names a real distinction. The column that matters is the last one:
what the wrong word tells the reader that is false.

## The four layers

| Word | What it names | What the wrong word says |
|---|---|---|
| **Protocol** | The game's own terms: how many seats, how they connect, how urgent a frame is, whether everyone reads one screen. Settled at build. | Calling it "設定" suggests it can differ between two launches of one artifact. It cannot — that is what makes it a build fact. |
| **Match** | One gathering-to-ending. Roster, seed, links. | "セッション" is the runtime object inside a match, not the match. |
| **Stage** | One rule set. What the wider industry calls a scene. | "シーン" is the display state in front of a stage; the lobby is a screen with no rules and one stage may show several. |
| **Message** | One frame on the line. | "ワイヤ" was a CBOR profile name and the profiles are gone; shapes are 配列形状 / マップ形状 now. |
| **Schema** | The shape of a Stage's messages. Not a layer — a Stage has one. | Confusing it with Protocol makes a stage swap look like a protocol change. |

## The three types a rule set names

| Word | What it names | What the wrong word says |
|---|---|---|
| **World** | The authoritative state. | "State" was step 1's word, when the same value was truth, picture and evidence at once. Over a link those are three things in three places. |
| **Action** | One seat's one decision. The only thing that travels upstream. | — |
| **視界 (Sight)** | What one seat is allowed to see. | **観測** reads as a measurement the agent performs. A sight is handed to it by the session; the agent never reaches for one. |

`StageRuleSet` is the interface a game implements — six methods. Writing
"ゲームロジックを実装する" hides that it is one declared seam, and invites the
reader to imagine scattering rules through their own update loop.

## The recording and distillation vocabulary

Defined at `/architecture/terms/#記録と方策の語`. A page using one of these for
the first time in reading order owes the reader a clause.

| Word | One clause |
|---|---|
| **決定 (decision)** | A sight paired with the action taken from it. Not the move alone. |
| **エピソード (episode)** | One match's record; a directory of JSONL. |
| **コーパス (corpus)** | Episodes collected. Analysis and distillation run over one. |
| **述語 (predicate)** | A named true/false judgement about a sight. |
| **語彙 (vocabulary)** | The predicates and actions a rule may use. |
| **採掘 (mining)** | Finding condition→action pairs in a corpus. |
| **被覆 / 反例** | How many records supported a rule; how many contradicted it. |
| **チップ (chip)** | One approved rule. |
| **決定リスト (decision list)** | Chips read top down, first match wins. |
| **蒸留 (distillation)** | The whole path: a slow expensive teacher becomes a fast readable student. |
| **教師 / 生徒** | Who produced the recording; what was generated from it. |

`corpus` in Latin script is the directory name and the Go identifier. In prose
the word is コーパス — except as a gloss where the term is defined, and as a
path.

## Concept ids

`.knowledge` is the design catalogue and prose cites it by id: `concept:sight`,
`rule:shared-rng-seed`, `data:episode-log`. These are checkable citations, not
decoration — `--only=concepts` resolves every one.

Two that are easy to get wrong:

- **`data:game-version`**, not `data:protocol-version`. The catalogue entry says
  outright that calling it a protocol version borrows HTTP/1.1's vocabulary for
  an ordered negotiated revision, which is the opposite of a fingerprint compared
  once for equality.
- **`decision:no-ai-game-mode`** is why there is no "AI モード" or "ボットモード"
  to write about. The rules never learn who occupies a seat, so there is no
  switch — a sentence implying one contradicts the framework's central claim.
