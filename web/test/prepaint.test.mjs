// prepaint.js repeats prefs.js resolve() and cachedProfile() inline because it runs before first paint.
// This evaluates it against stubs and checks it agrees with them over a matrix of values.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import vm from 'node:vm';
import { PREFS, resolve, cachedProfile } from '../prefs.js';

const src = readFileSync(join(dirname(fileURLToPath(import.meta.url)), '..', 'prepaint.js'), 'utf8');

// Returns the page's dataset after the script ran: theme (unset for system), layout, and flip (present when on).
function run({ own = {}, profile, global, session, tab, phoneMedia = false, only }) {
  const store = new Map(Object.entries(own));
  if (profile !== undefined) store.set('sode-profile-prefs', JSON.stringify(profile));
  if (global !== undefined) store.set('sode-device-defaults', JSON.stringify(global));
  if (session) store.set('sode-session', 'tok');
  const dataset = {};
  const document = { documentElement: { dataset }, currentScript: { dataset: only ? { only } : {} } };
  const localStorage = { getItem: (k) => (store.has(k) ? store.get(k) : null) };
  const sessionStorage = { getItem: (k) => (k === 'sode-tab-session' && tab ? 'tok' : null) };
  vm.runInNewContext(src, { document, localStorage, sessionStorage, matchMedia: () => ({ matches: phoneMedia }) });
  return dataset;
}

const NAMES = ['theme', 'layout', 'flip'];
const junk = [undefined, 'nonsense', '', 7, null, {}, [], 'constructor'];
const valuesOf = (name) => [...Object.keys(PREFS[name].values), ...(PREFS[name].bool ? [true, false] : []), ...junk];
const sessions = [
  { session: false, tab: false },
  { session: true, tab: false },
  { session: false, tab: true },
];

for (const name of NAMES) {
  test(`prepaint agrees with resolve() for ${name}`, () => {
    const key = PREFS[name].key;
    const fails = [];
    let n = 0;
    for (const own of valuesOf(name)) {
      for (const prof of valuesOf(name)) {
        for (const glob of valuesOf(name)) {
          for (const { session, tab } of sessions) {
            for (const phoneMedia of name === 'layout' ? [false, true] : [false]) {
              const stored = typeof own === 'string' ? own : null;
              const profile = { user: 'amy', prefs: prof === undefined ? {} : { [name]: prof } };
              const global = glob === undefined ? {} : { [name]: glob };
              const ds = run({ own: stored === null ? {} : { [key]: stored }, profile, global, session, tab, phoneMedia });
              let want = resolve(name, stored, cachedProfile(profile, session || tab).prefs, global);
              if (name === 'layout' && want === 'auto') want = phoneMedia ? 'phone' : 'desktop';
              const got = name === 'theme' ? ds.theme || 'system' : name === 'flip' ? ('flip' in ds ? 'on' : 'off') : ds.layout;
              n++;
              if (got !== want) {
                const at = JSON.stringify({ own, prof, glob, session, tab, phoneMedia });
                fails.push(`${at}: prepaint ${got}, resolve ${want}`);
              }
            }
          }
        }
      }
    }
    assert.ok(n > 500, `only ${n} cases ran`);
    assert.deepEqual(fails.slice(0, 5), []);
  });
}

test('prepaint ignores a cached profile with no user or no prefs object', () => {
  const global = { theme: 'dark' };
  for (const profile of [{ user: '', prefs: { theme: 'light' } }, { user: 'amy', prefs: 'x' }, { prefs: { theme: 'light' } }, null, 'junk']) {
    assert.equal(run({ profile, global, session: true }).theme, 'dark', JSON.stringify(profile));
  }
});

test('prepaint with data-only=theme sets only the theme', () => {
  const ds = run({ global: { theme: 'dark', layout: 'phone', flip: true }, only: 'theme' });
  assert.deepEqual({ ...ds }, { theme: 'dark' });
});
