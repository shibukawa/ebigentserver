# What a good audit of step4.mdx produces

Ordered by what the reader loses, not by how easy each is to fix.

Must find, in the top group:

- **Terms used before they are defined.** 語彙, 述語, 採掘, チップ, 被覆, 反例 all
  land in the first two screens. The page's own description line uses 採掘器 and
  決定リスト. A reader arriving from step 3 has met none of them. The fix is a
  clause at first use plus a link to `/architecture/terms/#記録と方策の語` — not a
  glossary section on the page.

Should find:

- Numbers in the prose (59.2%, 19 chips, 123 positions) are quoted from test
  output, so an audit should say whether they still reproduce rather than
  assuming they do.
- Every 確かめ方 row names a test; `--only=refs` verifies those exist.

Should NOT report:

- The measurement tables as "should be prose". They are coordinate data and the
  table is the right shape.
- The 「まだないもの」 section as missing — it is present.

A good audit says plainly which parts are fine. An audit of this page that
produces only vague prose complaints has failed.
