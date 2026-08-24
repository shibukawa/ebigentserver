# What a good answer looks like

The page it belongs on:

- The behaviour is already described in `tutorial/step3.mdx` and
  `tutorial/step3-record/README.md`. A good answer notices the dual surface and
  updates both, rather than adding a third home for the same fact.

Properties of the prose:

- Opens on the reader's situation — playing again tomorrow — not on the name
  `ResumeIndex`.
- States the *reason* the index matters, which is that it carries the seed. A
  version that only says "files are not overwritten" has dropped the half that
  makes the design non-obvious.
- Names the boundary: `run.Serve` deliberately does not resume, because a
  headless batch is a reproducible unit.
- Japanese register matches the file — だ/である on the website page, です/ます in
  the README.

Should NOT:

- Add a table for two cases.
- Enumerate every field of `RecordOptions`; that is reference material.
- Introduce a new term for the concept when コーパス and エピソード already exist.
