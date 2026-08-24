# What each directory owes the reader

Density is not a site-wide constant. Holding a reference page to a guide's
prose ratio, or a tutorial step to a reference page's completeness, is the most
common way to make this site worse.

## `tutorial/`

A narrative read in order, one step per page, each mirrored by
`tutorial/stepN-*/README.md` beside the code.

A step opens on the problem the previous one left, names its change in a
sentence, and closes on where it stops. The middle is the part worth writing:
what the reader could not have predicted, backed by output that was measured
rather than described.

Prose-forward. Tables belong in 確かめ方 (claims to tests) and in measurement
grids, where the rows genuinely are coordinate.

A step may assume every step before it and nothing after.

## `integration/`

Reference for the four seams a game touches: `StageRuleSet`, the agent
interface, 視界, and the boundaries that must not be crossed. Correctly
table-forward and correctly exhaustive — this is where a reader comes to check
a signature.

Do not rewrite these into narrative. Do keep the "why" line under each table:
a signature without its reason is a thing to copy, not to understand.

## `connection/`

How instances reach each other: transports, signalling, synchronisation,
deployment combinations. Configuration-surface pages. A matrix here is the
right shape, and the accompanying prose says which row to take by default and
what changes the answer.

## `ai/`

The distillation pipeline end to end. Explanatory rather than reference: these
pages carry the argument for why the pipeline has a human in it, so they are
prose-forward with diagrams.

They are also where the tutorial sends a reader who wants the full version, so
they must stand alone — a reader may arrive here without having played
tic-tac-toe.

## `architecture/terms.mdx`

A glossary. Lookup tables are right. Each entry still says why the distinction
matters rather than only what the word means, because a glossary that only
renames things teaches nothing.

## `index.mdx`, `overview.mdx`

Entry points. Their job is to route, and their failure mode is using the site's
vocabulary before any of it is defined.
