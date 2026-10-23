// Recent's Filters dialog, shared by both layouts: which kinds of entry
// show, and filter expressions for sources and destinations, edited as
// rule rows (Visual) or typed (Code). Everything applies as you type and
// is kept per browser (core.js). Named sets (global from Settings, or this
// browser's own) are quick buttons in Recent's header. Share link puts the
// filter in a URL that applies it in another browser; the URL is cleaned
// once read.

import {
  $, el, S, on, wsSend, showResult, filterState, setFilter, filterCount, FILTER_DEFAULT, normFilter,
  allSets, activeSet, activeRef, adoptFilter, pickSet, saveLocalSet, deleteLocalSet, renameLocalSet, globalSets, filterSummary,
} from './core.js';
import { parseFilter, filterText, compileFilter } from './filter.js';

const root = $('filters');
const dialog = $('filters-dialog');
const MODE_KEY = 'sode-filter-mode';
let mode = 'visual';
try { if (localStorage.getItem(MODE_KEY) === 'code') mode = 'code'; } catch (e) {}

const KINDS = [
  ['name', 'Name contains'],
  ['num', 'Number is'],
  ['range', 'Number between'],
  ['re', 'Name matches regex'],
];
const off = () => (S.cfg ? S.cfg.numberOffset : 0);

// Whether a visual row says enough to be a rule.
function complete(r) {
  if (r.kind === 'num') return Number.isInteger(r.a);
  if (r.kind === 'range') return Number.isInteger(r.a) && Number.isInteger(r.b);
  if (r.kind === 're') {
    if (!r.text) return false;
    try { new RegExp(r.text, 'i'); return true; } catch (e) { return false; }
  }
  return !!r.text;
}

// One expression editor: key is 'src' or 'dst'.
function editor(key) {
  const noun = key === 'src' ? 'sources' : 'destinations';
  const box = $(`flt-${key}`);
  const ed = { text: '', rows: [], error: null };
  let timer = 0;

  // The expression changed: apply it if it parses, and redraw the preview.
  function changed(now = false) {
    const p = parseFilter(ed.text);
    ed.error = p.error ? `${p.error}${p.at != null ? ` at “${ed.text.slice(p.at, p.at + 12)}”` : ''}` : null;
    clearTimeout(timer);
    const go = () => {
      if (!ed.error) setFilter({ ...filterState(), [key]: ed.text });
      preview();
    };
    if (now) go();
    else timer = setTimeout(go, 150);
    if (ed.error) preview();
  }

  function preview() {
    const note = box.querySelector('.sode__filters-preview');
    const err = box.querySelector('.pb-note');
    const input = box.querySelector('.sode__filters-code');
    input?.classList.toggle('pb-input--invalid', !!ed.error);
    err.hidden = !ed.error;
    err.textContent = ed.error ? `${ed.error}` : '';
    const list = key === 'src' ? S.srcs : S.dsts;
    const p = parseFilter(ed.text);
    if (p.error) return;
    if (!p.rules.length) { note.textContent = `All ${noun}`; return; }
    if (!list.length) { note.textContent = ''; return; }
    const fn = compileFilter(p.rules);
    const hits = list.filter((i) => fn(i.n + off(), i.name));
    const shown = hits.slice(0, 4).map((i) => i.label).join(', ');
    note.textContent = hits.length
      ? `${hits.length} of ${list.length}: ${shown}${hits.length > 4 ? ', …' : ''}`
      : `No ${noun} match`;
  }

  // Rows → text, keeping rows still being filled in out of it.
  function fromRows() {
    ed.text = filterText(ed.rows.filter(complete));
    changed();
  }

  function rowNode(r, i) {
    const sel = (opts, value, label, onChange) => {
      const s = el('select', 'pb-select');
      for (const [v, t] of opts) {
        const o = el('option', null, t);
        o.value = v;
        s.append(o);
      }
      s.value = value;
      s.setAttribute('aria-label', label);
      s.addEventListener('change', () => onChange(s.value));
      return el('span', 'pb-select-wrap', s);
    };
    const input = (value, label, onInput, numeric) => {
      const f = el('input', 'pb-input');
      f.value = value ?? '';
      f.spellcheck = false;
      f.autocomplete = 'off';
      if (numeric) f.inputMode = 'numeric';
      f.setAttribute('aria-label', label);
      f.addEventListener('input', () => {
        onInput(f.value);
        for (const x of f.closest('.sode__filters-rule').querySelectorAll('input')) {
          x.classList.toggle('pb-input--invalid', x.value.trim() !== '' && !complete(r));
        }
        fromRows();
      });
      return f;
    };
    const num = (v) => (/^\s*\d+\s*$/.test(v) ? Number(v) : NaN);
    const n = i + 1;
    const values = r.kind === 'range'
      ? [input(r.a, `Rule ${n} from`, (v) => { r.a = num(v); }, true), el('span', 'sode__hint', 'to'),
        input(r.b, `Rule ${n} to`, (v) => { r.b = num(v); }, true)]
      : r.kind === 'num' ? [input(r.a, `Rule ${n} number`, (v) => { r.a = num(v); }, true)]
      : [input(r.text, `Rule ${n} ${r.kind === 're' ? 'regex' : 'text'}`, (v) => { r.text = v; })];
    const remove = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Remove');
    remove.type = 'button';
    remove.setAttribute('aria-label', `Remove rule ${n}`);
    remove.addEventListener('click', () => {
      ed.rows.splice(i, 1);
      fromRows();
      render();
      box.querySelector('.sode__filters-add')?.focus();
    });
    return el('div', 'sode__filters-rule',
      sel([['in', 'Include'], ['out', 'Exclude']], r.not ? 'out' : 'in', `Rule ${n}: include or exclude`, (v) => { r.not = v === 'out'; fromRows(); }),
      sel(KINDS, r.kind, `Rule ${n}: what to match`, (v) => {
        // Keep what was typed where it still makes sense.
        if ((v === 'num' || v === 'range') !== (r.kind === 'num' || r.kind === 'range')) Object.assign(r, { a: undefined, b: undefined, text: '' });
        r.kind = v;
        fromRows();
        render();
        box.querySelectorAll('.sode__filters-rule')[i]?.querySelector('input')?.focus();
      }),
      el('span', 'sode__filters-vals', ...values),
      remove);
  }

  function render() {
    const head = el('div', 'sode__filters-ed-head', el('span', 'sode__label', key === 'src' ? 'Sources' : 'Destinations'));
    let body;
    if (mode === 'code') {
      const f = el('input', 'pb-input sode__code sode__filters-code');
      f.value = ed.text;
      f.placeholder = key === 'src' ? 'CAM, 1-12, !Black' : 'DSM, /^MV\\d+ PIP/, !65';
      f.spellcheck = false;
      f.autocomplete = 'off';
      f.setAttribute('aria-label', `${key === 'src' ? 'Sources' : 'Destinations'} filter`);
      f.addEventListener('input', () => { ed.text = f.value; changed(); });
      body = f;
    } else {
      const add = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode__filters-add', '+ Add rule');
      add.type = 'button';
      add.addEventListener('click', () => {
        ed.rows.push({ not: false, kind: 'name', text: '' });
        render();
        const rules = box.querySelectorAll('.sode__filters-rule');
        rules[rules.length - 1]?.querySelector('input')?.focus();
      });
      body = el('div', 'sode__filters-rules',
        ...ed.rows.map(rowNode),
        el('div', null, add));
    }
    const err = el('div', 'pb-note pb-note--error');
    err.setAttribute('role', 'alert');
    err.hidden = true;
    box.replaceChildren(head, body, el('div', 'sode__hint sode__filters-preview'), err);
    preview();
  }

  return {
    // From the saved filter (opening, Clear all).
    load(text) {
      ed.text = text;
      const p = parseFilter(text);
      ed.rows = p.rules ? p.rules.map((r) => ({ ...r })) : [];
      ed.error = null;
      render();
      if (p.error) changed(true);
    },
    // Switching modes; false if the code doesn't parse, so can't be rows.
    toVisual() {
      const p = parseFilter(ed.text);
      if (p.error) { changed(true); return false; }
      ed.rows = p.rules.map((r) => ({ ...r }));
      return true;
    },
    render,
    preview,
    flush() {
      clearTimeout(timer);
      if (!ed.error) setFilter({ ...filterState(), [key]: ed.text });
      preview();
    },
  };
}

const eds = { src: editor('src'), dst: editor('dst') };

function renderMode() {
  $('flt-visual').setAttribute('aria-pressed', String(mode === 'visual'));
  $('flt-code').setAttribute('aria-pressed', String(mode === 'code'));
  $('flt-syntax').hidden = mode !== 'code';
}
function setMode(m) {
  if (m === mode) return;
  if (m === 'visual' && !['src', 'dst'].map((k) => eds[k].toVisual()).every(Boolean)) return;
  mode = m;
  try { localStorage.setItem(MODE_KEY, m); } catch (e) {}
  renderMode();
  eds.src.render();
  eds.dst.render();
}

function renderShow() {
  const f = filterState();
  $('flt-sode').checked = f.sode;
  $('flt-ext').checked = f.ext;
  $('flt-hidden').checked = f.hidden;
  $('flt-hidden').disabled = !f.ext;
  const g = (S.cfg?.hiddenDsts || '').trim();
  $('flt-hidden-note').replaceChildren(...(g ? [el('code', null, g)] : ['None set in Settings']));
}

// ---- the Filters buttons in Recent's headers

function renderButtons() {
  const n = filterCount();
  for (const b of document.querySelectorAll('[data-filters]')) {
    b.textContent = n ? `Filters · ${n}` : 'Filters';
    b.classList.toggle('sode__filters-btn--on', n > 0);
    b.setAttribute('aria-label', n ? `Filters, ${n} active` : 'Filters, none active');
  }
  // Quick buttons for the sets, once there's any besides All. The active
  // one is selected; * when its filter has been edited since.
  const sets = allSets();
  const a = activeSet();
  const chip = (s) => {
    const on = !!a && setKey(a) === setKey(s);
    const b = el('button', 'pb-chip sode__set', s.name + (on && a.edited ? '*' : ''));
    b.type = 'button';
    b.setAttribute('aria-pressed', String(on));
    if (on && a.edited) b.setAttribute('aria-label', `${s.name} (edited)`);
    b.title = filterSummary(s.f) + (on && a.edited ? ' (edited)' : '');
    b.addEventListener('click', () => {
      pickSet(s.scope, s.name);
      if (filtersOpen()) reload();
    });
    return b;
  };
  // All | for everyone | this browser's own, each group labelled for
  // screen readers.
  const groups = [['all', 'Default'], ['global', 'Everyone'], ['local', 'This browser']]
    .map(([scope, label]) => [label, sets.filter((s) => s.scope === scope)]).filter(([, g]) => g.length);
  for (const box of document.querySelectorAll('[data-sets]')) {
    const kids = [];
    if (sets.length > 1) {
      for (const [label, g] of groups) {
        if (kids.length) {
          const bar = el('span', 'sode__sets-sep', '|');
          bar.setAttribute('aria-hidden', 'true');
          kids.push(bar);
        }
        const grp = el('span', 'sode__sets-group', ...g.map(chip));
        grp.setAttribute('role', 'group');
        grp.setAttribute('aria-label', label);
        kids.push(grp);
      }
    }
    box.replaceChildren(...kids);
  }
  if (filtersOpen()) renderSetBar();
}

// ---- open / close

let back = null;
export const filtersOpen = () => !root.hidden;

export function openFilters() {
  back = document.activeElement;
  const f = filterState();
  renderMode();
  renderShow();
  eds.src.load(f.src);
  eds.dst.load(f.dst);
  $('flt-share').hidden = true;
  $('flt-set-name-row').hidden = true;
  $('flt-pin-row').hidden = true;
  naming = null;
  deleting = false;
  renderSetBar();
  root.hidden = false;
  dialog.focus();
}

function closeFilters() {
  eds.src.flush();
  eds.dst.flush();
  root.hidden = true;
  // An unlock for sets for everyone lasts while the dialog is open.
  if (unlocked) wsSend({ t: 'lock' });
  unlocked = false;
  pending = null;
  $('flt-pin-row').hidden = true;
  if (back?.isConnected) back.focus();
}

for (const b of document.querySelectorAll('[data-filters]')) b.addEventListener('click', openFilters);
$('flt-done').addEventListener('click', closeFilters);
root.addEventListener('click', (e) => { if (e.target === root) closeFilters(); });
$('flt-visual').addEventListener('click', () => setMode('visual'));
$('flt-code').addEventListener('click', () => setMode('code'));
for (const k of ['sode', 'ext', 'hidden']) {
  $(`flt-${k}`).addEventListener('change', (e) => {
    setFilter({ ...filterState(), [k]: e.target.checked });
    renderShow();
  });
}
// Loads the editors from the filter now (after a set is picked).
function reload() {
  const f = filterState();
  renderShow();
  eds.src.load(f.src);
  eds.dst.load(f.dst);
  $('flt-share').hidden = true;
}

$('flt-clear').addEventListener('click', () => {
  pickSet('all', 'All');
  reload();
});

// ---- the set bar: pick, save, save as, rename, delete. Works the same on
// this browser's sets and on sets for everyone; those are settings, saved
// on the server, so with a settings PIN they ask for it first.

const setKey = (s) => `${s.scope}:${s.name}`;
let deleting = false; // Delete was pressed once; the next press deletes
let naming = null;    // 'saveas' | 'rename' while the name row is open
let pending = null;   // a change to sets for everyone: { id, run, done }
let unlocked = false; // the PIN was accepted on this connection
let pinSeq = 1e6;     // unlock ids, apart from Settings' own
let opSeq = 0;

function renderSetBar() {
  const a = activeSet();
  const select = $('flt-set');
  const groups = [['all', 'Default'], ['global', 'Everyone'], ['local', 'This browser']];
  const sets = allSets();
  const opts = [];
  if (!a) {
    const o = el('option', null, 'Unsaved');
    o.value = '';
    opts.push(o);
  }
  for (const [scope, label] of groups) {
    const these = sets.filter((s) => s.scope === scope);
    if (!these.length) continue;
    const g = el('optgroup');
    g.label = label;
    for (const s of these) {
      const o = el('option', null, s.name + (a && setKey(a) === setKey(s) && a.edited ? ' (edited)' : ''));
      o.value = setKey(s);
      g.append(o);
    }
    opts.push(g);
  }
  select.replaceChildren(...opts);
  select.value = a ? setKey(a) : '';
  const named = a && a.scope !== 'all';
  const busy = !!pending;
  $('flt-set-save').disabled = busy || !(named && a.edited);
  $('flt-set-saveas').disabled = busy;
  $('flt-set-rename').disabled = busy || !named;
  $('flt-set-del').disabled = busy || !named;
  $('flt-set-del').textContent = deleting && named ? `Delete “${a.name}”?` : 'Delete';
  $('flt-set-del').classList.toggle('pb-btn--danger', deleting && named);
  if (busy) return;
  setNote(a && a.scope !== 'all' ? `${a.scope === 'global' ? 'Everyone' : 'This browser'}${a.edited ? ' · edited' : ''}` : '');
}
const setNote = (text, error = false) => {
  $('flt-set-note').textContent = text;
  $('flt-set-note').classList.toggle('sode__error-text', error);
};

// ---- sets for everyone: send the whole list, after the PIN if needed

function changeGlobal(list, done) {
  pending = { run: () => send(list), done };
  if (S.pinSet && !unlocked) return askPin();
  pending.run();
}
function send(list) {
  pending.id = ++opSeq;
  if (!wsSend({ t: 'filterSets', id: pending.id, sets: list.map((s) => ({ name: s.name, ...s.f })) })) {
    pending = null;
    renderSetBar();
    setNote('Not connected to Sode.', true);
    return;
  }
  renderSetBar();
  setNote('Saving…');
}
const globalsWith = (fn) => fn(globalSets().map((s) => ({ name: s.name, f: s.f })));

on('filterSetsResult', (m) => {
  if (!pending || m.id !== pending.id) return;
  const p = pending;
  if (m.locked) { unlocked = false; return askPin('Enter the PIN again.'); }
  pending = null;
  if (!m.ok) { renderSetBar(); setNote(sentence(m.err), true); return; }
  p.done();
  renderSetBar();
});

function askPin(err = '') {
  $('flt-pin-row').hidden = false;
  $('flt-pin').value = '';
  setNote(err || 'Enter the settings PIN.', !!err);
  $('flt-pin').focus();
}
function cancelPin() {
  $('flt-pin-row').hidden = true;
  pending = null;
  renderSetBar();
}
function sendPin() {
  if (!wsSend({ t: 'unlock', pin: $('flt-pin').value, id: ++pinSeq })) return setNote('Not connected to Sode.', true);
  $('flt-pin-ok').disabled = true;
}
on('unlockResult', (m) => {
  if (m.id !== pinSeq) return;
  $('flt-pin-ok').disabled = false;
  if (!m.ok) { setNote(sentence(m.err), true); $('flt-pin').select(); return; }
  unlocked = true;
  $('flt-pin-row').hidden = true;
  pending?.run();
});
on('locked', () => { unlocked = false; });
on('conn', () => { if (S.wsState !== 'up') unlocked = false; });
const sentence = (t) => {
  const s = String(t || '').trim();
  return s && s[0].toUpperCase() + s.slice(1) + (/[.!?…]$/.test(s) ? '' : '.');
};
$('flt-pin-ok').addEventListener('click', sendPin);
$('flt-pin-cancel').addEventListener('click', cancelPin);
$('flt-pin').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') { e.preventDefault(); sendPin(); }
  if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); cancelPin(); }
});

// ---- the buttons

$('flt-set').addEventListener('change', (e) => {
  const [scope, ...rest] = e.target.value.split(':');
  if (!scope) return;
  deleting = false;
  pickSet(scope, rest.join(':'));
  reload();
});

$('flt-set-save').addEventListener('click', () => {
  const a = activeSet();
  if (!a || a.scope === 'all') return;
  flushEds();
  if (a.scope === 'local') return saveLocalSet(a.name);
  const f = filterState();
  changeGlobal(globalsWith((l) => l.map((s) => (s.name === a.name ? { ...s, f } : s))),
    () => adoptFilter(f, { scope: 'global', name: a.name, base: f }));
});

$('flt-set-del').addEventListener('click', () => {
  const a = activeSet();
  if (!a || a.scope === 'all') return;
  if (!deleting) { deleting = true; renderSetBar(); return; }
  deleting = false;
  if (a.scope === 'local') {
    deleteLocalSet(a.name);
    $('flt-set').focus();
    return;
  }
  changeGlobal(globalsWith((l) => l.filter((s) => s.name !== a.name)), () => {
    adoptFilter(filterState());
    $('flt-set').focus();
  });
});

function askName(mode) {
  const a = activeSet();
  naming = mode;
  const f = $('flt-set-name');
  const named = a && a.scope !== 'all';
  f.value = mode === 'rename' ? a.name : !named ? '' : a.scope === 'global' ? `${a.name} (mine)` : `${a.name} 2`;
  f.classList.remove('pb-input--invalid');
  $('flt-set-scope-wrap').hidden = mode === 'rename';
  $('flt-set-scope').value = 'local';
  $('flt-set-name-ok').textContent = mode === 'rename' ? 'Rename' : 'Save set';
  $('flt-set-name-row').hidden = false;
  f.select();
}
function closeName() {
  $('flt-set-name-row').hidden = true;
  $(naming === 'rename' ? 'flt-set-rename' : 'flt-set-saveas').focus();
  naming = null;
}
function saveName() {
  const f = $('flt-set-name');
  const name = f.value.trim().slice(0, 32);
  const a = activeSet();
  const scope = naming === 'rename' ? a.scope : $('flt-set-scope').value;
  const bad = (msg) => { f.classList.add('pb-input--invalid'); setNote(msg, true); f.focus(); };
  if (!name) return bad('Name required.');
  if (name.toLowerCase() === 'all') return bad('“All” is taken.');
  const clash = allSets().find((s) => s.scope === scope && s.name.toLowerCase() === name.toLowerCase());
  const self = naming === 'rename' && clash && setKey(clash) === setKey(a);
  if (clash && !self && naming === 'rename') return bad(`“${clash.name}” already exists.`);
  flushEds();
  const mode = naming;
  closeName();
  if (mode === 'rename') {
    if (scope === 'local') return renameLocalSet(a.name, name);
    return changeGlobal(globalsWith((l) => l.map((s) => (s.name === a.name ? { ...s, name } : s))),
      () => adoptFilter(filterState(), { ...activeRef(), name }));
  }
  // Save as: a name that's taken (in that scope) is replaced.
  if (scope === 'local') return saveLocalSet(name);
  const fNow = filterState();
  changeGlobal(globalsWith((l) => [...l.filter((s) => s.name.toLowerCase() !== name.toLowerCase()), { name, f: fNow }]),
    () => adoptFilter(fNow, { scope: 'global', name, base: fNow }));
}
const flushEds = () => { eds.src.flush(); eds.dst.flush(); };
$('flt-set-saveas').addEventListener('click', () => askName('saveas'));
$('flt-set-rename').addEventListener('click', () => askName('rename'));
$('flt-set-name-ok').addEventListener('click', saveName);
$('flt-set-name-cancel').addEventListener('click', closeName);
$('flt-set-name').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') { e.preventDefault(); saveName(); }
  if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closeName(); }
});

// Keys stay in the dialog: Esc closes, Tab wraps. Stopped here so the
// layouts' own keys don't act behind it.
root.addEventListener('keydown', (e) => {
  e.stopPropagation();
  if (e.key === 'Escape') { e.preventDefault(); closeFilters(); return; }
  if (e.key !== 'Tab') return;
  const all = [...dialog.querySelectorAll('button, input, select')].filter((x) => !x.disabled && x.offsetParent);
  const first = all[0], last = all[all.length - 1];
  if (e.shiftKey && (document.activeElement === first || document.activeElement === dialog)) { e.preventDefault(); last.focus(); }
  else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
});

// ---- share links: ?f=<base64url of the filter's non-default parts>

const b64 = (s) => btoa(String.fromCharCode(...new TextEncoder().encode(s))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
const unb64 = (s) => new TextDecoder().decode(Uint8Array.from(atob(s.replace(/-/g, '+').replace(/_/g, '/')), (c) => c.charCodeAt(0)));

$('flt-share-btn').addEventListener('click', async () => {
  eds.src.flush();
  eds.dst.flush();
  const f = filterState();
  const out = {};
  for (const k of Object.keys(FILTER_DEFAULT)) if (f[k] !== FILTER_DEFAULT[k]) out[k] = f[k];
  const a = activeSet();
  if (a && a.scope !== 'all') out.name = a.name;
  const url = `${location.origin}${location.pathname}?f=${b64(JSON.stringify(out))}`;
  $('flt-share').hidden = false;
  $('flt-share-url').value = url;
  $('flt-share-url').select();
  let ok = false;
  try { await navigator.clipboard.writeText(url); ok = true; } catch (e) {}
  const dot = el('span', `pb-dot pb-dot--${ok ? 'ok' : 'info'}`);
  dot.setAttribute('aria-hidden', 'true');
  $('flt-share-status').replaceChildren(dot, ok ? 'Copied' : 'Copy it above');
});

// A shared link: apply its filter in this browser, then clean the URL.
export function applySharedLink() {
  const params = new URLSearchParams(location.search);
  const raw = params.get('f');
  if (raw == null) return;
  params.delete('f');
  const q = params.toString();
  history.replaceState(null, '', location.pathname + (q ? `?${q}` : '') + location.hash);
  let next, name;
  try {
    const raw2 = JSON.parse(unb64(raw));
    next = normFilter(raw2);
    name = typeof raw2.name === 'string' ? raw2.name.trim().slice(0, 32) : '';
  } catch (e) {
    showResult('warn', 'Invalid filter link', '');
    return;
  }
  const prev = filterState(), prevRef = activeRef();
  adoptFilter(next);
  const btn = (text, fn) => {
    const b = el('button', 'pb-btn pb-btn--outline pb-btn--sm', text);
    b.type = 'button';
    b.addEventListener('click', fn);
    return b;
  };
  const undo = () => btn('Undo', () => {
    adoptFilter(prev, prevRef);
    showResult('ok', 'Filters restored', '');
  });
  const keep = () => btn('Save as set', () => {
    saveLocalSet(name);
    showResult('ok', `Saved “${name}”`, '');
  });
  showResult('info', name ? `Shared filters “${name}” applied` : 'Shared filters applied', () => el('span', 'sode__filters-empty',
    name && name.toLowerCase() !== 'all' ? keep() : null, undo()));
}

// ---- when filters hide everything

// A note for an empty Recent whose entries the filters hide, or null.
export function emptyNote(rows) {
  if (rows.length || !S.hist.length) return null;
  const clear = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Clear filters');
  clear.type = 'button';
  clear.addEventListener('click', () => pickSet('all', 'All'));
  return el('div', 'sode__filters-empty', el('span', 'sode__hint', 'Nothing matches.'), clear);
}

on('filters', renderButtons);
on('hello', () => {
  renderButtons();
  if (filtersOpen()) { renderShow(); eds.src.preview(); eds.dst.preview(); }
});
on('names', () => { if (filtersOpen()) { eds.src.preview(); eds.dst.preview(); } });
renderButtons();
