import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { keyLabel } from '../keys.js';

const web = new URL('../', import.meta.url);
const read = (f) => readFileSync(new URL(f, web), 'utf8');
// A hand-written label: a modifier word with +, or a Mac modifier sign. "Ctrl-click" is prose and stays.
const LABEL = /(?:Ctrl|Alt|Cmd|Shift)\+|[⌘⌃⌥]/;
const PAGES = readdirSync(web).filter((f) => f.endsWith('.html'));

test('each key list entry with data-key shows keyLabel of it, and no data-mac is left', () => {
  let seen = 0;
  for (const page of PAGES) {
    const html = read(page);
    assert.ok(!html.includes('data-mac'), `${page}: data-mac`);
    for (const [, spec, text] of html.matchAll(/<kbd data-key="([^"]*)">([^<]*)<\/kbd>/g)) {
      assert.equal(text, keyLabel(spec, false), `${page}: ${spec}`);
      seen++;
    }
  }
  assert.ok(seen > 40, 'the key lists were read');
});

test('no page writes a key label by hand', () => {
  for (const page of PAGES) {
    const html = read(page)
      .replace(/<!--[\s\S]*?-->/g, '')
      .replace(/<kbd data-key="[^"]*">[^<]*<\/kbd>/g, '');
    const hit = LABEL.exec(html);
    assert.equal(hit && html.slice(Math.max(0, hit.index - 40), hit.index + 30), null, page);
  }
});

test('no module writes a key label by hand: keyLabel in keys.js makes them all', () => {
  const hits = [];
  for (const f of readdirSync(web).filter((n) => n.endsWith('.js') && n !== 'keys.js')) {
    read(f)
      .replace(/\/\*[\s\S]*?\*\//g, '')
      .split('\n')
      .forEach((line, i) => {
        const code = line.replace(/(^|\s)\/\/.*$/, '');
        if (LABEL.test(code)) hits.push(`${f}:${i + 1}: ${code.trim().slice(0, 80)}`);
      });
  }
  assert.deepEqual(hits, []);
});
