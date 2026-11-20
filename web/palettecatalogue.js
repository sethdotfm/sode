// The catalogue of Palette rows: everything a person can use, with a check of their Rights and Role. Pure (no DOM, no
// storage), so Node tests build it. The Palette (palette.js) lists the rows. Bookmarks reuse the same rows and keys.
//
// catalogue(src) gives Row[]. The fields of src:
// - page: 'main', 'log', 'panels', 'editor' or 'panel', the page the Palette is on (pageOf).
// - presets: [{ scope, id, name }]. The scope is 'global' (Everyone), 'user' (Me) or 'local' (This browser).
// - filterSets: [{ scope, id, name }]. The scope 'all' is the built-in All, id All. A row key holds the hidden id.
// - panels: [{ id, name, type }], the panels that the browser's own sign-in reaches (palettePanels).
// - settings: [{ section, sectionTitle, topic, id, label, button }], from the Settings form (palettesettings.js). button
//   is the button's id, or '' for a setting.
// - who: { role, user, rights, locks, users, phone, stepped }. This is the browser's own sign-in, never a panel key.
//   It holds lockState's role and user, myRights, and the hello's cfg.locks and users.
// - keyList: true when this page has its own key list (Home and a Grid panel page). Keyboard shortcuts opens it here,
//   else it opens Home and says so.
//
// The fields of a Row:
// - key: stable text that names the row, '<kind>:<target>'. Last picks and Bookmarks keep it. rowByKey finds it.
// - kind: 'action', 'preset', 'page', 'panel', 'filterSet', 'setting' or 'settingsButton'.
// - cat: a CATEGORIES id (paletterules.js).
// - target: what the row opens. An action has { id }, a preset { scope, id, name }, a page or panel { href }.
//   A filter set has { scope, name }, a setting { section, sectionTitle, id }. A Settings button also has button (its
//   id) and mode (buttonMode).
// - home: true when the row runs on Home. Another page sends it to Home first (palette.js).
// - access: { ok, lock, why, offer }. When ok is false, the row is grey and why says why.
//   offer lists what can still get it: 'stepIn' (v2's Step in) or 'approval' (its Save asks for v2's Approval).
//   lock: the page asks for a sign-in first, and the row leads there.
// - alias: true for a Settings button that does the same as an action (Sign out). It is that action under the Settings
//   key, so a bookmark of the button keeps working. The Palette lists only the action (paletteView).
import { allowed } from './rightsrules.js';
import { sectionView, SECTION_ROLES, ROLE_WHO } from './settingsrolerules.js';
import { stepInOffered } from './stepinrules.js';

const SCOPE_WORD = { global: 'Everyone', user: 'Me', local: 'This browser', all: '' };

// The page under the app's root path (rel has no leading slash): '' Home, 'log', 'panels', 'panels/editor' or a panel.
export function pageOf(rel) {
  const p = String(rel || '').replace(/^\/+|\/+$/g, '');
  if (!p) return 'main';
  if (p === 'log') return 'log';
  if (p === 'panels') return 'panels';
  if (p === 'panels/editor') return 'editor';
  if (p.startsWith('panels/')) return 'panel';
  return 'main';
}

// Actions that Sode already has. home: runs on Home. desktop: the phone layout has no such control. Re-read needs only
// the socket, so it runs on every page.
export const ACTIONS = Object.freeze([
  { id: 'reread', label: 'Re-read router names', icon: 'refresh-cw', words: ['refresh', 'names', 'reload'] },
  { id: 'undo', label: 'Undo the last Sode take', icon: 'undo-2', home: true, desktop: true, words: ['undo', 'back'] },
  { id: 'presets', label: 'All presets', icon: 'list', home: true, words: ['presets popup', 'fire'] },
  { id: 'filters', label: 'Filters', icon: 'filter', home: true, words: ['history', 'filter sets'] },
  { id: 'compact', label: 'Compact view on or off', icon: 'minimize-2', home: true, desktop: true, words: ['compact', 'hide history'] },
  { id: 'theme', label: 'Dark or light theme', icon: 'moon', words: ['theme', 'dark', 'light', 'colour', 'color'] },
  { id: 'shortcuts', label: 'Keyboard shortcuts', icon: 'keyboard', home: true, desktop: true, words: ['keys', 'help', 'shortcut list'] },
  { id: 'lock', label: 'Lock', icon: 'lock', words: ['lock screen', 'away'] },
  { id: 'signIn', label: 'Sign in', icon: 'key-round', signedOut: true, words: ['pin', 'user', 'log in', 'login'] },
  { id: 'signOut', label: 'Sign out', icon: 'door-open', signedIn: true, words: ['log out', 'logout', 'user', 'leave'] },
  { id: 'stepIn', label: 'Step in', icon: 'user', home: true, desktop: true, words: ['step in', 'pin', 'user'] },
  { id: 'stepOut', label: 'Step out', icon: 'door-open', stepped: true, words: ['step out', 'user', 'back'] },
]);

// The Settings buttons that do the same as an action. Settings > My user shows each one only when its action applies.
export const BUTTON_ACTIONS = Object.freeze({
  'account-login': 'signIn',
  'account-logout': 'signOut',
  'account-stepin': 'stepIn',
  'account-stepout': 'stepOut',
  'lock-btn': 'lock',
});
// A Palette row presses these buttons where the Palette is (on Home), with no Settings view. Each one opens its own
// dialog, which works over any view.
const BUTTONS_RUN = new Set(['account-pin', 'account-keys', 'account-clients', 'account-profile', 'account-copy', 'account-transfer', 'xi-open']);
// These open Settings at the button and then press it, since what they do shows in the Settings view. Clear History and
// Clear Sode's Protects show an inline confirm. A my default button changes the value next to it. Manage users, Apply
// an Access preset and Run setup again stage edits for Settings' one Save.
const BUTTONS_IN_SETTINGS = new Set(['clear-history', 'protects-clear-all', 'cfg-apply-preset', 'users-manage', 'setup-again-open']);
// Only a signed-in user has these, and only a user with a PIN has API keys and API clients (devicesettings.js).
const BUTTONS_SIGNED_IN = new Set(['account-pin', 'account-keys', 'account-clients', 'account-profile', 'account-copy', 'account-transfer']);
const BUTTONS_PIN = new Set(['account-keys', 'account-clients']);

// What a Palette row of a Settings button does. 'action': an ACTIONS row does it. 'run': press it here. 'settings':
// open Settings at it, then press it. 'locate': open Settings at it (a button that no list names).
export function buttonMode(id) {
  if (BUTTON_ACTIONS[id]) return 'action';
  if (BUTTONS_RUN.has(id)) return 'run';
  if (BUTTONS_IN_SETTINGS.has(id) || /^my-default-/.test(id)) return 'settings';
  return 'locate';
}

// Whether the browser's own sign-in has the action at all.
function actionApplies(a, who) {
  if (a.desktop && who.phone) return false;
  if (a.signedOut && (who.user || !(who.users || []).some((u) => u.len > 0))) return false;
  if (a.signedIn && (!who.user || who.stepped)) return false;
  if (a.stepped && !who.stepped) return false;
  if (a.id === 'stepIn' && !stepOffer(who).length) return false;
  return true;
}

function buttonShown(id, who) {
  if (BUTTONS_SIGNED_IN.has(id) && !who.user) return false;
  return !(BUTTONS_PIN.has(id) && (who.users || []).find((u) => u.name.toLowerCase() === who.user.toLowerCase())?.len === 0);
}

const OK = Object.freeze({ ok: true, lock: false, why: '', offer: [] });

const stepOffer = (who) => (stepInOffered({ locked: false, phone: !!who.phone, stepped: !!who.stepped, user: who.user, users: who.users }) ? ['stepIn'] : []);

// A page with its lock on asks a Guest for a sign-in first. The row is not grey: it leads to that sign-in.
const pageLock = (who, gate) => (who.locks?.[gate] && !who.user ? { ...OK, lock: true, why: 'Sign in on this page to open it.' } : OK);

function settingAccess(section, who) {
  const view = sectionView(section, who.role || '', false);
  if (view === 'edit') return OK;
  const t = SECTION_ROLES[section];
  const by = ROLE_WHO[t?.edit] || 'a higher Role';
  if (view === 'read')
    return { ok: false, lock: false, why: `Outside your Role. A change here needs Approval from ${by}.`, offer: ['approval', ...stepOffer(who)] };
  return { ok: false, lock: false, why: `Outside your Role. Only ${by} can see this.`, offer: stepOffer(who) };
}

const presetAccess = (p, who) =>
  allowed(who.rights, 'presets', p.id) ? OK : { ok: false, lock: false, why: `Outside your Rights: preset ${p.name}.`, offer: stepOffer(who) };

export function catalogue(src) {
  const who = src.who || {};
  const rows = [];
  const add = (row) => rows.push({ hint: '', words: [], icon: '', home: false, access: OK, ...row });

  if (src.page !== 'main')
    add({
      key: 'page:home',
      kind: 'page',
      cat: 'pages',
      label: 'Home',
      icon: 'house',
      target: { href: './' },
      access: pageLock(who, 'main'),
      words: ['main', 'patch'],
    });

  for (const a of ACTIONS) {
    if (!actionApplies(a, who)) continue;
    const own = a.id === 'shortcuts' && src.page !== 'main' && src.keyList;
    const hint = a.home && !own && src.page !== 'main' ? 'Opens Home' : '';
    add({
      key: `action:${a.id}`,
      kind: 'action',
      cat: 'actions',
      label: a.label,
      icon: a.icon,
      words: a.words,
      home: !!a.home && !own,
      hint,
      target: { id: a.id },
    });
  }

  for (const p of src.presets || []) {
    add({
      key: `preset:${p.scope}:${p.id}`,
      kind: 'preset',
      cat: 'presets',
      label: p.name,
      hint: SCOPE_WORD[p.scope] || '',
      icon: 'zap',
      words: ['preset'],
      home: true,
      target: { scope: p.scope, id: p.id, name: p.name },
      access: presetAccess(p, who),
    });
  }

  add({
    key: 'page:settings',
    kind: 'page',
    cat: 'pages',
    label: 'Settings',
    icon: 'settings',
    home: true,
    target: { href: './' },
    access: pageLock(who, 'main'),
  });
  add({
    key: 'page:log',
    kind: 'page',
    cat: 'pages',
    label: 'Communication Log',
    icon: 'scroll-text',
    target: { href: 'log' },
    access: pageLock(who, 'log'),
    words: ['log'],
  });
  add({ key: 'page:panels', kind: 'page', cat: 'pages', label: 'Panels', icon: 'layout-grid', target: { href: 'panels' }, access: pageLock(who, 'panel') });
  if (!who.phone)
    add({
      key: 'page:editor',
      kind: 'page',
      cat: 'pages',
      label: 'Panel editor',
      icon: 'pencil',
      target: { href: 'panels/editor' },
      access: pageLock(who, 'panel'),
      words: ['edit panels'],
    });

  for (const p of src.panels || []) {
    add({
      key: `panel:${p.id}`,
      kind: 'panel',
      cat: 'panels',
      label: p.name,
      hint: p.type === 'grid' ? 'Grid panel' : 'Buttons panel',
      icon: p.type === 'grid' ? 'grid-3x3' : 'layout-grid',
      words: ['panel'],
      target: { href: `panels/${encodeURIComponent(p.id)}`, id: p.id },
      access: pageLock(who, 'panel'),
    });
  }

  for (const s of src.filterSets || []) {
    add({
      key: `filterSet:${s.scope}:${s.id}`,
      kind: 'filterSet',
      cat: 'filterSets',
      label: s.scope === 'all' ? 'All (no filter set)' : s.name,
      hint: SCOPE_WORD[s.scope] || '',
      icon: 'filter',
      words: ['filter set', 'history'],
      home: true,
      target: { scope: s.scope, id: s.id },
    });
  }

  for (const s of src.settings || []) {
    if (who.phone && sectionView(s.section, who.role || '', true) === 'hidden') continue;
    const key = `setting:${s.section}/${s.id}`;
    const hint = [s.topic, s.sectionTitle].filter(Boolean).join(' › ');
    const mode = s.button ? buttonMode(s.button) : '';
    if (mode === 'action') {
      const a = rowByKey(rows, `action:${BUTTON_ACTIONS[s.button]}`);
      if (a) add({ ...a, key, hint, alias: true });
      continue;
    }
    if (s.button && !buttonShown(s.button, who)) continue;
    const acts = mode === 'run' || mode === 'settings';
    add({
      key,
      kind: s.button ? 'settingsButton' : 'setting',
      cat: 'settings',
      label: s.label,
      hint,
      icon: acts ? 'mouse-pointer-click' : 'settings',
      home: true,
      target: { section: s.section, sectionTitle: s.sectionTitle, id: s.id, ...(s.button ? { button: s.button, mode } : {}) },
      access: settingAccess(s.section, who),
    });
  }
  return rows;
}

export const rowByKey = (rows, key) => rows.find((r) => r.key === key) || null;
