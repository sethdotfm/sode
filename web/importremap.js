// The Import tab's remap step. Each file router has a row with its target. Below it, each file source or destination
// that does not import as is has a row. Matching is in remaprules.js; the server applies the numbers (importReq.names).

import { el } from './core.js';
import * as M from './remaprules.js';

const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};

const KIND = { source: 'Source', destination: 'Destination' };

// st: { targets: file UID → target UID, rowsBy: file UID → rows, choices: key → numbers, open: Set of file UIDs whose
// Moved automatically group is open }.
export function remapState(check, old) {
  const rm = check.plan.remap || { routers: [], targets: [] };
  const st = { targets: {}, rowsBy: {}, choices: {}, open: new Set(old?.open || []), info: rm };
  for (const r of rm.routers) {
    st.targets[r.uid] = r.target;
    const t = rm.targets.find((x) => x.uid === r.target);
    st.rowsBy[r.uid] = t ? M.remapRows(r, t) : [];
    Object.assign(st.choices, M.newChoices(st.rowsBy[r.uid]));
  }
  if (old) for (const k of Object.keys(st.choices)) if (old.choices[k]) st.choices[k] = [...old.choices[k]];
  return st;
}

// The importReq part. routers is sent whole, so the server and the page agree on every target.
export function remapReq(st) {
  const names = {};
  for (const rows of Object.values(st.rowsBy)) Object.assign(names, M.namesReq(rows, st.choices));
  return { routers: { ...st.targets }, names };
}

const shown = (n, name, off) => (M.norm(name) ? `${name.trim()} (${n + off})` : String(n + off));

function targetNames(st, uid, kind) {
  const t = st.info.targets.find((x) => x.uid === st.targets[uid]);
  return (kind === 'source' ? t?.sources : t?.destinations) || [];
}

// One row's choice: a select of the quick answers, and Other… for any target name or number.
function picker(st, uid, row, off, onChange) {
  const names = targetNames(st, uid, row.kind);
  const nameOf = (n) => names.find((x) => x.n === n)?.name || '';
  const cur = st.choices[row.key] || [];
  const opts = [['skip', 'Skip']];
  if (row.state === 'blank') opts.push([`n:${row.n}`, `Keep number ${row.n + off}`]);
  for (const n of row.candidates) opts.push([`n:${n}`, shown(n, nameOf(n), off)]);
  if (row.addBoth) opts.push([`all`, `Add both`]);
  if (row.suggestion !== undefined) opts.push([`n:${row.suggestion}`, `Suggestion: ${shown(row.suggestion, nameOf(row.suggestion), off)}`]);
  let value = 'skip';
  if (cur.length > 1) value = 'all';
  else if (cur.length === 1) {
    value = `n:${cur[0]}`;
    if (!opts.some(([v]) => v === value)) opts.push([value, shown(cur[0], nameOf(cur[0]), off)]);
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
  const other = el('input', 'pb-input sode__xi-other');
  other.setAttribute('list', `xi-names-${uid}-${row.kind}`);
  other.placeholder = 'Name or number';
  other.setAttribute('aria-label', `Target for ${M.rowLabel(row, off)}`);
  other.hidden = true;
  other.addEventListener('change', () => {
    const q = other.value.trim();
    const num = /^(\d+)/.exec(q);
    const hit = num ? names.find((x) => x.n === +num[1] - off) : names.find((x) => M.norm(x.name) === M.norm(q));
    if (!hit) return;
    st.choices[row.key] = [hit.n];
    onChange(row.key);
  });
  sel.addEventListener('change', () => {
    if (sel.value === 'other') {
      other.hidden = false;
      other.focus();
      return;
    }
    st.choices[row.key] = sel.value === 'skip' ? [] : sel.value === 'all' ? [...row.candidates] : [+sel.value.slice(2)];
    onChange(row.key);
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

// opts: off() the shown offset, onChange(key) after a name choice, onRouter() after a target change.
export function remapView(st, { off, onChange, onRouter }) {
  const rm = st.info;
  if (!rm.routers.length) return [];
  const o = off();
  const out = [el('h3', 'sode__label sode__xi-h', 'Router names')];
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
    out.push(el('div', 'sode__xi-router', label, el('span', 'pb-select-wrap', sel)));
    const rows = st.rowsBy[r.uid] || [];
    if (!st.targets[r.uid]) continue;
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
    out.push(box, ...datalists(st, r.uid, o));
  }
  return out;
}

// The review: per router pair what the names do, and the notes, each linked to its row.
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
    const notes = rows.filter((x) => x.note && (st.choices[x.key] || []).length);
    if (notes.length)
      li.append(
        el(
          'ul',
          'sode__xi-list',
          ...notes.map((x) =>
            el(
              'li',
              null,
              btn('pb-btn--ghost pb-btn--sm', `${x.name.trim()}: ${x.note}`, () => goTo(r.uid, x.key)),
            ),
          ),
        ),
      );
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
