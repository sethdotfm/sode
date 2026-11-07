// Review sheet: what an import file or share link would change, shown before anything is saved. Used by transfer.js and share links.
// Saving goes through the same stores and savers as the Filters and Presets dialogs; pure rules are in exportfmt.js.

import { el, S, on, off, sentence } from './core.js';
import { trapDialog } from './widgets.js';
import { showResult } from './plans.js';
import { globalSaver, profileSaver } from './globalsave.js';
import { profile, profileName, fetchProfile, saveProfile } from './profile.js';
import { PREFS } from './prefs.js';
import { ownPref, setPref } from './device.js';
import { setsIn, putLocalSets, filterSummary, normFilter } from './filterstore.js';
import { presetsIn, putLocalPresets, normPreset, presetProblem, presetsProblem, saved } from './presetstore.js';
import * as fmt from './exportfmt.js';
import { scopeLabel } from './scopes.js';


// ---- the dialog

const root = el('div', 'pb-modal-backdrop sode__scrim');
root.hidden = true;
const dialog = el('div', 'pb-modal sode__filters sode__rv');
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-labelledby', 'rv-title');
dialog.tabIndex = -1;

const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};
const status = (dot, text) => {
  const d = el('span', `pb-dot pb-dot--${dot}`);
  d.setAttribute('aria-hidden', 'true');
  return el('span', 'pb-status', d, text);
};
const select = (label, options, value, onChange) => {
  const s = el('select', 'pb-select');
  s.setAttribute('aria-label', label);
  for (const [v, t] of options) s.append(new Option(t, v));
  s.value = value;
  s.addEventListener('change', () => onChange(s.value));
  return el('span', 'pb-select-wrap', s);
};

const titleEl = Object.assign(el('span', 'pb-modal__title', 'Review import'), { id: 'rv-title' });
const fromEl = el('span', 'sode__hint');
const targetBox = el('div', 'sode__filters-set');
const noteTop = el('span', 'sode__hint');
const prefsBox = el('div', 'sode__rv-sec');
const rowsBox = el('div', 'sode__rv-sec');
const sumEl = el('span', 'sode__hint');
sumEl.setAttribute('aria-live', 'polite');
const noteEl = el('span', 'sode__hint');
noteEl.setAttribute('aria-live', 'polite');
// A PIN row per saver for everyone (filter sets and presets are two messages); one shows at a time.
function pinControls(cancel) {
  const pinIn = el('input', 'pb-input');
  pinIn.type = 'password';
  pinIn.placeholder = 'PIN';
  pinIn.autocomplete = 'current-password';
  pinIn.setAttribute('aria-label', 'Settings PIN');
  const pinOk = btn('pb-btn--ok pb-btn--sm', 'Unlock', () => {});
  const pinRow = el('div', 'sode__filters-set', pinIn, pinOk, btn('pb-btn--ghost pb-btn--sm', 'Cancel', cancel));
  pinRow.hidden = true;
  return { pinIn, pinOk, pinRow };
}
const pinA = pinControls(() => gsave.cancel());
const pinB = pinControls(() => gsavePresets.cancel());
const cancelBtn = btn('pb-btn--outline', 'Cancel', () => close());
const nowBtn = btn('pb-btn--outline', 'Apply now', () => applyNowClick());
const applyBtn = btn('pb-btn--ok', 'Apply', () => apply());

dialog.append(
  el('div', 'pb-modal__header', titleEl),
  el('div', 'pb-modal__body sode__filters-body', fromEl, targetBox, pinA.pinRow, pinB.pinRow, noteTop, prefsBox, rowsBox, sumEl, noteEl),
  el('div', 'pb-modal__footer', cancelBtn, nowBtn, applyBtn));
root.append(dialog);
document.body.append(root);

const setNote = (t, error = false) => {
  noteEl.textContent = t;
  noteEl.classList.toggle('sode__error-text', error);
};

// ---- state

let open = false, back = null, busy = false;
// doc: the export document; rows: one per filter set / preset in it.
let st = null;
let fetched = null; // another user's profile while reviewing for them: { state: 'wait' | 'ok' | 'err', profile, err }

const gsave = globalSaver({
  t: 'filterSets', resultT: 'filterSetsResult', ...pinA, setNote,
  isOpen: () => open, failTitle: 'Import not saved', render: () => refresh(),
});
const gsavePresets = globalSaver({
  t: 'presets', resultT: 'presetsResult', ...pinB, setNote,
  isOpen: () => open, failTitle: 'Import not saved', render: () => refresh(),
});
const psave = profileSaver({ setNote, isOpen: () => open, failTitle: 'Import not saved', render: () => refresh() });
const saving = () => busy || gsave.busy() || gsavePresets.busy() || psave.busy();

const forUser = () => st?.user && st.user !== profileName();
const live = () => ({ srcs: S.srcs, dsts: S.dsts, levelCount: S.levels.length });
const scope = () => st.scope;

function curSets(sc) {
  if (sc === 'user' && forUser()) return (fetched?.profile?.filterSets || []).filter((s) => s?.name).map((s) => ({ name: String(s.name), f: normFilter(s) }));
  return setsIn(sc).map((s) => ({ name: s.name, f: s.f }));
}
function curPresets(sc) {
  if (sc === 'user' && forUser()) return (fetched?.profile?.presets || []).filter((s) => s?.name).map((s) => ({ ...normPreset(s), pinned: s.pinned !== false }));
  return presetsIn(sc).map((s) => ({ name: s.name, items: s.items, pinned: s.pinned !== false }));
}
const curNames = (kind, sc) => (kind === 'filter' ? curSets(sc) : curPresets(sc)).map((x) => x.name);
const curPrefs = (sc) => (sc === 'user' ? (forUser() ? fetched?.profile?.prefs : profile().prefs) || {} : null);

// A preset row's match is made fresh each time, so names loaded after the sheet opened count.
function makeRows(doc) {
  return [
    ...(doc.filterSets || []).map((s) => ({ kind: 'filter', name: String(s.name), f: fmt.filterOf(s), choice: '', as: '' })),
    ...(doc.presets || []).map((p) => ({ kind: 'preset', name: String(p.name), src: p, choice: '', as: '' })),
  ];
}
const matched = (r) => fmt.matchPreset(r.src, live());
const clashOf = (r) => fmt.conflict(r.name, curNames(r.kind, scope()));

// Own choice, else default (Keep both on a clash, else import; an ambiguous preset match waits as Skip).
function choiceOf(r) {
  const clash = clashOf(r);
  const c = r.choice || (r.kind === 'preset' && matched(r).count.ambiguous ? 'skip' : fmt.defaultChoice(clash));
  if (c === 'skip') return 'skip';
  if (!clash) return 'import';
  return c === 'replace' ? 'replace' : 'both';
}

function prefList() {
  const out = [];
  for (const [name, value] of Object.entries(st.doc.prefs || {})) {
    const ok = Object.hasOwn(PREFS, name) && Object.hasOwn(PREFS[name].values, value);
    out.push({ name, value, ok });
  }
  return out;
}
const prefTarget = () => (forUser() || (st.prefsTo === 'user' && profileName()) ? 'user' : 'local');


function plan() {
  const sc = scope();
  const out = { problems: [], parts: {}, res: {}, prefs: {} };
  for (const kind of ['filter', 'preset']) {
    const rs = st.rows.filter((r) => r.kind === kind);
    if (!rs.length) continue;
    const existing = kind === 'filter' ? curSets(sc) : curPresets(sc);
    const incoming = rs.map((r) => (kind === 'filter' ? { name: r.name, f: r.f } : { ...normPreset({ name: r.name, items: matched(r).saved.items }), pinned: true }));
    const choices = rs.map(choiceOf);
    const picks = rs.map((r, i) => {
      if (choices[i] !== 'both') return { choice: choices[i] };
      const others = [...existing.map((x) => x.name), ...rs.filter((x, j) => j !== i && choices[j] !== 'skip').map((x) => x.name)];
      const as = r.as || fmt.renameFor(r.name, others);
      const bad = clashOf(r) ? fmt.renameProblem(as, others) : '';
      if (bad) out.problems.push(`“${r.name}”: ${bad}`);
      r.autoAs = as;
      r.asProblem = bad;
      return { choice: 'both', as };
    });
    rs.forEach((r, i) => { if (choices[i] !== 'both') r.asProblem = ''; });
    const res = fmt.applyChoices(existing, incoming, picks);
    out.res[kind] = res;
    if (kind === 'preset') {
      rs.forEach((r, i) => {
        if (choices[i] === 'skip') return;
        const err = presetProblem(incoming[i]);
        if (err) out.problems.push(`Preset “${r.name}”: ${err}`);
      });
      if (res.added + res.replaced) {
        const err = presetsProblem(res.list);
        if (err) out.problems.push(err);
      }
    }
    if (res.added + res.replaced) {
      out.parts[kind === 'filter' ? 'filterSets' : 'presets'] = kind === 'filter'
        ? res.list.map((s) => ({ name: s.name, ...s.f }))
        : res.list.map(saved);
    }
  }
  const pl = prefList().filter((p) => p.ok && st.prefOn[p.name] !== false && p.value !== currentPref(p.name));
  for (const p of pl) out.prefs[p.name] = p.value;
  if (Object.keys(out.prefs).length && prefTarget() === 'user') {
    out.parts.prefs = { ...curPrefs('user'), ...out.prefs };
  }
  const sizeParts = { ...out.parts };
  if (sc === 'global') {
    const g = {};
    if (sizeParts.filterSets) g.sets = sizeParts.filterSets;
    if (sizeParts.presets) g.presets = sizeParts.presets;
    const big = fmt.sizeProblem('global', g);
    if (big) out.problems.push(big);
  } else if (sc === 'user') {
    const base = { prefs: curPrefs('user'), filterSets: forUser() ? fetched?.profile?.filterSets || [] : profile().filterSets, presets: forUser() ? fetched?.profile?.presets || [] : profile().presets };
    const big = fmt.sizeProblem('user', { ...base, ...sizeParts });
    if (big) out.problems.push(big);
  }
  return out;
}

function currentPref(name) {
  return prefTarget() === 'user' ? (curPrefs('user')[name] || '') : ownPref(name);
}

// ---- drawing

const pluralize = (n, one, many = one + 's') => `${n} ${n === 1 ? one : many}`;
const dnum = (n) => String(n + off());

function summaryOf(kind, x) {
  if (kind === 'filter') return filterSummary(x.f);
  const n = x.items.reduce((a, it) => a + it.dsts.length, 0);
  return `${pluralize(x.items.length, 'source')}, ${pluralize(n, 'destination')}`;
}

function matchLines(r) {
  const m = matched(r);
  const lines = [];
  if (m.status === 'same') lines.push(status('ok', 'Numbers and names match this router'));
  const endLine = (what, e, label) => {
    if (e.status === 'byName') return status('info', `${what} “${label}”: matched by name, number ${dnum(e.was)} is ${dnum(e.n)} here`);
    if (e.status === 'ambiguous') return status('warn', `${what} “${label}”: ${e.count} on this router have that name, so it is not remapped`);
    if (e.status === 'unmatched') return status('warn', `${what} ${dnum(e.n)}${label ? ` “${label}”` : ''}: not on this router`);
    return null;
  };
  const seen = new Set();
  for (const it of r.src.items.map(fmt.itemIn).map((x, i) => [x, m.items[i]])) {
    const [item, mm] = it;
    for (const l of [endLine('Source', mm.src, item.srcName), ...mm.dsts.map((d, i) => endLine('Destination', d, item.dsts[i].name))]) {
      if (!l || seen.has(l.textContent)) continue;
      seen.add(l.textContent);
      lines.push(l);
    }
  }
  if (m.count.ambiguous) lines.push(status('warn', 'Ambiguous names are not guessed: this preset is skipped until you change the choice, and then keeps those numbers as written'));
  if (m.levelsOver.length) lines.push(status('warn', `Level ${m.levelsOver.join(', ')} is above this router’s ${S.levels.length}`));
  return lines;
}

const RANK = { warn: 0, error: 0, info: 1, ok: 2, idle: 3 };
const rankOf = (n) => RANK[[...n.firstChild.classList].find((c) => c.startsWith('pb-dot--'))?.slice(8)] ?? 3;
function worstFirst(lines) {
  const sorted = lines.map((n, i) => [n, i]).sort((a, b) => rankOf(a[0]) - rankOf(b[0]) || a[1] - b[1]).map((x) => x[0]);
  // Every warning stays visible; notes (info, ok) fold away, behind the first one when there is no warning.
  const loud = sorted.filter((n) => rankOf(n) === 0);
  const shown = loud.length ? loud : sorted.slice(0, 1);
  const rest = sorted.filter((n) => !shown.includes(n));
  if (!rest.length) return shown;
  return [...shown, el('details', 'sode__rv-more', el('summary', 'sode__hint', `${rest.length} more note${rest.length > 1 ? 's' : ''}`), ...rest.map((n) => el('div', null, n)))];
}

function rowNode(r, i, choices) {
  const sc = scope();
  const cur = (r.kind === 'filter' ? curSets(sc) : curPresets(sc)).find((x) => fmt.conflict(r.name, [x.name]));
  const m = r.kind === 'preset' ? matched(r) : null;
  const incoming = r.kind === 'filter' ? { f: r.f } : { items: m.saved.items };
  const inCell = el('div', 'sode__rv-in', el('div', 'sode__rv-line', el('strong', null, r.name), el('span', 'sode__hint', r.kind === 'filter' ? 'Filter set' : 'Preset'),
    el('span', 'sode__hint', summaryOf(r.kind, incoming))),
    ...worstFirst(r.kind === 'filter' ? [status('info', 'Carried as written')] : matchLines(r)));
  const curCell = el('div', 'sode__rv-cur', cur
    ? el('div', 'sode__rv-line', el('strong', null, cur.name), el('span', 'sode__hint', summaryOf(r.kind, cur)))
    : status('idle', 'Nothing with this name yet'));
  const choice = choices[i];
  const opts = cur ? [['replace', 'Replace'], ['both', 'Keep both'], ['skip', 'Skip']] : [['import', 'Import'], ['skip', 'Skip']];
  const pick = select(`What to do with ${r.name}`, opts, choice, (v) => { r.choice = v; render(); });
  const pickCell = el('div', 'sode__rv-pick', pick);
  if (choice === 'both' && cur) {
    const as = el('input', 'pb-input');
    as.value = r.as || r.autoAs || '';
    as.maxLength = 32;
    as.autocomplete = 'off';
    as.setAttribute('aria-label', `Save ${r.name} as`);
    const prob = el('span', 'sode__hint sode__error-text');
    prob.textContent = r.asProblem || '';
    as.addEventListener('input', () => { r.as = as.value; refresh(); prob.textContent = r.asProblem || ''; });
    pickCell.append(as, prob);
  }
  return el('div', `sode__rv-row${choice === 'skip' ? ' is-skip' : ''}`, inCell, curCell, pickCell);
}

function prefsNode() {
  const list = prefList();
  if (!list.length) return null;
  const sc = prefTarget();
  const opts = forUser() ? [] : [['local', 'This browser']];
  if (profileName() || forUser()) opts.push(['user', forUser() ? `${st.user}’s profile` : 'Me']);
  const rows = list.map((p) => {
    const label = fmt.PREF_LABELS[p.name] || p.name;
    if (!p.ok) {
      return el('div', 'sode__rv-row is-skip', el('div', 'sode__rv-in', el('strong', null, label), status('warn', `“${p.value}” is not a setting this Sode knows, so it is skipped`)), el('div', 'sode__rv-cur'), el('div', 'sode__rv-pick'));
    }
    const now = currentPref(p.name);
    const same = now === p.value;
    const on = st.prefOn[p.name] !== false && !same;
    const pick = select(`Import ${label}`, [['on', 'Import'], ['off', 'Skip']], on ? 'on' : 'off', (v) => { st.prefOn[p.name] = v === 'on'; render(); });
    pick.querySelector('select').disabled = same;
    const valText = (v) => PREFS[p.name].values[v];
    return el('div', `sode__rv-row${on ? '' : ' is-skip'}`,
      el('div', 'sode__rv-in', el('strong', null, label), el('div', 'sode__hint', valText(p.value)), same ? status('ok', 'Already set to this') : status('info', 'Changes this setting')),
      el('div', 'sode__rv-cur', now ? el('div', 'sode__hint', valText(now)) : status('idle', sc === 'user' ? 'Not set in the profile' : 'Following the default')),
      el('div', 'sode__rv-pick', pick));
  });
  const to = select('Settings go to', opts, sc, (v) => { st.prefsTo = v; render(); });
  return el('div', 'sode__rv-sec', el('div', 'sode__filters-set', el('strong', null, 'This device settings'), el('span', 'sode__hint', 'go to'), to),
    el('span', 'sode__hint', 'Everyone’s defaults are set in Settings, not here.'), ...rows);
}

function render() {
  if (!open || !st) return;
  const opts = [['local', 'This browser']];
  if (forUser()) opts.length = 0;
  if (forUser()) opts.push(['user', `${st.user}’s profile`]);
  else {
    if (profileName()) opts.push(['user', 'Me']);
    opts.push(['global', 'Everyone']);
  }
  if (!opts.some((o) => o[0] === st.scope)) st.scope = opts[0][0];
  const choices = st.rows.map(choiceOf);
  const head = el('div', 'sode__rv-head', el('span', 'sode__label', 'Importing'), el('span', 'sode__label', 'Current'), el('span', 'sode__label', 'Do'));
  targetBox.replaceChildren(el('span', 'sode__label', 'Save to'), select('Save to', opts, st.scope, (v) => { st.scope = v; gsave.cancel(); render(); }));
  noteTop.textContent = st.rows.some((r) => r.kind === 'filter')
    ? 'Filter expressions are copied as written. Source and destination numbers in them may differ on this router; check them after.' : '';
  const pn = prefsNode();
  prefsBox.replaceChildren(...(pn ? [pn] : []));
  prefsBox.hidden = !pn;
  rowsBox.replaceChildren(...(st.rows.length ? [head, ...st.rows.map((r, i) => rowNode(r, i, choices))] : [el('span', 'sode__hint', 'Nothing to import in this file.')]));
  nowBtn.hidden = !(st.applyNow && st.rows.length === 1 && st.rows[0].kind === 'filter');
  refresh();
}

function refresh() {
  if (!open || !st) return;
  let p;
  if (forUser() && st.scope === 'user' && fetched?.state !== 'ok') {
    sumEl.textContent = '';
    applyBtn.disabled = true;
    if (!saving()) setNote(fetched?.state === 'err' ? fetched.err : 'Reading their profile…', fetched?.state === 'err');
    return;
  }
  p = plan();
  const n = (k, key) => (p.res[k]?.[key] || 0);
  const parts = [];
  const add = n('filter', 'added') + n('preset', 'added'), rep = n('filter', 'replaced') + n('preset', 'replaced'), skip = n('filter', 'skipped') + n('preset', 'skipped');
  if (add) parts.push(`add ${add}`);
  if (rep) parts.push(`replace ${rep}`);
  if (skip) parts.push(`skip ${skip}`);
  const pn = Object.keys(p.prefs).length;
  if (pn) parts.push(`change ${pluralize(pn, 'setting')}`);
  sumEl.textContent = parts.length ? `Will ${parts.join(', ')}.` : 'Nothing will change.';
  const nothing = !(add + rep + pn);
  applyBtn.disabled = saving() || nothing || p.problems.length > 0;
  if (!saving()) {
    setNote(p.problems[0] || '', true);
    if (p.problems.length > 1) noteEl.textContent += ` (${p.problems.length - 1} more)`;
  }
}

// ---- applying

function summaryText(p) {
  const bits = [];
  const c = (k, key) => p.res[k]?.[key] || 0;
  const nf = c('filter', 'added') + c('filter', 'replaced'), np = c('preset', 'added') + c('preset', 'replaced');
  if (nf) bits.push(pluralize(nf, 'filter set'));
  if (np) bits.push(pluralize(np, 'preset'));
  const ns = Object.keys(p.prefs).length;
  if (ns) bits.push(pluralize(ns, 'setting'));
  return bits.join(', ') || 'Nothing';
}

function finish(p) {
  if (prefTarget() === 'local') for (const [name, v] of Object.entries(p.prefs)) setPref(name, v);
  const where = st.scope === 'user' && forUser() ? `${st.user}’s profile` : scopeLabel(st.scope).toLowerCase();
  const text = `${summaryText(p)} imported to ${where}.`;
  close();
  showResult('ok', 'Import done', text);
}

function apply() {
  if (saving() || applyBtn.disabled) return;
  const p = plan();
  if (p.problems.length) return;
  const sc = scope();
  const done = () => finish(p);
  if (!Object.keys(p.parts).length) return done();
  // Settings going to my profile while the lists go elsewhere are their own save, after the lists.
  const withPrefs = (next) => {
    if (!p.parts.prefs || sc === 'user') return next();
    busy = true;
    refresh();
    setNote('Saving…');
    saveProfile({ prefs: p.parts.prefs }).then((r) => {
      busy = false;
      if (!open) return;
      if (r.ok) return next();
      refresh();
      setNote(sentence(r.err) || 'Settings not saved to your profile.', true);
    });
  };
  if (sc === 'local') {
    if (p.parts.filterSets) putLocalSets(p.res.filter.list);
    if (p.parts.presets) putLocalPresets(p.res.preset.list);
    return withPrefs(done);
  }
  if (sc === 'user') {
    if (forUser()) {
      busy = true;
      refresh();
      setNote('Saving…');
      saveProfile(p.parts, st.user).then((r) => {
        busy = false;
        if (!open) return;
        if (r.ok) return done();
        refresh();
        setNote(r.locked ? 'Only a trusted user or the settings PIN can change another user’s profile.' : sentence(r.err) || 'Not saved.', true);
      });
      return;
    }
    return psave.change(p.parts, done);
  }
  const afterLists = () => withPrefs(done);
  const sendPresets = () => (p.parts.presets ? gsavePresets.change({ presets: p.parts.presets }, afterLists) : afterLists());
  if (p.parts.filterSets) gsave.change({ sets: p.parts.filterSets }, sendPresets);
  else sendPresets();
}

function applyNowClick() {
  const r = st?.rows[0];
  if (!r || r.kind !== 'filter' || !st.applyNow) return;
  const run = st.applyNow;
  close();
  run(r.f, r.name);
}

// ---- open / close

// Options: scope, user (review for that user's profile, saved by name), applyNow (f, name) for one filter set (a link), source (where it came from).
export function openReview({ doc, scope: sc = 'local', user = '', applyNow = null, source = '', title = 'Review import', onLocked = null }) {
  back = document.activeElement;
  st = { doc, scope: sc, user, applyNow, onLocked, rows: makeRows(doc), prefsTo: sc === 'user' ? 'user' : 'local', prefOn: {} };
  fetched = null;
  open = true;
  busy = false;
  titleEl.textContent = title;
  const bits = [doc.site && `From ${doc.site}`, doc.user && `profile of ${doc.user}`, doc.exported && `exported ${doc.exported.slice(0, 10)}`].filter(Boolean);
  fromEl.textContent = source || bits.join(' · ');
  fromEl.hidden = !fromEl.textContent;
  pinA.pinRow.hidden = true;
  pinB.pinRow.hidden = true;
  setNote('');
  dlg.show();
  render();
  dialog.focus();
  if (user && user !== profileName()) {
    st.scope = 'user';
    fetched = { state: 'wait' };
    const mine = st;
    fetchProfile(user).then((r) => {
      if (st !== mine || !open) return;
      fetched = r.ok ? { state: 'ok', profile: r.profile } : { state: 'err', err: r.locked ? 'Only a trusted user or the settings PIN can read another user’s profile.' : sentence(r.err) };
      render();
    });
    render();
  }
}

function close() {
  if (!open) return;
  open = false;
  dlg.hide();
  psave.close();
  gsave.close();
  gsavePresets.close();
  st = null;
  if (back?.isConnected) back.focus();
}

const dlg = trapDialog(root, dialog, () => {
  if (!pinA.pinRow.hidden || !pinB.pinRow.hidden) { gsave.cancel(); gsavePresets.cancel(); return; }
  close();
});

on('locking', () => {
  if (!open) return;
  const again = st?.onLocked;
  close();
  if (again) again();
});
on('profile', () => render());
on('presets', () => render());
on('filters', () => render());
on('helloData', () => render());
