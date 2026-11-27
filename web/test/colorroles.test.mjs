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
  // Green is routed only: the panel pages' current source and connected ring. In the Tie line picker, it is the bar
  // of a tie line that carries the source already.
  assert.deepEqual(using(/--pb-status-ok/), ['panels.css', 'tiepicker.css']);
  assert.match(src.get('tiepicker.css'), /\.sode-tiepick__choice--has \{\s*box-shadow: inset var\(--pb-accent-bar-w\) 0 0 var\(--pb-status-ok\);\s*\}/);
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
  // The green result banner is for a Take (plans.js, and a take with tie routes, tieflow.js). Other results are neutral
  // ('done' or 'info').
  assert.deepEqual(using(/pb-alert--ok\b/), ['banner.js']);
  assert.deepEqual(using(/(showResult|setResult)\(\s*\{?\s*(kind:\s*)?'ok'|kind: 'ok', title/), ['plans.js', 'tieflow.js']);
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

// Seam of the color roles and the tie line UI: tie lines use --pb-tieline, and only tie lines use it. The rules that use
// it are the picker's chosen choice and its wire, and the Via mark on Home and on a Grid panel.
test('only tie lines use --pb-tieline: the picker wire and the Via marks', () => {
  const rules = files.filter((f) => f.endsWith('.css')).flatMap((f) => (src.get(f).match(/[^{}]+\{[^}]*\}/g) || []).map((r) => [f, r]));
  const head = (r) =>
    r
      .split('{')[0]
      .replace(/\/\*[\s\S]*?\*\//g, '')
      .trim();
  const users = rules.filter(([, r]) => /--pb-tieline/.test(r.split('{')[1])).map(([f, r]) => `${f} ${head(r)}`);
  assert.deepEqual(users.sort(), [
    'base.css .sode-via',
    'gridpanel.css .sode-gridpanel__xp[data-tie-via]:not([data-n], [data-skip])::after',
    'tiepicker.css .sode-tiepick__choice--on',
    'tiepicker.css .sode-tiepick__choice--on .sode-tiepick__wire',
  ]);
  // No script sets the color in code, and the tie line hue has no other name.
  const code = files.filter((f) => f.endsWith('.js') && /--pb-tieline/.test(src.get(f).replace(/\/\/.*$/gm, '')));
  assert.deepEqual(code, []);
  assert.deepEqual(using(/--pb-tieline-hover|#d4e8be|#586647/), []);
});

test('a Down router icon is red, and each router mark has its text', () => {
  assert.match(src.get('bar.css'), /\.sode__state\[data-state="error"\] \{\s*color: var\(--pb-status-error\);/);
  // Each router mark is a .pb-status with the icon and a word (connview.js and index.html).
  assert.match(src.get('connview.js'), /el\('span', 'pb-status', dot, text\)/);
  assert.match(src.get('index.html'), /id="conn-routers"><span class="pb-status"><span class="sode__state"><\/span><span>Router<\/span>/);
});
