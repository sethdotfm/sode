import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import './domstub.mjs';
import { el } from '../core.js';
import { placeTip } from '../tooltip.js';

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

// Every data-pb-tip draws in tooltip.js's fixed tip on the body, so no modal or scroller cuts it off (AB-09).
test('the CSS tip makes no box, and no page draws one of its own', () => {
  const dir = new URL('../', import.meta.url);
  assert.match(readFileSync(new URL('base.css', dir), 'utf8'), /\[data-pb-tip\]::after \{\n {2}content: none;\n\}/);
  const files = readdirSync(dir).filter((f) => f.endsWith('.css'));
  const draws = files.flatMap((f) =>
    [...readFileSync(new URL(f, dir), 'utf8').matchAll(/([^{}]*\[data-pb-tip[^{}]*::after[^{}]*)\{([^}]*)\}/g)]
      .filter(([, sel, body]) => /\bcontent:/.test(body) && !/^\s*\[data-pb-tip\]::after\s*$/.test(sel.replace(/\/\*[\s\S]*?\*\//g, '')))
      .map(([, sel]) => `${f}: ${sel.trim()}`),
  );
  assert.deepEqual(draws, []);
});

test('placeTip keeps the tip inside the window', () => {
  const vw = 1000,
    vh = 600,
    w = 200,
    h = 30;
  const box = (left, top, width = 100, height = 40) => ({ left, top, width, height, right: left + width, bottom: top + height });
  // Under the element, centred.
  assert.deepEqual(placeTip({ r: box(400, 100), w, h, vw, vh }), { left: 350, top: 144 });
  // pb-tip--start and pb-tip--end line it up with an edge.
  assert.equal(placeTip({ r: box(400, 100), w, h, vw, vh, align: 'start' }).left, 400);
  assert.equal(placeTip({ r: box(400, 100), w, h, vw, vh, align: 'end' }).left, 300);
  // pb-tip--top puts it over the element.
  assert.equal(placeTip({ r: box(400, 100), w, h, vw, vh, up: true }).top, 66);
  // No room under it: over it. No room over it: under it.
  assert.equal(placeTip({ r: box(400, 550), w, h, vw, vh }).top, 516);
  assert.equal(placeTip({ r: box(400, 10), w, h, vw, vh, up: true }).top, 54);
  // Near a side of the window it moves in, a gap from the edge.
  assert.equal(placeTip({ r: box(950, 100, 40), w, h, vw, vh }).left, 796);
  assert.equal(placeTip({ r: box(0, 100, 20), w, h, vw, vh }).left, 4);
  // No room on either side: the side with more room, still inside the window.
  const tall = placeTip({ r: box(400, 20, 100, 560), w, h, vw, vh: 600 });
  assert.ok(tall.top >= 4 && tall.top + h <= 596);
  assert.equal(placeTip({ r: box(400, 10, 100, 560), w, h, vw, vh: 600 }).top, 566);
});
