#!/usr/bin/env node
// Mechanical checks for the ebigentserver documentation site.
//
// Run from the repository root:
//   node .claude/skills/docs-quality/check_docs.mjs
//   node .claude/skills/docs-quality/check_docs.mjs --only=links,concepts
//   node .claude/skills/docs-quality/check_docs.mjs --path=tutorial
//   node .claude/skills/docs-quality/check_docs.mjs --json
//
// Everything here is decidable by a machine. Judgement calls — whether a page
// earns its third example, whether a table dropped a "because" — belong to
// SKILL.md, not to this file.
//
// The site is Japanese-only and has no locale subtree, so there is no parity
// check between languages. The parity that does exist here is between the
// website page and the repository README that covers the same tutorial step,
// and between the docs and the code they quote.

import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { join, relative, dirname, basename, extname } from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = dirname(fileURLToPath(import.meta.url));
const ROOT = process.env.EBIGENT_REPO_ROOT || join(HERE, '..', '..', '..');
const DOCS = join(ROOT, 'website', 'src', 'content', 'docs');
const ASTRO_CONFIG = join(ROOT, 'website', 'astro.config.mjs');
const KNOWLEDGE = join(ROOT, '.knowledge');
const TUTORIAL_DIR = join(ROOT, 'tutorial');

// ---------------------------------------------------------------------------
// Tunables. The only per-project state; extend as the site grows.
// ---------------------------------------------------------------------------

// The `.knowledge` catalogue's id namespaces. A doc citing `concept:sight`
// promises `.knowledge/concept/sight.md` exists; when a concept is renamed the
// prose is what goes stale, and nothing else notices.
const CONCEPT_TYPES = [
  'actor', 'api', 'concept', 'data', 'decision', 'flow', 'metric',
  'permission', 'policy', 'requirement', 'rule', 'sample', 'system',
  'term', 'ui', 'vision',
];

// Words this project has settled on. The wrong one is not a style slip: each
// says something false about how the framework is built.
const BANNED_TERMS = [
  {
    // Exempts the two places the Latin word is right: `corpus/` as a path, and
    // `（corpus）` as the gloss beside the Japanese term where it is defined.
    re: /(?<![（(])\bcorpus\b(?![)）/])/gi,
    say: 'the docs say コーパス in prose — `corpus` in Latin script is the directory name and the Go identifier, not the word for the concept',
  },
  {
    re: /観測(?!者)/g,
    say: '`Sight` is 視界 throughout. 観測 reads as a measurement the agent performs, when a sight is handed to it by the session',
  },
  {
    re: /ゲームロジック(?:を|は|が)?(?:実装|書)/g,
    say: 'the rules a game implements are a `StageRuleSet` — naming it "ゲームロジック" hides that it is one declared interface with six methods',
  },
  {
    re: /\bワイヤ形式\b|\bワイヤプロファイル\b/g,
    say: 'what travels is a Message and its shape is the Stage\'s Schema; the CBOR profiles that "wire" named were replaced by 配列形状 / マップ形状',
  },
  {
    re: /(?:AI|ボット)モード/g,
    say: 'decision:no-ai-game-mode — the rules never learn who occupies a seat, so there is no mode to switch',
  },
];

const SEVERITY_ORDER = { error: 0, warn: 1, info: 2 };

// ---------------------------------------------------------------------------
// CLI
// ---------------------------------------------------------------------------

const args = process.argv.slice(2);
const argOf = (name) => {
  const hit = args.find((a) => a.startsWith(`--${name}=`));
  return hit ? hit.slice(name.length + 3) : null;
};
const asJson = args.includes('--json');
const only = argOf('only')?.split(',').map((s) => s.trim()).filter(Boolean) ?? null;
const pathFilter = argOf('path');
const wants = (check) => !only || only.includes(check);
// ---------------------------------------------------------------------------
// Loading
// ---------------------------------------------------------------------------

function walk(dir) {
  const out = [];
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) out.push(...walk(full));
    else if (['.md', '.mdx'].includes(extname(name))) out.push(full);
  }
  return out;
}

function splitFrontmatter(raw) {
  if (!raw.startsWith('---')) return { fm: {}, body: raw, bodyOffset: 0, fmRaw: '' };
  const end = raw.indexOf('\n---', 3);
  if (end === -1) return { fm: {}, body: raw, bodyOffset: 0, fmRaw: '' };
  const fmRaw = raw.slice(4, end);
  const body = raw.slice(raw.indexOf('\n', end + 1) + 1);
  return { fm: parseSimpleYaml(fmRaw), body, bodyOffset: raw.slice(0, end).split('\n').length + 1, fmRaw };
}

// Enough YAML for Starlight frontmatter: scalars, one nesting level, sequences
// of scalars. No anchors, no flow maps beyond `{}`.
function parseSimpleYaml(text) {
  const root = {};
  const stack = [{ indent: -1, node: root }];
  for (const line of text.split('\n')) {
    if (!line.trim() || line.trim().startsWith('#')) continue;
    const indent = line.length - line.trimStart().length;
    while (stack.length > 1 && indent <= stack[stack.length - 1].indent) stack.pop();
    const parent = stack[stack.length - 1].node;
    const trimmed = line.trim();
    if (trimmed.startsWith('- ')) {
      const key = stack[stack.length - 1].lastKey;
      if (key) {
        if (!Array.isArray(parent[key])) parent[key] = [];
        parent[key].push(unquote(trimmed.slice(2)));
      }
      continue;
    }
    const m = trimmed.match(/^([\w.-]+):\s*(.*)$/);
    if (!m) continue;
    const [, key, rest] = m;
    if (rest === '') {
      const child = {};
      parent[key] = child;
      stack[stack.length - 1].lastKey = key;
      stack.push({ indent, node: child });
    } else {
      parent[key] = unquote(rest);
      stack[stack.length - 1].lastKey = key;
    }
  }
  return root;
}

const unquote = (s) => s.replace(/^['"]|['"]$/g, '').trim();

function routeOf(file) {
  let rel = relative(DOCS, file).replace(/\\/g, '/').replace(/\.mdx?$/, '');
  if (basename(rel) === 'index') rel = dirname(rel) === '.' ? '' : dirname(rel);
  return rel === '' ? '/' : `/${rel}/`;
}

// Fenced code, inline code and HTML comments are invisible to the prose checks.
function stripCode(body) {
  return body
    .replace(/^```[\s\S]*?^```/gm, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/`[^`\n]*`/g, (m) => ' '.repeat(m.length))
    .replace(/<!--[\s\S]*?-->/g, (m) => m.replace(/[^\n]/g, ' '));
}

function fencedBlocks(body) {
  const blocks = [];
  const re = /^```([^\n]*)\n([\s\S]*?)^```/gm;
  let m;
  while ((m = re.exec(body)) !== null) {
    blocks.push({
      meta: m[1].trim(),
      code: m[2],
      line: body.slice(0, m.index).split('\n').length,
    });
  }
  return blocks;
}

// Mirrors github-slugger, which is what Starlight's heading ids come from.
// Two details matter and are easy to get wrong: punctuation is deleted rather
// than replaced, and each remaining space becomes its own dash — so a heading
// written `cookie — no storage at all` anchors as `cookie--no-storage-at-all`.
// A third detail decides the two emphasis markers, and they part company: `*` is
// removed and `_` is kept, everywhere and regardless of code spans. The library
// removes a punctuation set rather than parsing emphasis, and `_` is a word
// character that never made that set. So `**b** _i_` anchors as `b-_i_`, and a
// heading named for a key — `### \`_operation\`: what it does` — anchors as
// `_operation-what-it-does` with the underscore intact.
function slugify(text) {
  return text
    .replace(/`([^`]*)`/g, '$1')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\p{M}_ -]/gu, '')
    .replace(/ /g, '-');
}

// Only fenced blocks are masked here. Inline code inside a heading is part of
// its text, and blanking it would invent anchors nobody wrote.
function headingsOf(body) {
  const seen = new Map();
  const out = [];
  const masked = body.replace(/^```[\s\S]*?^```/gm, (m) => m.replace(/[^\n]/g, ' '));
  for (const line of masked.split('\n')) {
    const m = line.match(/^(#{2,6})\s+(.*)$/);
    if (!m) continue;
    let s = slugify(m[2]);
    if (seen.has(s)) {
      const n = seen.get(s) + 1;
      seen.set(s, n);
      s = `${s}-${n}`;
    } else seen.set(s, 0);
    out.push({ text: m[2].trim(), slug: s, level: m[1].length });
  }
  return out;
}

const pages = walk(DOCS)
  .sort()
  .map((file) => {
    const raw = readFileSync(file, 'utf8');
    const { fm, body, fmRaw } = splitFrontmatter(raw);
    const rel = relative(ROOT, file).replace(/\\/g, '/');
    return {
      file,
      rel,
      raw,
      fm,
      fmRaw,
      body,
      route: routeOf(file),
      locale: routeOf(file).startsWith('/ja/') ? 'ja' : 'en',
      headings: headingsOf(body),
    };
  });

const byRoute = new Map(pages.map((p) => [p.route, p]));
const selected = pathFilter
  ? pages.filter((p) => p.rel.includes(pathFilter) || p.route.includes(pathFilter))
  : pages;// ---------------------------------------------------------------------------
// Findings
// ---------------------------------------------------------------------------

const findings = [];
const report = (check, severity, page, line, message, hint) =>
  findings.push({ check, severity, file: page.rel, line, message, hint });// --- links -----------------------------------------------------------------

// Routes deleted by past reorganisations, read out of git so a rename reports
// where the page went instead of just "not found".
function removedRoutes() {
  const map = new Map();
  try {
    const out = execFileSync(
      'git',
      ['log', '--diff-filter=D', '--name-only', '--pretty=format:', '--', 'website/src/content/docs'],
      { cwd: ROOT, encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] },
    );
    for (const line of out.split('\n')) {
      const p = line.trim();
      if (!p.startsWith('website/src/content/docs/')) continue;
      let rel = p.replace('website/src/content/docs/', '').replace(/\.mdx?$/, '');
      if (basename(rel) === 'index') rel = dirname(rel) === '.' ? '' : dirname(rel);
      const route = rel === '' ? '/' : `/${rel}/`;
      if (!byRoute.has(route)) map.set(route, null);
    }
  } catch {
    /* shallow clone or no git: the route map alone still catches dead links */
  }
  return map;
}

if (wants('links')) {
  const gone = removedRoutes();
  for (const page of selected) {
    const re = /\[[^\]]*\]\((\/[^)\s]*)\)|href="(\/[^"]*)"/g;
    let m;
    while ((m = re.exec(page.body)) !== null) {
      const raw = m[1] ?? m[2];
      const line = page.body.slice(0, m.index).split('\n').length;
      const [target, anchor] = raw.split('#');
      const route = target.endsWith('/') || target === '' ? target || '/' : `${target}/`;

      const dest = byRoute.get(route);
      if (!dest) {
        const hint = gone.has(route)
          ? 'this route was deleted — link the page that replaced it'
          : 'no page produces this route';
        report('links', 'error', page, line, `dead link ${raw}`, hint);
        continue;
      }
      if (anchor && !dest.headings.some((h) => h.slug === anchor)) {
        report(
          'links',
          'error',
          page,
          line,
          `dead anchor ${raw}`,
          `${dest.rel} has no heading with id "${anchor}" (ids come from github-slugger; punctuation is deleted, not replaced)`,
        );
      }
    }
  }
}

// --- frontmatter -----------------------------------------------------------

if (wants('frontmatter')) {
  for (const page of selected) {
    if (!page.fm.title) {
      report('frontmatter', 'error', page, 1, 'no `title`', 'Starlight falls back to the filename in the sidebar and the tab');
    }
    const desc = page.fm.description;
    if (!desc) {
      report('frontmatter', 'error', page, 1, 'no `description`', 'this is the search result and the LinkCard subtitle; without it the reader gets the first sentence of the body');
      continue;
    }
    if (desc.length < 20) {
      report('frontmatter', 'warn', page, 1, `description is ${desc.length} characters`, 'too short to say what the page settles');
    } else if (desc.length > 200) {
      report('frontmatter', 'warn', page, 1, `description is ${desc.length} characters`, 'this is a summary, not the page — one or two sentences');
    }
  }
}

// --- sidebar ---------------------------------------------------------------

// The sidebar is an explicit ordered array rather than an autogenerated tree,
// which is the right call for a site whose reading order is not alphabetical —
// and which means a new page is invisible until somebody adds a line to it.
if (wants('sidebar')) {
  if (!existsSync(ASTRO_CONFIG)) {
    console.error(`sidebar: ${relative(ROOT, ASTRO_CONFIG)} not found`);
  } else {
    const cfg = readFileSync(ASTRO_CONFIG, 'utf8');
    const start = cfg.indexOf('sidebar:');
    const cfgPage = { rel: relative(ROOT, ASTRO_CONFIG).replace(/\\/g, '/'), route: '', headings: [] };

    if (start === -1) {
      report('sidebar', 'error', cfgPage, 1, 'no `sidebar:` in the Astro config', 'every page would fall back to alphabetical order');
    } else {
      // Bounded to the sidebar array so sibling config — customCss, social,
      // locales — cannot be mistaken for entries.
      let depth = 0, end = start;
      for (let i = cfg.indexOf('[', start); i < cfg.length; i++) {
        if (cfg[i] === '[') depth++;
        else if (cfg[i] === ']' && --depth === 0) { end = i; break; }
      }
      const block = cfg.slice(start, end + 1);
      const declared = new Set();
      for (const m of block.matchAll(/slug:\s*'([^']+)'|slug:\s*"([^"]+)"/g)) {
        declared.add(m[1] ?? m[2]);
      }

      for (const slug of declared) {
        const route = slug === '' ? '/' : `/${slug}/`;
        if (!byRoute.has(route)) {
          const line = cfg.slice(0, cfg.indexOf(`'${slug}'`)).split('\n').length;
          report('sidebar', 'error', cfgPage, line, `sidebar names \`${slug}\`, which has no page`, 'the build fails with AstroUserError: The slug ... does not exist');
        }
      }
      for (const page of selected) {
        if (page.fm.template === 'splash' || page.route === '/') continue;
        const slug = page.route.replace(/^\/|\/$/g, '');
        if (!declared.has(slug)) {
          report('sidebar', 'warn', page, 1, 'in no sidebar group', `add \`{ label: '…', slug: '${slug}' }\` to ${cfgPage.rel}, or the page is reachable only by search`);
        }
      }
    }
  }
}

// --- concept ids -----------------------------------------------------------

// `.knowledge` is the design catalogue and the docs cite it by id. A renamed
// concept leaves the prose pointing at nothing, and the reader who goes looking
// finds no such file.
if (wants('concepts')) {
  const known = new Map();
  for (const type of CONCEPT_TYPES) {
    const dir = join(KNOWLEDGE, type);
    if (!existsSync(dir)) continue;
    for (const name of readdirSync(dir)) {
      if (name.endsWith('.md')) known.set(`${type}:${basename(name, '.md')}`, true);
    }
  }
  if (known.size === 0) {
    console.error('concepts: no .knowledge catalogue found — skipping');
  } else {
    const re = new RegExp(`\\b(${CONCEPT_TYPES.join('|')}):([a-z0-9-]+)`, 'g');
    for (const page of selected) {
      let m;
      re.lastIndex = 0;
      while ((m = re.exec(page.raw)) !== null) {
        const id = `${m[1]}:${m[2]}`;
        if (known.has(id)) continue;
        const line = page.raw.slice(0, m.index).split('\n').length;
        const near = [...known.keys()].filter((k) => k.endsWith(m[2].split('-').pop()));
        report('concepts', 'error', page, line, `\`${id}\` is not in .knowledge`, near.length ? `did you mean ${near.slice(0, 3).join(', ')}?` : 'the concept was renamed or never written');
      }
    }
  }
}

// --- code the docs quote ---------------------------------------------------

// The tutorial pages name test functions and repository paths as evidence. A
// reader who follows one and finds nothing loses trust in the rest of the page,
// and this is the one class of error that a green site build will never catch.
if (wants('refs')) {
  // Walked rather than `git grep`, because a step being written is untracked
  // and its tests are exactly the ones the new page cites.
  const testNames = new Set();
  (function collect(dir) {
    let entries;
    try {
      entries = readdirSync(dir, { withFileTypes: true });
    } catch {
      return;
    }
    for (const e of entries) {
      if (e.isDirectory()) {
        if (e.name.startsWith('.') || e.name === 'node_modules' || e.name === 'dist') continue;
        collect(join(dir, e.name));
      } else if (e.name.endsWith('_test.go')) {
        const src = readFileSync(join(dir, e.name), 'utf8');
        for (const m of src.matchAll(/^func ((?:Test|Benchmark)[A-Za-z0-9_]*)/gm)) testNames.add(m[1]);
      }
    }
  })(ROOT);

  for (const page of selected) {
    if (testNames.size) {
      for (const m of page.raw.matchAll(/`((?:Test|Benchmark)[A-Z][A-Za-z0-9_]*)`/g)) {
        if (testNames.has(m[1])) continue;
        const line = page.raw.slice(0, m.index).split('\n').length;
        report('refs', 'error', page, line, `\`${m[1]}\` is not a test in this repository`, 'the test was renamed or removed; the claim it backed is now unsupported');
      }
    }
    // Repository paths written as inline code or as a Markdown link target.
    for (const m of page.raw.matchAll(/`((?:tutorial|run|session|behavior|episode|analysis|samples|examples|matchloop|cli|cmd|codegen|statesync)\/[A-Za-z0-9_./-]+\.(?:go|json|md|toml))`/g)) {
      if (existsSync(join(ROOT, m[1]))) continue;
      const line = page.raw.slice(0, m.index).split('\n').length;
      report('refs', 'error', page, line, `\`${m[1]}\` does not exist`, 'a path the reader is invited to open');
    }
  }
}

// --- website page and repository README ------------------------------------

// Each tutorial step exists twice: as a page on the site and as a README beside
// the code. Starlight cannot notice when one of them is missing, and a reader
// who arrives from GitHub sees a different set of steps than one who arrives
// from the site.
if (wants('mirror')) {
  const steps = existsSync(TUTORIAL_DIR)
    ? readdirSync(TUTORIAL_DIR).filter((d) => /^step\d+-/.test(d)).sort()
    : [];
  const cfgPage = { rel: 'tutorial/', route: '', headings: [] };

  for (const dir of steps) {
    const n = dir.match(/^step(\d+)-/)[1];
    const readme = join(TUTORIAL_DIR, dir, 'README.md');
    const route = `/tutorial/step${n}/`;
    if (!existsSync(readme)) {
      report('mirror', 'warn', { ...cfgPage, rel: `tutorial/${dir}` }, 1, 'no README.md', 'a reader arriving from GitHub has nothing to read beside the code');
    }
    if (!byRoute.has(route)) {
      report('mirror', 'error', { ...cfgPage, rel: `tutorial/${dir}` }, 1, `no website page for this step (${route})`, `write website/src/content/docs/tutorial/step${n}.mdx`);
    }
  }
  for (const page of selected) {
    const m = page.route.match(/^\/tutorial\/step(\d+)\/$/);
    if (!m) continue;
    if (!steps.some((d) => d.startsWith(`step${m[1]}-`))) {
      report('mirror', 'error', page, 1, `no tutorial/step${m[1]}-* directory`, 'the page documents code that is not in the repository');
    }
  }
}

// --- terminology -----------------------------------------------------------

if (wants('terms')) {
  for (const page of selected) {
    // stripCode blanks code with spaces of equal length, so offsets into the
    // result are still offsets into the file — which is what the line number
    // has to be counted against.
    const text = stripCode(page.raw);
    for (const term of BANNED_TERMS) {
      term.re.lastIndex = 0;
      let m;
      while ((m = term.re.exec(text)) !== null) {
        const line = text.slice(0, m.index).split('\n').length;
        report('terms', 'error', page, line, `"${m[0].trim().replace(/\s+/g, ' ')}"`, term.say);
      }
    }
  }
}

// --- prose shape (advisory input to an audit, never a verdict) -------------

if (wants('shape')) {
  for (const page of selected) {
    if (page.fm.template === 'splash') continue;
    const lines = stripCode(page.body).split('\n');
    const total = lines.filter((l) => l.trim()).length;
    if (total < 10) continue;

    const listed = lines.filter((l) => /^\s*([-*+]\s|\d+\.\s|\|)/.test(l)).length;
    const share = listed / total;
    if (share > 0.45) {
      report('shape', 'warn', page, 1, `${Math.round(share * 100)}% of the prose lines are bullets or table rows`, 'the connectives are usually the content — check whether a row here dropped a "because"');
    }

    // Every page states where it stops. On a tutorial step that is the
    // 「まだないもの」 section; elsewhere it is often a sentence in the opening.
    // A hit is a question for the auditor, not a verdict: read the page and see
    // whether the boundary is there in wording this pattern missed.
    // Japanese states a boundary in more ways than a pattern can hold, so this
    // keeps to the phrasings that rarely appear by accident. A bare 〜ではない
    // is deliberately absent: it is common enough that including it would
    // silence the check on pages that never state a limit at all.
    const limits = /まだない|ここでは扱わない|向いて(い)?ない|使わない|適して(い)?ない|できない|限界|別の(ステップ|チュートリアル|ページ)|扱わない|残っている|とは限らない|ものでは(ない|ありません)|だけでは|しかない/;
    if (!limits.test(page.body)) {
      report('shape', 'info', page, 1, 'nothing here says where this page stops', 'every page states its own boundary — check whether one is present in wording this check missed');
    }
  }
}

// ---------------------------------------------------------------------------
// Output
// ---------------------------------------------------------------------------

findings.sort(
  (a, b) =>
    SEVERITY_ORDER[a.severity] - SEVERITY_ORDER[b.severity] ||
    a.check.localeCompare(b.check) ||
    a.file.localeCompare(b.file) ||
    a.line - b.line,
);

if (asJson) {
  console.log(JSON.stringify({ pages: pages.length, findings }, null, 2));
} else {
  const counts = { error: 0, warn: 0, info: 0 };
  let lastGroup = '';
  for (const f of findings) {
    counts[f.severity]++;
    const group = `${f.severity}/${f.check}`;
    if (group !== lastGroup) {
      console.log(`\n── ${f.severity.toUpperCase()}  ${f.check} ──`);
      lastGroup = group;
    }
    console.log(`${f.file}:${f.line}  ${f.message}`);
    if (f.hint) console.log(`    ↳ ${f.hint}`);
  }
  console.log(
    `\n${pages.length} pages · ${counts.error} error · ${counts.warn} warn · ${counts.info} info`,
  );
}

process.exit(findings.some((f) => f.severity === 'error') ? 1 : 0);
