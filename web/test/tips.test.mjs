import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import './domstub.mjs';
import { el } from '../core.js';

// A blank data-pb-tip draws an empty bubble on hover (patchbay's ::after), so a tip is either words or no attribute.
test('el() leaves out a blank tip', () => {
  const real = globalThis.document;
  globalThis.document = {
    createElement: (tag) => ({
      tag,
      attrs: {},
      setAttribute(k, v) {
        this.attrs[k] = v;
      },
      append() {},
    }),
  };
  try {
    assert.ok(!('data-pb-tip' in el('label', { 'data-pb-tip': '' }).attrs));
    assert.ok(!('data-pb-tip' in el('label', { 'data-pb-tip': '  ' }).attrs));
    assert.equal(el('label', { 'data-pb-tip': 'Must be unused' }).attrs['data-pb-tip'], 'Must be unused');
  } finally {
    globalThis.document = real;
  }
});

test('no page or module sets an empty tip or title', () => {
  const dir = new URL('../', import.meta.url);
  const files = readdirSync(dir).filter((f) => /\.(js|html)$/.test(f));
  const blank = /data-pb-tip=(""|'')|\btitle=(""|'')|pbTip\s*=\s*(''|""|``)|'data-pb-tip':\s*(''|""|``)/;
  const hits = files.flatMap((f) =>
    readFileSync(new URL(f, dir), 'utf8')
      .split('\n')
      .flatMap((line, i) => (blank.test(line) ? [`${f}:${i + 1}: ${line.trim()}`] : [])),
  );
  assert.deepEqual(hits, []);
});
