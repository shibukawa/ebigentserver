# Eval cases

Two cases, one per mode, each a page-sized task with the properties a good
answer has to have. They are graded by reading, not by a script — the mechanical
half of this skill is already covered by `tests/run_tests.mjs`, and what is left
is judgement.

Run a case by giving the input file to a fresh session with this skill
available, then compare the result against the expectations file. The
expectations are properties, not a target text: two good answers will differ.
