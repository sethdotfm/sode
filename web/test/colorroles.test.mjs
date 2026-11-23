// Static check of the color roles (docs/color-roles.md). Each color has one job, and Take is amber. Red is failure
// only, and healthy and info states show no color. TestChromeColorRoles checks the pages themselves.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const dir = join(dirname(fileURLToPath(import.meta.url)), '..');
const files = readdirSync(dir).filter((f) => /\.(js|css|html)$/.test(f));
const src = new Map(files.map((f) => [f, readFileSync(join(dir, f), 'utf8')]));
const doc = readFileSync(join(dir, '..', 'docs', 'color-roles.md'), 'utf8');
const using = (re) => files.filter((f) => re.test(src.get(f)));

test('no pb-btn--ok: the main action is sode-btn--primary', () => {
  assert.deepEqual(using(/pb-btn--ok/), []);
  assert.match(src.get('base.css'), /\.pb-btn\.sode-btn--primary \{[^}]*--pb-accent/);
});

test('each pb-btn--danger left is a failure named in docs/color-roles.md', () => {
  for (const f of using(/pb-btn--danger/)) assert.ok(doc.includes(f), `${f} uses pb-btn--danger but docs/color-roles.md does not name it`);
});

test('every Take control is amber, never the red button', () => {
  const takes = [
    [/class="pb-btn pb-btn--commit sode__patch"/, 'index.html'],
    [/class="pb-btn pb-btn--commit sode__confirm-ok" id="confirm-ok">Take/, 'index.html'],
    [/class="pb-btn pb-btn--commit sode__confirm-ok" id="confirm-ok">Take/, 'panel.html'],
    [/btn\('pb-btn--commit', 'Take'/, 'panelmount.js'],
    [/'pb-btn pb-btn--commit sode-panel__take sode-gridpanel__take'/, 'gridpaneltake.js'],
    [/act\('pb-btn--commit sode-gridpop__take'/, 'gridpopup.js'],
    [/toggle\('pb-btn--commit', !t\.ok\)/, 'desktop.js'],
    [/toggle\('pb-btn--commit', last && !t\.ok\)/, 'mobile.js'],
  ];
  for (const [re, f] of takes) assert.match(src.get(f), re, f);
  // The Take cells and buttons, the phone's sheet and slider: no red.
  const rules = [...files.filter((f) => f.endsWith('.css')).flatMap((f) => src.get(f).match(/[^{}]+\{[^}]*\}/g) || [])];
  const take = rules.filter((r) => /sode-panel__take|sode-gridpanel__take|sode__slide|sode-m__sheet|sode-ov__k-take|sode__patch/.test(r.split('{')[0]));
  assert.ok(take.length > 5);
  for (const r of take) assert.doesNotMatch(r, /--pb-status-error|--pb-danger-hover|--pb-hue-red/, r);
});

test('info and healthy states show no color', () => {
  assert.deepEqual(using(/pb-badge--info|pb-banner--info|pb-toast--info|pb-badge--ok|--pb-status-info/), []);
  // Green is routed only: the panel pages' current source and connected ring.
  assert.deepEqual(using(/--pb-status-ok/), ['panels.css']);
  assert.doesNotMatch(src.get('panels.css'), /\.sode-panel__status--ok \{[^}]*--pb-status-ok/);
});

test('every state row in docs/color-roles.md names a non-color cue', () => {
  const state = doc.split('## State')[1].split('\n\n')[1];
  const rows = state.split('\n').filter((l) => l.startsWith('| ') && !l.startsWith('| Role') && !l.startsWith('|---'));
  assert.ok(rows.length >= 6);
  for (const r of rows) {
    const cells = r.split('|').map((c) => c.trim());
    assert.ok(cells[4]?.length > 5, `no cue in: ${r}`);
  }
});

// A status modifier built from a template ("pb-btn--${s}") hides its color from the checks above, so each one is a literal.
test('status modifier classes are literals, never built from a template', () => {
  assert.deepEqual(using(/pb-(btn|alert|toast|icon|badge|note|progress__bar)--(\$\{|['"`]\s*\+)/), []);
});

test('green is routed only: no green or blue status modifier outside a Take result', () => {
  assert.deepEqual(using(/pb-(icon|badge|toast|note)--ok\b|pb-progress__bar--ok\b|pb-(icon|alert|note)--info\b/), []);
  // The green result banner is for a Take (plans.js, and the take of the Tie line picker). Other results are neutral
  // ('done' or 'info').
  assert.deepEqual(using(/pb-alert--ok\b/), ['banner.js']);
  assert.deepEqual(using(/(showResult|setResult)\(\s*\{?\s*(kind:\s*)?'ok'|kind: 'ok', title/), ['plans.js', 'tiepicker.js']);
});

test('a neutral alert and toast have no blue: base.css overrides patchbay with tokens', () => {
  const css = src.get('base.css');
  assert.match(css, /\.pb-alert:not\(\[class\*="pb-alert--"\]\) \{[^}]*border-left-color: var\(--pb-[a-z-]+\)/);
  assert.match(css, /\.pb-toast:not\(\[class\*="pb-toast--"\]\) \{[^}]*--pb-toast-role: var\(--pb-[a-z-]+\)/);
});

test('unsaved work is amber, never red, and a clean Save has no state color', () => {
  assert.match(src.get('index.html'), /class="pb-note pb-note--warn" id="settings-unsaved"/);
  assert.match(src.get('bookmarkeditor.js'), /toggle\('pb-btn--commit', dirty && !ro\)/);
  assert.match(src.get('presets.js'), /toggle\('pb-btn--commit', dirty\)/);
});

test('no modifier class whose CSS is gone', () => {
  assert.deepEqual(using(/sode__role-badge/), []);
});
