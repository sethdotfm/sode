// Recent's Filters dialog, shared by both layouts: which kinds of entry
// show, and filter expressions for sources and destinations, edited as
// rule rows (Visual) or typed (Code). Everything applies as you type and
// is kept per browser (core.js). Named sets (global from Settings, or this
// browser's own) are quick buttons in Recent's header. Share link puts the
// filter in a URL that applies it in another browser; the URL is cleaned
// once read.

import {
  $, el, S, on, showResult, filterState, setFilter, filterCount, FILTER_DEFAULT, normFilter,
  allSets, activeSet, activeRef, adoptFilter, pickSet, saveLocalSet, deleteLocalSet, filterSummary,
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
    err.textContent = ed.error ? `${ed.error}. The last working filter still applies.` : '';
    const list = key === 'src' ? S.srcs : S.dsts;
    const p = parseFilter(ed.text);
    if (p.error) return;
    if (!p.rules.length) { note.textContent = list.length ? `All ${list.length} ${noun}.` : `All ${noun}.`; return; }
    if (!list.length) { note.textContent = 'Names not loaded yet.'; return; }
    const fn = compileFilter(p.rules);
    const hits = list.filter((i) => fn(i.n + off(), i.name));
    const shown = hits.slice(0, 4).map((i) => i.label).join(', ');
    note.textContent = hits.length
      ? `Matches ${hits.length} of ${list.length} ${noun}: ${shown}${hits.length > 4 ? ` (+${hits.length - 4})` : ''}`
      : `Matches none of ${list.length} ${noun}.`;
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
        ...(ed.rows.length ? ed.rows.map(rowNode) : [el('span', 'sode__hint', `No rules: every one of the ${noun}.`)]),
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
    flush() { clearTimeout(timer); if (!parseFilter(ed.text).error) setFilter({ ...filterState(), [key]: ed.text }); },
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
  $('flt-hidden-note').replaceChildren(...(g ? ['Hidden for everyone in Settings: ', el('code', null, g)] : ['None are set in Settings → History.']));
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
  for (const box of document.querySelectorAll('[data-sets]')) {
    box.replaceChildren(...(sets.length < 2 ? [] : sets.map((s) => {
      const on = !!a && setKey(a) === setKey(s);
      const b = el('button', 'pb-chip sode__set', s.name + (on && a.edited ? '*' : ''));
      b.type = 'button';
      b.setAttribute('aria-pressed', String(on));
      if (on && a.edited) b.setAttribute('aria-label', `${s.name} (edited)`);
      b.title = `${s.scope === 'global' ? 'For everyone' : s.scope === 'local' ? 'This browser' : 'Built in'}: ${filterSummary(s.f)}${on && a.edited ? '. Edited; click to go back to it.' : ''}`;
      b.addEventListener('click', () => {
        pickSet(s.scope, s.name);
        if (filtersOpen()) reload();
      });
      return b;
    })));
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
  deleting = false;
  renderSetBar();
  root.hidden = false;
  dialog.focus();
}

function closeFilters() {
  eds.src.flush();
  eds.dst.flush();
  root.hidden = true;
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

// ---- the set bar: pick, save, save as, delete

const setKey = (s) => `${s.scope}:${s.name}`;
let deleting = false; // Delete was pressed once; the next press deletes

function renderSetBar() {
  const a = activeSet();
  const select = $('flt-set');
  const groups = [['all', 'Built in'], ['global', 'For everyone (Settings)'], ['local', 'This browser']];
  const sets = allSets();
  const opts = [];
  if (!a) {
    const o = el('option', null, 'Unsaved filters');
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
  const local = a?.scope === 'local';
  $('flt-set-save').disabled = !(local && a.edited);
  $('flt-set-del').disabled = !local;
  $('flt-set-del').textContent = deleting && local ? `Delete “${a.name}”?` : 'Delete';
  $('flt-set-del').classList.toggle('pb-btn--danger', deleting && local);
  $('flt-set-note').textContent = !a ? 'Save as… names these filters for a quick button.'
    : a.scope === 'global' ? (a.edited ? 'Edited from a set for everyone. Save as… keeps your version in this browser.' : 'Set up for everyone in Settings. Save as… makes your own copy to tweak.')
    : a.scope === 'local' ? (a.edited ? 'Edited. Save updates the set.' : 'Saved in this browser.')
    : '';
}

$('flt-set').addEventListener('change', (e) => {
  const [scope, ...rest] = e.target.value.split(':');
  if (!scope) return;
  deleting = false;
  pickSet(scope, rest.join(':'));
  reload();
});
$('flt-set-save').addEventListener('click', () => {
  const a = activeSet();
  if (a?.scope === 'local') saveLocalSet(a.name);
});
$('flt-set-del').addEventListener('click', () => {
  const a = activeSet();
  if (a?.scope !== 'local') return;
  if (!deleting) { deleting = true; renderSetBar(); return; }
  deleting = false;
  deleteLocalSet(a.name);
  $('flt-set').focus();
});

function askName() {
  const a = activeSet();
  const f = $('flt-set-name');
  f.value = !a || a.scope === 'all' ? '' : a.scope === 'global' ? `${a.name} (mine)` : a.name;
  f.classList.remove('pb-input--invalid');
  $('flt-set-name-row').hidden = false;
  f.select();
}
function closeName() {
  $('flt-set-name-row').hidden = true;
  $('flt-set-saveas').focus();
}
function saveName() {
  const f = $('flt-set-name');
  const name = f.value.trim();
  if (!name || name.toLowerCase() === 'all') {
    f.classList.add('pb-input--invalid');
    $('flt-set-note').textContent = name ? '“All” is built in; pick another name.' : 'Give the set a name.';
    f.focus();
    return;
  }
  eds.src.flush();
  eds.dst.flush();
  saveLocalSet(name.slice(0, 32));
  closeName();
}
$('flt-set-saveas').addEventListener('click', askName);
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
  $('flt-share-status').replaceChildren(dot, ok ? 'Copied' : 'Copy the link above');
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
    showResult('warn', 'Filter link not understood', 'This browser\'s filters are unchanged.');
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
    showResult('ok', 'Filters restored', 'Back to this browser\'s filters from before the link.');
  });
  const keep = () => btn(`Save as “${name}”`, () => {
    saveLocalSet(name);
    showResult('ok', 'Set saved', `“${name}” is a quick button in this browser's Recent.`);
  });
  showResult('info', name ? `Filters “${name}” from a shared link` : 'Filters from a shared link', () => el('span', 'sode__filters-empty',
    'Recent in this browser now shows only what they let through.', name && name.toLowerCase() !== 'all' ? keep() : null, undo()));
}

// ---- when filters hide everything

// A note for an empty Recent whose entries the filters hide, or null.
export function emptyNote(rows) {
  if (rows.length || !S.hist.length) return null;
  const clear = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Clear filters');
  clear.type = 'button';
  clear.addEventListener('click', () => pickSet('all', 'All'));
  const open = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Filters…');
  open.type = 'button';
  open.addEventListener('click', openFilters);
  return el('div', 'sode__filters-empty', el('span', 'sode__hint', 'Nothing loaded matches this browser\'s filters.'), open, clear);
}

on('filters', renderButtons);
on('hello', () => {
  renderButtons();
  if (filtersOpen()) { renderShow(); eds.src.preview(); eds.dst.preview(); }
});
on('names', () => { if (filtersOpen()) { eds.src.preview(); eds.dst.preview(); } });
renderButtons();
