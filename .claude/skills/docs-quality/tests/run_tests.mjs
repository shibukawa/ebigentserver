#!/usr/bin/env node
// Self-test for check_docs.mjs.
//
//   node .claude/skills/docs-quality/tests/run_tests.mjs
//
// Two halves. The fixture under tests/fixture/ is a miniature site with one
// planted defect per check, and every check has to find its own — plus a clean
// page beside them that every check has to stay quiet on, which is the half
// that catches a rule written too broadly.
//
// The real site is then swept for errors. That guards the part of the checker
// most likely to rot silently: the slugger has to agree with github-slugger,
// and a disagreement would turn every working anchor into a false alarm.

import { execFileSync } from 'node:child_process';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = dirname(fileURLToPath(import.meta.url));
const SKILL = join(HERE, '..');
const REPO = join(SKILL, '..', '..', '..');
const CHECKER = join(SKILL, 'check_docs.mjs');
const FIXTURE = join(HERE, 'fixture');

function run(root) {
  const opts = {
    cwd: REPO,
    encoding: 'utf8',
    env: { ...process.env, EBIGENT_REPO_ROOT: root },
    stdio: ['ignore', 'pipe', 'inherit'],
  };
  try {
    return JSON.parse(execFileSync('node', [CHECKER, '--json'], opts)).findings;
  } catch (e) {
    // A non-empty error list exits 1, which is the expected case for the fixture.
    if (e.stdout) return JSON.parse(e.stdout).findings;
    throw e;
  }
}

// check, severity, a substring of the message, and the file it must land on.
const EXPECTED = [
  ['links', 'error', 'dead link', 'tutorial/step2.mdx'],
  ['links', 'error', 'dead anchor', 'tutorial/step2.mdx'],
  ['frontmatter', 'error', 'no `description`', 'tutorial/step2.mdx'],
  ['frontmatter', 'warn', 'description is', 'overview.mdx'],
  ['sidebar', 'error', 'which has no page', 'astro.config.mjs'],
  ['concepts', 'error', 'is not in .knowledge', 'tutorial/step2.mdx'],
  ['refs', 'error', 'is not a test in this repository', 'tutorial/step2.mdx'],
  ['refs', 'error', 'does not exist', 'tutorial/step2.mdx'],
  ['mirror', 'error', 'no website page for this step', 'tutorial/step3-orphan'],
  ['mirror', 'error', 'no tutorial/step2-* directory', 'tutorial/step2.mdx'],
  ['terms', 'error', 'corpus', 'tutorial/step2.mdx'],
  ['terms', 'error', '観測', 'tutorial/step2.mdx'],
  ['terms', 'error', 'ゲームロジックを実装', 'tutorial/step2.mdx'],
  ['terms', 'error', 'AIモード', 'tutorial/step2.mdx'],
  ['shape', 'warn', 'bullets or table rows', 'tutorial/step2.mdx'],
  ['shape', 'info', 'where this page stops', 'tutorial/step2.mdx'],
];

// The page every check has to leave alone.
const CLEAN = 'tutorial/step1.mdx';

let failures = 0;
const fail = (msg) => { console.log(`  ✗ ${msg}`); failures++; };

console.log('fixture');
const found = run(FIXTURE);
for (const [check, severity, needle, file] of EXPECTED) {
  const hit = found.find(
    (f) => f.check === check && f.severity === severity &&
           f.message.includes(needle) && f.file.includes(file),
  );
  if (hit) console.log(`  ✓ ${check}/${severity} ${needle}`);
  else fail(`${check}/${severity} "${needle}" was not reported on ${file}`);
}

const noise = found.filter((f) => f.file.includes(CLEAN));
if (noise.length === 0) console.log(`  ✓ ${CLEAN} is quiet`);
else for (const f of noise) fail(`${CLEAN} should be quiet, got ${f.check}: ${f.message}`);

const extra = found.length - EXPECTED.length;
if (extra > 0) {
  console.log(`  · ${extra} finding(s) beyond the table — new checks need a line in EXPECTED`);
}

console.log('\nreal site');
const real = run(REPO);
const errors = real.filter((f) => f.severity === 'error');
if (errors.length === 0) console.log('  ✓ no errors');
else for (const f of errors) fail(`${f.file}:${f.line} ${f.check}: ${f.message}`);

console.log(failures === 0 ? '\nall good' : `\n${failures} failure(s)`);
process.exit(failures === 0 ? 0 : 1);
