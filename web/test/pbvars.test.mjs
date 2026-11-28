// Every var(--pb-*) that Sode's CSS reads must be a name that patchbay or Sode defines. An undefined name makes the
// whole declaration invalid at computed time, so a border or weight goes away with no error in the console.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const dir = join(dirname(fileURLToPath(import.meta.url)), '..');
const vendor = join(dir, 'vendor', 'patchbay', 'css');
const own = readdirSync(dir).filter((f) => f.endsWith('.css'));
const read = (p) => readFileSync(p, 'utf8');
const sheets = [...readdirSync(vendor).map((f) => read(join(vendor, f))), ...own.map((f) => read(join(dir, f)))];
const defined = new Set(sheets.flatMap((s) => [...s.matchAll(/(--pb-[\w-]+)\s*:/g)].map((m) => m[1])));

test('the scan finds the patchbay tokens', () => {
  for (const n of ['--pb-space-2', '--pb-edge-soft', '--pb-weight-normal', '--pb-border-w']) assert.ok(defined.has(n), n);
});

test('every var(--pb-*) in web/*.css is defined', () => {
  const misses = [];
  for (const f of own) {
    read(join(dir, f))
      .split('\n')
      .forEach((line, i) => {
        for (const m of line.matchAll(/var\(\s*(--pb-[\w-]+)/g)) if (!defined.has(m[1])) misses.push(`${f}:${i + 1} ${m[1]}`);
      });
  }
  assert.deepEqual(misses, []);
});
