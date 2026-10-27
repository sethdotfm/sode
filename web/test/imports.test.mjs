// Static check of the module graph: every name a web/*.js module imports
// from another one is exported there. Most modules touch the DOM, so they
// can't be loaded in Node; the sources are read and parsed instead, which
// catches a missing or misspelled export without a browser.
// Run: node --test web/test/
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const dir = join(dirname(fileURLToPath(import.meta.url)), '..');
const files = readdirSync(dir).filter((f) => f.endsWith('.js'));
const strip = (src) => src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '');
const sources = new Map(files.map((f) => [f, strip(readFileSync(join(dir, f), 'utf8'))]));
const names = (list) => list.split(',').map((x) => x.trim()).filter(Boolean);

// The names a module exports, following `export * from`.
function exportsOf(file, seen = new Set()) {
  if (seen.has(file)) return new Set();
  seen.add(file);
  const src = sources.get(file);
  const out = new Set();
  for (const m of src.matchAll(/^export\s+(?:async\s+)?(?:function\*?|const|let|var|class)\s+([\w$]+)/gm)) out.add(m[1]);
  for (const m of src.matchAll(/^export\s*\{([^}]*)\}(?:\s*from\s*'[^']*')?/gm)) {
    for (const n of names(m[1])) out.add(n.split(/\s+as\s+/).pop());
  }
  for (const m of src.matchAll(/^export\s*\*\s*from\s*'\.\/([\w.-]+\.js)'/gm)) {
    for (const n of exportsOf(m[1], seen)) out.add(n);
  }
  if (/^export\s+default\b/m.test(src)) out.add('default');
  return out;
}

// Every `import … from './x.js'` and `import './x.js'`.
function importsOf(file) {
  const src = sources.get(file);
  const out = [];
  for (const m of src.matchAll(/^import\s*(?:([\w$]+)\s*,?\s*)?(?:\{([^}]*)\})?\s*(?:from\s*)?'\.\/([\w.-]+\.js)'/gm)) {
    out.push({ from: m[3], named: names(m[2] || '').map((n) => n.split(/\s+as\s+/)[0]), def: m[1] });
  }
  for (const m of src.matchAll(/^export\s*\{([^}]*)\}\s*from\s*'\.\/([\w.-]+\.js)'/gm)) {
    out.push({ from: m[2], named: names(m[1]).map((n) => n.split(/\s+as\s+/)[0]) });
  }
  return out;
}

test('every named import exists in the module it comes from', () => {
  const problems = [];
  let count = 0;
  for (const file of files) {
    for (const imp of importsOf(file)) {
      if (!sources.has(imp.from)) {
        problems.push(`${file}: imports from ./${imp.from}, which does not exist`);
        continue;
      }
      const have = exportsOf(imp.from);
      for (const n of imp.named) {
        count++;
        if (!have.has(n)) problems.push(`${file}: ${n} is not exported by ${imp.from}`);
      }
      if (imp.def && !have.has('default')) problems.push(`${file}: ${imp.from} has no default export`);
    }
  }
  assert.ok(count > 100, `only ${count} imports found; the parser is broken`);
  assert.deepEqual(problems, []);
});

test('the parser sees the exports it should', () => {
  const have = exportsOf('core.js');
  for (const n of ['$', 'el', 'S', 'on', 'emit', 'connect']) assert.ok(have.has(n), n);
  assert.ok(exportsOf('levels.js').has('parseLevelsAs'));
});

test('no module imports a file that imports it back', () => {
  const graph = new Map(files.map((f) => [f, [...new Set(importsOf(f).map((i) => i.from))]]));
  const state = new Map(); // 1 = on the path, 2 = done
  const cycles = [];
  const visit = (f, path) => {
    if (state.get(f) === 2) return;
    if (state.get(f) === 1) return void cycles.push([...path.slice(path.indexOf(f)), f].join(' -> '));
    state.set(f, 1);
    for (const t of graph.get(f) || []) visit(t, [...path, f]);
    state.set(f, 2);
  };
  for (const f of files) visit(f, []);
  assert.deepEqual(cycles, []);
});

test('no module is over 1000 lines', () => {
  const big = files.filter((f) => readFileSync(join(dir, f), 'utf8').split('\n').length > 1000);
  assert.deepEqual(big, []);
});
