// Source rules, checked as text. Sources are scanned as text, vendor/ is skipped.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join, relative, sep } from 'node:path';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const self = fileURLToPath(import.meta.url);

function walk(dir) {
  const out = [];
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) {
      if (e.name !== 'vendor') out.push(...walk(p));
    } else out.push(p);
  }
  return out;
}

const files = walk(root)
  .filter((p) => p !== self)
  .map((p) => ({ path: relative(root, p).split(sep).join('/'), full: p }));

// Existing hits, each with its reason. Empty today; add "path: reason" entries rather than editing the code blindly.
const allowInnerHTML = new Map();
const allowRadius = new Map();

function scan(exts, needle, allow, message) {
  const hits = files.filter((f) => exts.some((x) => f.path.endsWith(x)) && readFileSync(f.full, 'utf8').includes(needle));
  const bad = hits.filter((f) => !allow.has(f.path)).map((f) => `web/${f.path}: ${message}`);
  assert.deepEqual(bad, []);
  const stale = [...allow.keys()].filter((p) => !hits.some((f) => f.path === p));
  assert.deepEqual(stale, [], 'allowlist entries with no hit; remove them');
}

// A router name can contain markup, so names must reach the page as text nodes, never as HTML.
test('no innerHTML in web sources', () => {
  scan(['.js', '.mjs'], 'inner' + 'HTML', allowInnerHTML, 'uses innerHTML; build DOM with el()');
});

// Patchbay has no rounded corners. Inline style strings in .js are covered by scanning the whole file.
test('no border-radius in CSS or inline styles', () => {
  scan(['.css', '.js', '.mjs'], 'border-' + 'radius', allowRadius, 'uses border-radius; patchbay allows none');
});
