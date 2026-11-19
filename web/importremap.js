// The Import tab's remap step. Each file router has a row with its target. Below it, each file source or destination
// that does not import as is has a row. Matching is in remaprules.js; the server applies the numbers (importReq.names).
// The rows form one list for selection (remaplist.js). Filter number and range terms follow (remapterms.js).

import { el, icon } from './core.js';
import * as M from './remaprules.js';
import * as T from './remaptermrules.js';
import { newSelection } from './remapselectrules.js';
import { LIST_HELP, selectList } from './remaplist.js';
import { TERM_CHOICE, termView } from './remapterms.js';

const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};

const KIND = { source: 'Source', destination: 'Destination' };

// st holds targets (file UID → target UID), rowsBy (file UID → rows) and choices (key → numbers). It holds
// termsBy (file UID → term rows) and terms (term key → keep, names or skip). open is the file UIDs whose Moved
// automatically group is open, and sel is the row selection.
export function remapState(check, old) {
  const rm = check.plan.remap || { routers: [], targets: [] };
  const st = { targets: {}, rowsBy: {}, choices: {}, termsBy: {}, terms: {}, open: new Set(old?.open || []), info: rm, sel: old?.sel || newSelection() };
  for (const r of rm.routers) {
    st.targets[r.uid] = r.target;
    const t = rm.targets.find((x) => x.uid === r.target);
    st.rowsBy[r.uid] = t && !t.noNames ? M.remapRows(r, t) : [];
    st.termsBy[r.uid] = t && !t.noNames ? T.termRows(r, t) : [];
    Object.assign(st.choices, M.newChoices(st.rowsBy[r.uid]));
    Object.assign(st.terms, T.termChoices(st.termsBy[r.uid]));
  }
  if (old) {
    for (const k of Object.keys(st.choices)) if (old.choices[k]) st.choices[k] = [...old.choices[k]];
    for (const rows of Object.values(st.termsBy))
      for (const r of rows) if (old.terms[r.key] && (old.terms[r.key] !== 'names' || r.canConvert)) st.terms[r.key] = old.terms[r.key];
  }
  return st;
}

// The importReq part. routers is sent whole, so the server and the page agree on every target.
export function remapReq(st) {
  const names = {};
  const filters = {};
  for (const rows of Object.values(st.rowsBy)) Object.assign(names, M.namesReq(rows, st.choices));
  for (const r of st.info.routers) if (st.targets[r.uid]) Object.assign(filters, T.filtersReq(r, st.termsBy[r.uid] || [], st.terms));
  return { routers: { ...st.targets }, names, filters };
}

const shown = (n, name, off) => (M.norm(name) ? `${name.trim()} (${n + off})` : String(n + off));

function targetNames(st, uid, kind) {
  const t = st.info.targets.find((x) => x.uid === st.targets[uid]);
  return (kind === 'source' ? t?.sources : t?.destinations) || [];
}

// One row's choice: a select of the quick answers, and Other… for any target name or number. A choice on a selected
// row goes to every selected row (applyChoice).
function picker(st, uid, row, off, onChange) {
  const names = targetNames(st, uid, row.kind);
  const nameOf = (n) => names.find((x) => x.n === n)?.name || '';
  const cur = st.choices[row.key] || [];
  const opts = [['skip', 'Skip']];
  if (row.state === 'blank') opts.push(['keep', `Keep number ${row.n + off}`]);
  for (const n of row.candidates) opts.push([`n:${n}`, shown(n, nameOf(n), off)]);
  if (row.addBoth) opts.push(['all', 'Add both']);
  if (row.suggestion !== undefined) opts.push(['suggest', `Suggestion: ${shown(row.suggestion, nameOf(row.suggestion), off)}`]);
  let value = 'skip';
  if (cur.length > 1) value = 'all';
  else if (cur.length === 1) {
    value = `n:${cur[0]}`;
    if (row.state === 'blank' && cur[0] === row.n) value = 'keep';
    else if (!row.candidates.includes(cur[0]) && cur[0] === row.suggestion) value = 'suggest';
    else if (!opts.some(([v]) => v === value)) opts.push([value, shown(cur[0], nameOf(cur[0]), off)]);
  }
  opts.push(['other', 'Other…']);
  const sel = el('select', 'pb-select');
  for (const [v, t] of opts) {
    const o = el('option', null, t);
    o.value = v;
    sel.append(o);
  }
  sel.value = value;
  sel.dataset.key = `remap:${row.key}`;
  sel.setAttribute('aria-label', `${KIND[row.kind]} ${M.rowLabel(row, off)}`);
  const choose = (v) => {
    M.applyChoice(Object.values(st.rowsBy).flat(), st.choices, row.key, v, st.sel.sel);
    onChange(row.key);
  };
  const other = el('input', 'pb-input sode__xi-other');
  other.setAttribute('list', `xi-names-${uid}-${row.kind}`);
  other.placeholder = 'Name or number';
  other.setAttribute('aria-label', `Target for ${M.rowLabel(row, off)}`);
  other.hidden = true;
  other.addEventListener('change', () => {
    const q = other.value.trim();
    const num = /^(\d+)/.exec(q);
    const hit = num ? names.find((x) => x.n === +num[1] - off) : names.find((x) => M.norm(x.name) === M.norm(q));
    if (hit) choose(`n:${hit.n}`);
  });
  sel.addEventListener('change', () => {
    if (sel.value === 'other') {
      other.hidden = false;
      other.focus();
      return;
    }
    choose(sel.value);
  });
  return el('span', 'sode__xi-pick', el('span', 'pb-select-wrap', sel), other);
}

function nameRow(st, uid, row, off, onChange) {
  const hint = [M.usedBy(row), row.note].filter(Boolean).join('. ');
  const status =
    row.state === 'missing'
      ? 'Not on the target'
      : row.state === 'dup-target'
        ? 'Twice on the target'
        : row.state === 'blank'
          ? 'No name in the file'
          : 'Same name';
  const r = el(
    'div',
    'sode__xi-name',
    el('span', null, el('strong', null, M.rowLabel(row, off)), ' ', el('span', 'sode__hint', `${KIND[row.kind]}, ${status}`)),
    picker(st, uid, row, off, onChange),
    hint ? el('span', 'sode__hint sode__xi-sub', hint) : '',
  );
  r.dataset.row = row.key;
  r.dataset.key = `row:${row.key}`;
  r.setAttribute('role', 'option');
  return r;
}

function datalists(st, uid, off) {
  return ['source', 'destination'].map((kind) => {
    const dl = el('datalist');
    dl.id = `xi-names-${uid}-${kind}`;
    for (const x of targetNames(st, uid, kind)) {
      const o = el('option');
      o.value = `${x.n + off} ${x.name}`.trim();
      dl.append(o);
    }
    return dl;
  });
}

// Use all suggestions, Keep numbers for all blank names, Reset to Skip: on the selected rows, else on every row that
// needs a choice.
function bulkBar(st, onChange) {
  const run = (action) => () => {
    const keys = st.sel.sel.size ? st.sel.sel : null;
    for (const rows of Object.values(st.rowsBy)) M.bulk(rows, st.choices, action, keys);
    onChange('');
  };
  const b = (id, text, action) => Object.assign(btn('pb-btn--outline pb-btn--sm', text, run(action)), { id });
  const count = el('span', 'sode__hint sode__xi-count');
  count.id = 'xi-selected';
  const help = el('span', 'sode__xi-help pb-tip--end', icon('circle-help'));
  help.tabIndex = 0;
  help.setAttribute('aria-label', 'Keys for the list');
  help.setAttribute('data-pb-tip', `${LIST_HELP} With rows selected, these buttons change only those rows.`);
  return el(
    'div',
    'sode__xi-bulk',
    b('xi-bulk-suggest', 'Use all suggestions', 'suggest'),
    b('xi-bulk-keep', 'Keep numbers for all blank names', 'keepBlank'),
    b('xi-bulk-skip', 'Reset to Skip', 'skip'),
    count,
    help,
  );
}

// opts: off() the shown offset, onChange(key) after a name choice, onRouter() after a target change.
export function remapView(st, { off, onChange, onRouter, fmt }) {
  const rm = st.info;
  if (!rm.routers.length) return [];
  const o = off();
  const list = el('div', 'sode__xi-remap');
  for (const r of rm.routers) {
    const sel = el('select', 'pb-select');
    sel.id = `xi-router-${r.uid}`;
    sel.dataset.key = `router:${r.uid}`;
    for (const x of M.routerOptions(r, st.targets)) {
      const op = el('option', null, x.text);
      op.value = x.value;
      op.disabled = x.disabled;
      sel.append(op);
    }
    sel.value = st.targets[r.uid] || '';
    sel.addEventListener('change', () => {
      st.targets[r.uid] = sel.value;
      onRouter();
    });
    const label = Object.assign(el('label', null, el('strong', null, r.title || r.uid)), { htmlFor: sel.id });
    list.append(el('div', 'sode__xi-router', label, el('span', 'pb-select-wrap', sel)));
    const rows = st.rowsBy[r.uid] || [];
    const target = rm.targets.find((x) => x.uid === st.targets[r.uid]);
    if (!target) continue;
    const note = M.namesNote(target, fmt);
    if (note) list.append(el('p', `pb-note${target.noNames ? ' pb-note--warn' : ''} sode__xi-offline`, note));
    const need = rows.filter(M.needsChoice);
    const moved = rows.filter((x) => x.state === 'moved');
    const asIs = rows.length - need.length - moved.length;
    const box = el('div', 'sode__xi-names');
    box.append(...need.map((x) => nameRow(st, r.uid, x, o, onChange)));
    if (moved.length) {
      const open = st.open.has(r.uid);
      const caret = btn('pb-btn--ghost pb-btn--sm sode__xi-moved', `${open ? '▾' : '▸'} Moved automatically (${moved.length})`, () => {
        if (open) st.open.delete(r.uid);
        else st.open.add(r.uid);
        onChange('');
      });
      caret.dataset.key = `moved:${r.uid}`;
      caret.setAttribute('aria-expanded', String(open));
      box.append(caret);
      if (open) box.append(...moved.map((x) => nameRow(st, r.uid, x, o, onChange)));
    }
    if (asIs) box.append(el('p', 'sode__hint', `${asIs} ${asIs === 1 ? 'name imports' : 'names import'} as is.`));
    list.append(box, ...termView(r, st.termsBy[r.uid] || [], st.terms, () => onChange('')), ...datalists(st, r.uid, o));
  }
  const any = Object.values(st.rowsBy).some((rows) => rows.some((x) => x.state !== 'as-is'));
  const bar = any ? bulkBar(st, onChange) : '';
  const count = bar && bar.querySelector('.sode__xi-count');
  selectList(list, st.sel, (n) => {
    if (count) count.textContent = n ? `${n} selected` : '';
  });
  return [el('h3', 'sode__label sode__xi-h', 'Router names'), bar, list];
}

const linkTo = (text, fn) => el('li', null, btn('pb-btn--ghost pb-btn--sm', text, fn));

// The review: per router pair what the names do, the changed filter terms and the notes, each linked to its row.
export function remapReview(st, goTo) {
  const out = [];
  for (const r of st.info.routers) {
    const to = st.info.targets.find((x) => x.uid === st.targets[r.uid]);
    if (!to) {
      out.push(el('li', null, `${r.title || r.uid}: no router. Its items arrive Broken.`));
      continue;
    }
    const rows = st.rowsBy[r.uid] || [];
    const s = M.remapSummary(rows, st.choices);
    const parts = [`${s.asIs} matched as is`, `${s.auto} moved automatically`, `${s.hand} chosen by hand`, `${s.skipped} skipped`];
    const li = el('li', null, `${r.title || r.uid} to ${to.title}: ${parts.join(', ')}.`);
    const links = [];
    const ch = T.termChanges(r, st.termsBy[r.uid] || [], st.terms);
    for (const t of ch.terms)
      links.push(
        linkTo(`${t.side === 'src' ? 'Sources' : 'Destinations'} ${t.label} in ${t.sets.join(', ')}: ${TERM_CHOICE[t.choice]}`, () => goTo(r.uid, t.key)),
      );
    for (const name of ch.open) links.push(el('li', 'sode__hint', `${name}: no number or name term is left on one side, so that side matches all.`));
    for (const x of rows.filter((x) => x.note && (st.choices[x.key] || []).length)) links.push(linkTo(`${x.name.trim()}: ${x.note}`, () => goTo(r.uid, x.key)));
    if (links.length) li.append(el('ul', 'sode__xi-list', ...links));
    out.push(li);
  }
  if (!out.length) return [];
  return [el('h3', 'sode__label sode__xi-h', 'Router names'), el('ul', 'sode__xi-list sode__xi-pairs', ...out)];
}

// The row a Broken item's missing reference comes from, as [file UID, key], or null.
export function rowOf(st, m) {
  for (const [uid, rows] of Object.entries(st.rowsBy)) {
    if (st.targets[uid] !== m.ref?.router) continue;
    const r = rows.find((x) => x.kind === m.kind && x.n === m.ref.n);
    if (r) return [uid, r.key];
  }
  return null;
}
