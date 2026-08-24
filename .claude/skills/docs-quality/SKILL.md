---
name: docs-quality
description: Write, rewrite, review, or audit any page of the ebigentserver documentation — the Astro Starlight site under website/src/content/docs/ and the tutorial READMEs under tutorial/ that mirror it. Use this whenever documentation is created or changed ("write the docs", "add a page", "document this", "update the docs", "review this page", "the docs are stale", "用語を追加して", "読みにくい"), and ALSO immediately after changing framework behaviour that a page describes, even when the request only says "and update the docs too" — this repository's pages quote measured output and real test names, so a code change usually invalidates prose somewhere. Carries the house style (Japanese prose over bullets, recommend rather than survey, every page says where it stops, terms defined at first use), the fixed vocabulary (Protocol/Match/Stage/Message, 視界, コーパス, 述語, チップ), and check_docs.mjs for dead links and anchors, frontmatter, sidebar drift, stale `.knowledge` concept ids, and test names and paths the prose cites that no longer exist.
---

# ebigentserver documentation

The site is Astro Starlight, Japanese only, under `website/src/content/docs/`.
There is no locale subtree and no English tree — Japanese is the root, so a
page has exactly one home. The sidebar is an explicit ordered array in
`website/astro.config.mjs`; a new page is invisible until a line is added
there, which is the price of having reading order rather than alphabetical
order.

Paths below are relative to the repository root.

Two things carry this skill. `check_docs.mjs` decides everything a machine can
decide, in under a second over the whole site. The rest of this file covers
what it cannot: whether a page is worth reading.

**Start with the script, in either mode.**

```bash
node .claude/skills/docs-quality/check_docs.mjs
```

Scope it while you work, and re-run it unscoped before you finish:

```bash
node .claude/skills/docs-quality/check_docs.mjs --path=tutorial
```

Exit status is 1 when any `error` finding is present. `--json` gives structured
output; `--only=links,concepts` restricts to named checks (`links`,
`frontmatter`, `sidebar`, `concepts`, `refs`, `mirror`, `terms`, `shape`).

The checker's own tests confirm it still works after you edit it:

```bash
node .claude/skills/docs-quality/tests/run_tests.mjs
```

## The property that makes this site different

**The pages quote measured output.** Coverage percentages, chip counts, tick
timings, the exact board a test reports. That is why the tutorial is convincing
and it is also the thing that rots first.

So: never retype a number, and never adjust one to match a claim. Run the
command, copy what it prints. If a number moved, the sentence around it is part
of the change — a page saying "800局でようやく正しい並びになる" is wrong the
moment the seeding changes, and no build will notice.

The same applies to test names. A page that says a claim is fixed by
`TestForkWordsTradeSilenceForConfidence` is making a checkable promise; `--only=refs`
verifies the function exists, and it is the only check that catches a rename.

## Two surfaces, one story

Each tutorial step exists twice: `tutorial/stepN-*/README.md` beside the code,
and `website/src/content/docs/tutorial/stepN.mdx` on the site. A reader arriving
from GitHub sees the first, one arriving from the site sees the second.

They are not translations of each other and should not be kept in sync
sentence by sentence. The README is for somebody looking at the directory —
shorter, more file-oriented. The page is for somebody reading in order — it can
assume the previous step and spend more on why. What has to match is the
**claims**: the same numbers, the same test names, the same boundary.

`--only=mirror` reports a step that has one surface and not the other.

## Mode 1 — writing or rewriting a page

### Decide what kind of page this is

`references/page-types.md` settles this per directory. Briefly:
`tutorial/` is a narrative in order; `integration/` and `connection/` are
reference-shaped and correctly table-forward; `ai/` explains a pipeline;
`architecture/terms.mdx` is a glossary and a lookup table is right there.

Holding all of them to one density is the most common way to make this site
worse.

### The shape of a tutorial step

The existing steps follow this, and they follow it because each part earns its
place.

1. **The command, first screen.** The reader can run something before they
   understand anything.
2. **What went wrong in the previous step.** Every step opens on a problem the
   last one left, not on the name of a feature.
3. **The change, as a diff-sized thing.** "Two fields below are the entire
   change." A step that cannot name its change in a sentence is two steps.
4. **The finding.** What the reader could not have predicted. This is the part
   worth writing, and the measured output belongs here.
5. **確かめ方** — the claims table, each row naming the test that fixes it.
6. **まだないもの** — where this step stops and which step picks it up.

### Prose carries the reasoning

A bullet list and a table both drop the connectives, and the connectives are
usually the content. When you write a row, ask whether it still says *because*.
If not, it goes back into a sentence.

This is not a ban on tables. A table is right when the items are genuinely
coordinate — a measurement grid, a term lookup, a claims-to-tests mapping. It is
wrong when it flattens an argument into cells and leaves the reader to
reconstruct why any row is true. `--only=shape` flags a page that is more than
45% list lines; treat it as a question, not a verdict.

### Define a term where the reader first meets it

This is the failure mode this site is most prone to, because the vocabulary is
large and interlocking: コーパス, エピソード, 述語, 語彙, チップ, 決定リスト,
被覆, 反例, 蒸留, 教師/生徒.

A term gets one clause of explanation at its first appearance in the reading
order, and the full entry lives in `/architecture/terms/#記録と方策の語`. One
clause, not a paragraph — 「**コーパス**——後から読み返せる対局記録の集まり——になる」
carries the reader without stopping them.

Check first use in reading order, not per page: the tutorial index is read
before step 3, so a word used in the index table is already a first use.

### Recommend

Where the reader chooses, say which one they should take and why, then name the
condition that changes the answer. 「どちらでもよい」 is not a sentence this site
publishes. The reader came for the judgement the author already made.

### Say where the page stops

Every page states its own boundary. On a tutorial step that is the まだないもの
section; elsewhere it is often a sentence in the opening. `--only=shape` reports
pages where it finds no such wording — a reading list, not a verdict, since the
boundary is frequently there in words the pattern missed.

### Terminology is fixed

`references/terminology.md` has the table and, for each entry, what the wrong
word tells the reader that is false. The short version: Protocol / Match /
Stage / Message are the four layers; a projection is 視界, never 観測; the rules
a game implements are a `StageRuleSet`; what travels is a Message whose shape is
the Stage's Schema.

Concept ids from `.knowledge` (`concept:sight`, `rule:shared-rng-seed`) are
citations, and `--only=concepts` resolves every one against the catalogue. A
renamed concept leaves prose pointing at nothing — that is how
`data:protocol-version` survived in `connection/signaling.mdx` after the concept
became `data:game-version`, whose own entry says calling it a protocol version
borrows HTTP's vocabulary for the opposite of what it is.

### Writing Japanese

Load the `japanese-cognitive-rhythm-writing` skill. The register differs by
surface and both are established: the website pages are だ/である, the repository
READMEs are です/ます. Match the file you are in.

### Before you call it done

```bash
node .claude/skills/docs-quality/check_docs.mjs
cd website && npx astro build
```

The build is the only check on MDX that Starlight itself rejects, and on
sidebar slugs — an explicit entry pointing at a missing page fails with
`AstroUserError: The slug ... does not exist`.

## Mode 2 — auditing existing pages

Run the checker, read the pages, then report findings **ordered by what a reader
loses**, not by how easy they are to fix.

1. **Wrong.** The page contradicts the code: a number that no longer reproduces,
   a test that was renamed, an example that does not compile. A reader who
   trusts it loses an afternoon.
2. **Broken.** Dead link, dead anchor, stale concept id, a step with only one of
   its two surfaces. Mechanical; the checker has already found these.
3. **Unusable.** A term used before it is defined, an example that is a fragment,
   no recommendation among options, no statement of where the page stops. The
   information is present and the reader still cannot act.
4. **Degraded.** A table where an argument belonged, drifted terminology, a
   missing prerequisite line.
5. **Rough.** Density, cadence, an unlanded abstraction.

For each finding give the file and line, what the reader loses, and the specific
fix. "Improve the prose" is not a finding. "コーパス is used in the step 2
hand-off and first defined in step 3, so the reader meets the word at the moment
they are deciding whether to continue" is.

Two honest verdicts are worth stating when they apply, because an auditor under
pressure to produce findings will otherwise invent work:

- **This page is fine as it is.** Reference tables usually are.
- **This page is the wrong shape entirely** and needs rewriting rather than a
  list of patches.

`references/exemplars.md` names the pages to compare against and says what each
one is doing well.

## What the checker looks at

`links` resolves every root-absolute Markdown link against the real route map,
including `#anchors`, and reports a route deleted in git history so a rename
names its replacement.

`frontmatter` requires `title` and `description` and flags a description too
short to say what the page settles or long enough to be the page.

`sidebar` compares the explicit array in `astro.config.mjs` against the
filesystem in both directions. The array is bounded before scanning, so sibling
config — `customCss`, `social`, `locales` — is not mistaken for entries.

`concepts` resolves every `type:name` citation against `.knowledge/`, and
suggests a near match when the concept was renamed.

`refs` verifies that every `` `TestXxx` `` the prose cites exists in a
`*_test.go`, and that every repository path written as inline code exists. Test
names are collected by walking the tree rather than by `git grep`, because a
step being written is untracked and its tests are exactly the ones the new page
cites.

`mirror` pairs `tutorial/stepN-*/README.md` with
`website/src/content/docs/tutorial/stepN.mdx` in both directions.

`terms` and `shape` are described above. Both are wording-sensitive by design;
`shape` is `info` and never fails the build.

## Gotchas

**Anchors are github-slugger's, and Japanese survives intact.** `## 記録と方策の語`
anchors as `#記録と方策の語`. Punctuation is deleted rather than replaced and each
surviving space becomes its own dash, so `cookie — no storage` anchors as
`cookie--no-storage` with two dashes. Inline code in a heading contributes its
text.

**`--path` filters the pages reported, not the route map.** Links are still
resolved against the whole site, which is what you want.

**The `terms` check counts lines against the raw file.** `stripCode` blanks code
with spaces of equal length so offsets stay aligned; if you add a masking step,
keep the length.

**`<FileTree>` content is not code.** It is scanned like prose, which is right —
its entries carry descriptions — so a path inside one needs the same exemptions
a path in a sentence does.

## Extending the checks

A new rule needs three edits, and the third keeps it alive: the pattern in
`check_docs.mjs`, a planted defect in `tests/fixture/`, and a line in the
`EXPECTED` table of `tests/run_tests.mjs`. The fixture is a miniature site with
one deliberate fault per check, so `run_tests.mjs` proves both that each check
fires and that the clean pages beside them stay quiet.
