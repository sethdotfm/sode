// The /settings/grids page (#150): make and change grid panels. Pick a
// grid, click or drag over squares, Join or Split them, and pick what each
// button is. Save sends all grids at once; with a settings PIN the page
// opens locked, like the Settings dialog.
import { $, el, S, on, onMsg, connect, connState, setDot, wsSend, search, sentence } from './core.js';
import { levelBox, combobox } from './widgets.js';
import { unlock, unlocked } from './pin.js';
import { MAX_SIZE, rectBetween, expandRect, cellsIn, meets, join, split, assign, remove, resize, savable, squares, validLink, freeLink } from './gridgeom.js';
import './device.js';
import './brand.js';

let draft = []; // grids being edited; cells include unassigned joined ones
let confirmDefault = true;
let cur = -1; // index in draft
let sel = null; // selected rect
let dirty = false;
let linkAuto = false; // the link still follows the name (a new grid)
let saving = 0; // id of the save in flight

const grid = () => draft[cur];
const copy = (g) => ({ ...g, levels: [...g.levels], cells: g.cells.map((c) => ({ ...c })) });

function load() {
  draft = (S.cfg?.grids || []).map(copy);
  confirmDefault = S.cfg?.gridConfirm !== false;
  cur = Math.min(Math.max(cur, 0), draft.length - 1);
  sel = null;
  setDirty(false);
}

function setDirty(d) {
  dirty = d;
  $('ged-dirty').textContent = d ? 'Unsaved changes.' : '';
}
const touch = () => { setDirty(true); $('ged-saved').hidden = true; };

// ---- lock

function showLock() {
  const locked = !!S.cfg && S.pinSet && !unlocked();
  $('ged-lock').hidden = !locked;
  $('ged-lock').dataset.shown = locked ? 'yes' : '';
  $('ged').hidden = locked || !S.cfg;
  if (locked) $('ged-pin').focus();
}

$('ged-lock').addEventListener('submit', async (e) => {
  e.preventDefault();
  $('ged-unlock').disabled = true;
  const r = await unlock($('ged-pin').value);
  $('ged-unlock').disabled = false;
  $('ged-pin').value = '';
  if (r.superseded) return;
  $('ged-lock-err').hidden = r.ok || !r.err;
  $('ged-lock-err').textContent = r.err || '';
  if (r.ok) showLock();
});

// ---- grid list and form

function drawList() {
  $('ged-items').replaceChildren(...draft.map((g, i) => {
    const b = el('button', 'pb-btn pb-btn--outline sode-ged__item', `${g.name || g.link || 'Untitled'}`);
    b.type = 'button';
    b.setAttribute('aria-pressed', String(i === cur));
    b.addEventListener('click', () => { cur = i; sel = null; draw(); });
    return b;
  }));
  $('ged-confirm-default').checked = confirmDefault;
}

const modes = [['default', 'Follow the global default'], ['on', 'Confirm (churning letter)'], ['off', 'Send at once']];
$('ged-gconfirm').replaceChildren(...modes.map(([v, t]) => { const o = el('option', null, t); o.value = v; return o; }));
$('ged-kind').replaceChildren(...[['', 'Empty'], ['src', 'Source'], ['dst', 'Destination'], ['salvo', 'Salvo']].map(([v, t]) => { const o = el('option', null, t); o.value = v; return o; }));

let loading = false; // filling the form: its events aren't edits
const lvl = levelBox($('ged-lvl'), $('ged-lvl-pick'), $('ged-lvl-all'), {
  hold: 'pointerdown',
  placeholder: () => 'default levels',
  onChange: () => {
    if (loading || !grid()) return;
    grid().levels = lvl.empty() ? [] : (lvl.levels() || []);
    touch();
  },
});

function drawForm() {
  const g = grid();
  $('ged-none').hidden = !!g;
  $('ged-form').hidden = !g;
  if (!g) return;
  loading = true;
  $('ged-name').value = g.name;
  $('ged-link').value = g.link;
  $('ged-cols').value = g.cols;
  $('ged-rows').value = g.rows;
  $('ged-gconfirm').value = g.confirm;
  lvl.reset();
  if (g.levels.length) lvl.set(g.levels);
  else lvl.clear();
  loading = false;
  drawUrl();
}

function draw() {
  drawList();
  drawForm();
  drawCanvas();
}

$('ged-new').addEventListener('click', () => {
  const link = freeLink('grid', draft.map((g) => g.link));
  draft.push({ link, name: 'New grid', cols: 6, rows: 4, confirm: 'default', levels: [], cells: [] });
  cur = draft.length - 1;
  sel = null;
  linkAuto = true;
  touch();
  draw();
  $('ged-name').select();
});

let deleteArmed = false;
$('ged-delete').addEventListener('click', () => {
  if (!deleteArmed) {
    deleteArmed = true;
    $('ged-delete').textContent = 'Really delete this grid?';
    setTimeout(() => { deleteArmed = false; $('ged-delete').textContent = 'Delete grid'; }, 4000);
    return;
  }
  deleteArmed = false;
  $('ged-delete').textContent = 'Delete grid';
  draft.splice(cur, 1);
  cur = Math.min(cur, draft.length - 1);
  sel = null;
  touch();
  draw();
});

$('ged-confirm-default').addEventListener('change', (e) => { confirmDefault = e.target.checked; touch(); });
$('ged-gconfirm').addEventListener('change', (e) => { grid().confirm = e.target.value; touch(); });

$('ged-name').addEventListener('input', (e) => {
  grid().name = e.target.value;
  if (linkAuto) {
    grid().link = freeLink(e.target.value, draft.filter((g) => g !== grid()).map((g) => g.link));
    $('ged-link').value = grid().link;
  }
  touch();
  drawList();
  drawUrl();
});
$('ged-link').addEventListener('input', (e) => {
  linkAuto = false;
  grid().link = e.target.value.trim().toLowerCase();
  e.target.classList.toggle('pb-input--invalid', !validLink(grid().link));
  touch();
  drawUrl();
});
function drawUrl() {
  $('ged-url').replaceChildren('Opens at ', el('code', null, new URL(`../grid/${grid().link}`, location.href).pathname));
}
for (const k of ['cols', 'rows']) {
  $(`ged-${k}`).addEventListener('change', (e) => {
    const g = grid();
    const n = Math.min(MAX_SIZE, Math.max(1, Math.round(Number(e.target.value)) || g[k]));
    g[k] = n;
    e.target.value = n;
    g.cells = resize(g.cells, g.cols, g.rows);
    sel = null;
    touch();
    drawCanvas();
  });
}

// ---- the canvas: click or drag to select squares

const canvas = $('ged-canvas');
let drag = null; // { from: {x, y} }

const squareAt = (t) => {
  const n = t?.closest?.('[data-x]');
  return n && canvas.contains(n) ? { x: +n.dataset.x, y: +n.dataset.y } : null;
};

function paintSel() {
  for (const n of canvas.children) {
    n.classList.toggle('sode-ged__sq--sel', !!sel && meets(sel, { x: +n.dataset.x, y: +n.dataset.y, w: +n.dataset.w, h: +n.dataset.h }));
  }
}

function drawCanvas() {
  const g = grid();
  canvas.replaceChildren();
  if (!g) return drawTools();
  canvas.style.setProperty('--sode-grid-cols', g.cols);
  canvas.style.gridTemplateRows = `repeat(${g.rows}, 4rem)`;
  const nodes = squares(g.cells, g.cols, g.rows).map((s) => {
    const c = s.cell;
    const kind = c?.kind || '';
    const n = el('div', `sode-ged__sq${kind ? ` sode-ged__sq--${kind}` : ''}${c && (c.w > 1 || c.h > 1) ? ' sode-ged__sq--joined' : ''}`);
    n.dataset.x = s.x;
    n.dataset.y = s.y;
    n.dataset.w = s.w;
    n.dataset.h = s.h;
    n.style.gridColumn = `${s.x + 1} / span ${s.w}`;
    n.style.gridRow = `${s.y + 1} / span ${s.h}`;
    n.setAttribute('role', 'gridcell');
    n.append(...cellText(c));
    return n;
  });
  canvas.append(...nodes);
  paintSel();
  drawTools();
}

const KIND_WORD = { src: 'Source', dst: 'Destination', salvo: 'Salvo' };

function cellText(c) {
  if (!c) return [];
  if (!c.kind || c.ref === '') return [el('span', null, c.w * c.h > 1 ? 'Unassigned' : '')];
  if (c.kind === 'salvo') return [el('b', null, c.label || c.ref), el('span', null, 'Salvo')];
  const it = c.kind === 'src' ? S.srcBy.get(+c.ref) : S.dstBy.get(+c.ref);
  const num = String(+c.ref + (S.cfg?.numberOffset || 0));
  return [el('b', null, c.label || it?.name || num), el('span', null, `${KIND_WORD[c.kind]} ${num}`)];
}

canvas.addEventListener('pointerdown', (e) => {
  const at = squareAt(e.target);
  if (!at || e.button) return;
  e.preventDefault();
  drag = { from: at };
  sel = expandRect(grid().cells, rectBetween(at, at));
  paintSel();
});
canvas.addEventListener('pointermove', (e) => {
  if (!drag) return;
  const at = squareAt(document.elementFromPoint(e.clientX, e.clientY));
  if (!at) return;
  sel = expandRect(grid().cells, rectBetween(drag.from, at));
  paintSel();
});
const endDrag = () => {
  if (!drag) return;
  drag = null;
  drawTools();
};
window.addEventListener('pointerup', endDrag);
window.addEventListener('pointercancel', endDrag);

// ---- tools: Join / Split / Clear and the selected button's form

const single = () => {
  if (!sel) return null;
  const inSel = cellsIn(grid().cells, sel);
  if (inSel.length === 1 && inSel[0].x === sel.x && inSel[0].y === sel.y && inSel[0].w === sel.w && inSel[0].h === sel.h) return { x: sel.x, y: sel.y, cell: inSel[0] };
  if (!inSel.length && sel.w === 1 && sel.h === 1) return { x: sel.x, y: sel.y, cell: null };
  return null;
};

function drawTools() {
  const g = grid();
  const inSel = g && sel ? cellsIn(g.cells, sel) : [];
  const area = sel ? sel.w * sel.h : 0;
  const one = g && sel ? single() : null;
  $('ged-join').disabled = !sel || area < 2 || !!(one && one.cell);
  $('ged-split').disabled = !inSel.some((c) => c.w > 1 || c.h > 1);
  $('ged-clear').disabled = !inSel.length;
  $('ged-sel').textContent = !sel ? 'Nothing selected.'
    : `${sel.w} x ${sel.h} selected${inSel.length > 1 ? `, ${inSel.length} buttons` : ''}.`;
  drawCell(one);
}

$('ged-join').addEventListener('click', () => {
  const r = join(grid().cells, sel);
  grid().cells = r.cells;
  sel = r.rect;
  touch();
  drawCanvas();
});
$('ged-split').addEventListener('click', () => {
  grid().cells = split(grid().cells, sel);
  sel = expandRect(grid().cells, sel);
  touch();
  drawCanvas();
});
$('ged-clear').addEventListener('click', () => {
  grid().cells = remove(grid().cells, sel);
  touch();
  drawCanvas();
});

const items = () => (single_kind === 'src' ? S.srcs : S.dsts);
let single_kind = '';
const refBox = $('ged-ref');
const cb = combobox(refBox, $('ged-menu'), {
  source: (q) => search(items(), q, 8),
  onPick: (it) => {
    const t = single();
    if (!t) return;
    grid().cells = assign(grid().cells, t.x, t.y, { ref: String(it.n) });
    refBox.value = '';
    cb.close();
    touch();
    drawCanvas();
  },
});
refBox.addEventListener('blur', () => cb.close());
refBox.addEventListener('keydown', (e) => {
  if (cb.nav(e)) return;
  if (e.key === 'Enter') { e.preventDefault(); cb.accept(); } else if (e.key === 'Escape' && cb.open) { e.preventDefault(); cb.close(); }
});

// Not rewritten when it already says so, so a caret in it stays put.
const setVal = (box, v) => { if (box.value !== v) box.value = v; };

function drawCell(one) {
  $('ged-cell').hidden = !one;
  if (!one) return;
  const c = one.cell;
  single_kind = c?.kind || '';
  $('ged-kind').value = single_kind;
  const salvo = single_kind === 'salvo';
  $('ged-ref-box').hidden = !single_kind;
  $('ged-ref-label').textContent = salvo ? 'Salvo name' : single_kind === 'src' ? 'Source' : 'Destination';
  refBox.placeholder = salvo ? 'Salvos arrive in a later version' : 'Search by number or name';
  setVal(refBox, salvo ? (c?.ref || '') : '');
  $('ged-ref-now').textContent = !single_kind || salvo ? '' : c?.ref !== undefined && c.ref !== ''
    ? `Now: ${cellText(c)[1].textContent}${S.cfg ? ` ${(c.kind === 'src' ? S.srcBy : S.dstBy).get(+c.ref)?.name || ''}` : ''}`
    : 'Nothing picked yet.';
  setVal($('ged-label'), c?.label || '');
  $('ged-cell-title').textContent = `Button at column ${one.x + 1}, row ${one.y + 1}${c && (c.w > 1 || c.h > 1) ? ` (${c.w} x ${c.h})` : ''}`;
}

$('ged-kind').addEventListener('change', (e) => {
  const t = single();
  if (!t) return;
  const kind = e.target.value;
  grid().cells = assign(grid().cells, t.x, t.y, { kind, ref: '' });
  // Unassigned single squares aren't kept.
  grid().cells = grid().cells.filter((c) => c.kind || c.w * c.h > 1);
  touch();
  drawCanvas();
});
$('ged-label').addEventListener('input', (e) => {
  const t = single();
  if (!t?.cell) return;
  grid().cells = assign(grid().cells, t.x, t.y, { label: e.target.value });
  touch();
  // In place: the text box keeps focus.
  drawCanvasKeep();
});
// Salvo names are typed, not searched.
refBox.addEventListener('input', () => {
  if (single_kind !== 'salvo') return cb.refresh();
  const t = single();
  if (!t?.cell) return;
  cb.close();
  grid().cells = assign(grid().cells, t.x, t.y, { ref: refBox.value });
  touch();
  drawCanvasKeep();
});
function drawCanvasKeep() {
  const keep = sel;
  drawCanvas();
  sel = keep;
  paintSel();
}

// ---- save

function problem() {
  const links = new Set();
  for (const g of draft) {
    if (!validLink(g.link)) return `The link "${g.link}" (grid "${g.name}") may only have lowercase letters, digits and single dashes, and can't be one of ws, vendor, icon, manifest.`;
    if (links.has(g.link)) return `Two grids have the link "${g.link}".`;
    links.add(g.link);
    if (!g.name.trim()) return 'Every grid needs a name.';
  }
  return '';
}

let seq = 0;
$('ged-save').addEventListener('click', () => {
  const err = problem();
  $('ged-err').hidden = !err;
  $('ged-err').textContent = err;
  if (err) return;
  const grids = draft.map((g) => ({ ...g, name: g.name.trim(), cells: savable(g.cells).map((c) => ({ ...c, ref: String(c.ref) })) }));
  saving = ++seq;
  $('ged-save').disabled = true;
  if (!wsSend({ t: 'grids', grids, gridConfirm: confirmDefault, id: saving })) {
    saving = 0;
    $('ged-save').disabled = false;
    $('ged-err').hidden = false;
    $('ged-err').textContent = 'Not saved: the connection to Sode was lost. Try again in a moment.';
  }
});
$('ged-revert').addEventListener('click', () => { load(); draw(); $('ged-err').hidden = true; });

onMsg('gridsResult', (m) => {
  if (m.id !== saving) return;
  saving = 0;
  $('ged-save').disabled = false;
  if (m.ok) {
    $('ged-err').hidden = true;
    const dropped = draft.reduce((n, g) => n + g.cells.length, 0) - draft.reduce((n, g) => n + savable(g.cells).length, 0);
    $('ged-saved').hidden = false;
    $('ged-saved').textContent = `Saved.${dropped ? ` ${dropped} unassigned joined button${dropped === 1 ? ' was' : 's were'} not kept.` : ''}`;
    setDirty(false);
    return;
  }
  $('ged-err').hidden = false;
  $('ged-err').textContent = m.locked ? 'Settings are locked; enter the PIN again.' : sentence(m.err);
  if (m.locked) showLock();
});

// The grids changed (another browser's save, or ours, which arrives while
// the edits still count as unsaved and so is left alone): follow unless there
// are unsaved edits here.
onMsg('grids', (m) => {
  if (!S.cfg) return;
  S.cfg.grids = m.grids;
  S.cfg.gridConfirm = m.gridConfirm;
  if (!dirty) { load(); draw(); }
});

on('hello', () => {
  showLock();
  if (!dirty) { load(); draw(); } else { lvl.reset(); }
});
on('locked', showLock);
on('conn', () => {
  const c = connState();
  setDot($('ged-ws-dot'), $('ged-ws-text'), c.ws);
  if (S.wsState !== 'up') showLock();
});
window.addEventListener('beforeunload', (e) => { if (dirty) e.preventDefault(); });

connect();
