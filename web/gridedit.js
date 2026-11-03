// The /settings/grids page (#150): make and change grid panels. Pick a
// grid, click or drag over squares, Join or Split them, and pick what each
// button is. Save sends the one panel being edited; with a settings PIN the
// page opens locked, like the Settings dialog. The server checks everything
// and its error text is shown as it is.
import { $, el, S, on, onMsg, connect, connState, setDot, search } from './core.js';
import { levelBox, combobox } from './widgets.js';
import { unlock, unlocked } from './pin.js';
import { globalSaver } from './globalsave.js';
import { allSalvos, salvoNamed } from './salvostore.js';
import { rectBetween, expandRect, cellsIn, meets, join, split, assign, remove, resize, squares, lostByJoin, lostByResize, validLink, freeLink } from './gridgeom.js';
import './device.js';
import './brand.js';

// Each draft grid is a server grid plus: was (its link as last saved, '' if
// never saved) and edited (changes not saved yet).
let draft = [];
let cur = -1; // index in draft
let sel = null; // selected rect
let linkAuto = false; // the link still follows the name (a new grid)

const grid = () => draft[cur];
const copy = (g) => ({ ...g, levels: [...g.levels], cells: g.cells.map((c) => ({ ...c })), was: g.link, edited: false });
let stale = false; // the grids changed while a save was in flight
const anyEdited = () => draft.some((g) => g.edited);

// Reads the grids from the server's list; edits not saved yet stay.
function load(keepEdits = true) {
  const now = grid();
  const kept = keepEdits ? draft.filter((g) => g.edited) : [];
  draft = (S.cfg?.grids || []).map((g) => kept.find((k) => k.was === g.link) || copy(g));
  for (const k of kept) if (!draft.includes(k)) draft.push(k);
  cur = draft.includes(now) ? draft.indexOf(now) : Math.min(Math.max(cur, 0), draft.length - 1);
  sel = null;
  linkAuto = false;
  warn('');
}

// A visible note of assignments an edit dropped (joins keep one, resizes cut cells).
function warn(text) {
  $('ged-warn').hidden = !text;
  $('ged-warn').textContent = text || '';
}
const lostText = (n, how) => (n ? `${n} assigned button${n === 1 ? ' was' : 's were'} dropped ${how}.` : '');
function touch() {
  grid().edited = true;
  $('ged-saved').hidden = true;
  showDirty();
}
function showDirty() {
  $('ged-dirty').textContent = grid()?.edited ? 'Unsaved changes.' : '';
}

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

// ---- saving: one panel at a time, a delete, and the confirm default

// Errors and notes from saves, in the page's two note boxes.
function setNote(text, error = false) {
  $('ged-err').hidden = !text || !error;
  $('ged-err').textContent = error ? text : '';
  $('ged-saved').hidden = !text || error;
  $('ged-saved').textContent = error ? '' : text;
}

const saver = globalSaver({
  t: 'gridSave', resultT: 'gridsResult', pinIn: $('ged-spin'), pinRow: $('ged-pin-row'), pinOk: $('ged-spin-ok'),
  setNote, isOpen: () => true, failTitle: 'Grid not saved', okNote: 'Saved.',
  render: () => {
    $('ged-save').disabled = saver.busy();
    $('ged-delete').disabled = saver.busy();
    $('ged-confirm-default').checked = S.cfg?.gridConfirm !== false;
  },
});
$('ged-spin-cancel').addEventListener('click', saver.cancel);

// What is sent: the grid without the editor's own bookkeeping.
const wire = (g) => ({
  link: g.link, name: g.name.trim(), cols: g.cols, rows: g.rows, confirm: g.confirm, levels: g.levels,
  cells: g.cells.map((c) => ({ x: c.x, y: c.y, w: c.w, h: c.h, kind: c.kind, ref: String(c.ref), label: c.label || '' })),
});

$('ged-save').addEventListener('click', () => {
  const g = grid();
  if (!g) return;
  const payload = { grid: wire(g) };
  if (g.was && g.was !== g.link) payload.was = g.was;
  const sent = JSON.stringify(payload.grid);
  saver.change(payload, () => {
    g.was = payload.grid.link;
    if (JSON.stringify(wire(g)) === sent) g.edited = false;
    if (stale) load(); // others' changes that came while this one was saving
    stale = false;
    draw();
  });
});

$('ged-revert').addEventListener('click', () => { load(false); draw(); setNote(''); });

let deleteArmed = false;
$('ged-delete').addEventListener('click', () => {
  if (!grid()) return;
  if (!deleteArmed) {
    deleteArmed = true;
    $('ged-delete').textContent = 'Really delete this grid?';
    setTimeout(() => { deleteArmed = false; $('ged-delete').textContent = 'Delete grid'; }, 4000);
    return;
  }
  deleteArmed = false;
  $('ged-delete').textContent = 'Delete grid';
  const g = grid();
  const gone = () => {
    draft.splice(draft.indexOf(g), 1);
    cur = Math.min(cur, draft.length - 1);
    sel = null;
    linkAuto = false;
    draw();
  };
  if (!g.was) return gone(); // never saved: nothing on the server
  saver.change({ t: 'gridDelete', link: g.was }, gone);
});

$('ged-confirm-default').addEventListener('change', (e) => {
  const on = e.target.checked;
  saver.change({ t: 'gridConfirm', on }, () => { if (S.cfg) S.cfg.gridConfirm = on; });
});

// ---- grid list and form

function drawList() {
  $('ged-items').replaceChildren(...draft.map((g, i) => {
    const b = el('button', 'pb-btn pb-btn--outline sode-ged__item', `${g.name || g.link || 'Untitled'}${g.edited ? ' (unsaved)' : ''}`);
    b.type = 'button';
    b.setAttribute('aria-pressed', String(i === cur));
    b.addEventListener('click', () => { cur = i; sel = null; linkAuto = false; draw(); });
    return b;
  }));
  $('ged-confirm-default').checked = S.cfg?.gridConfirm !== false;
}

const option = ([v, t]) => Object.assign(el('option', null, t), { value: v });
$('ged-gconfirm').replaceChildren(...[['default', 'Follow the global default'], ['on', 'Confirm (churning letter)'], ['off', 'Send at once']].map(option));
const KINDS = [['', 'Empty'], ['src', 'Source'], ['dst', 'Destination'], ['salvo', 'Salvo'], ['confirm', 'Confirm']];
$('ged-kind').replaceChildren(...KINDS.map(option));

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
  showDirty();
  if (!g) return;
  loading = true;
  $('ged-name').value = g.name;
  $('ged-link').value = g.link;
  $('ged-link').classList.toggle('pb-input--invalid', !validLink(g.link));
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
  draft.push({ link, name: 'New grid', cols: 6, rows: 4, confirm: 'default', levels: [], cells: [], was: '', edited: true });
  cur = draft.length - 1;
  sel = null;
  linkAuto = true;
  draw();
  $('ged-name').select();
});

$('ged-gconfirm').addEventListener('change', (e) => { grid().confirm = e.target.value; touch(); drawList(); });

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
  $('ged-url').replaceChildren('Opens at ', el('code', null, new URL(`grid/${grid().link}`, document.baseURI).pathname));
}
for (const k of ['cols', 'rows']) {
  $(`ged-${k}`).addEventListener('change', (e) => {
    const g = grid();
    const n = Math.max(1, Math.round(Number(e.target.value)) || g[k]);
    g[k] = n;
    e.target.value = n;
    const lost = lostByResize(g.cells, g.cols, g.rows);
    g.cells = resize(g.cells, g.cols, g.rows);
    warn(lostText(lost, `by making the grid ${g.cols} x ${g.rows}`));
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
    const kind = c ? c.kind || 'spacer' : '';
    const n = el('div', `sode-ged__sq${kind ? ` sode-ged__sq--${kind}` : ''}${c && (c.w > 1 || c.h > 1) ? ' sode-ged__sq--joined' : ''}`);
    n.dataset.x = s.x;
    n.dataset.y = s.y;
    n.dataset.w = s.w;
    n.dataset.h = s.h;
    n.style.gridColumn = `${s.x + 1} / span ${s.w}`;
    n.style.gridRow = `${s.y + 1} / span ${s.h}`;
    n.setAttribute('role', 'gridcell');
    const t = cellText(c);
    if (t) n.append(el('b', null, t.name), el('span', null, t.sub));
    return n;
  });
  canvas.append(...nodes);
  paintSel();
  drawTools();
}

const KIND_WORD = { src: 'Source', dst: 'Destination', salvo: 'Salvo', confirm: 'Confirm button' };

// A cell's two lines of text: { name, sub }, or null for an uncovered square.
function cellText(c) {
  if (!c) return null;
  if (!c.kind) return { name: c.w * c.h > 1 ? 'Empty' : '', sub: '' };
  if (c.kind === 'confirm') return { name: c.label || 'Confirm', sub: KIND_WORD.confirm };
  if (c.kind === 'salvo') return { name: c.label || c.ref || 'No salvo', sub: c.ref && !salvoNamed(c.ref) ? 'Salvo (not found here)' : 'Salvo' };
  const it = (c.kind === 'src' ? S.srcBy : S.dstBy).get(+c.ref);
  const num = String(+c.ref + (S.cfg?.numberOffset || 0));
  return { name: c.label || it?.name || num, sub: `${KIND_WORD[c.kind]} ${num}`, picked: c.ref !== '', itemName: it?.name || '' };
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
  warn(lostText(lostByJoin(grid().cells, sel), 'by joining: a joined button keeps only the first assigned one'));
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

let singleKind = '';
const refBox = $('ged-ref');
const cb = combobox(refBox, $('ged-menu'), {
  source: (q) => search(singleKind === 'src' ? S.srcs : S.dsts, q, 8),
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
refBox.addEventListener('input', () => cb.refresh());

// Not rewritten when it already says so, so a caret in it stays put.
const setVal = (box, v) => { if (box.value !== v) box.value = v; };

// The salvo picker: every salvo this browser knows, then the one the button
// names if it isn't among them. Salvos kept in this browser only exist here.
function drawSalvoPick(ref) {
  const pick = $('ged-salvo');
  const list = allSalvos();
  const opts = [['', 'Pick a salvo…'], ...list.map((s) => [s.name, `${s.name} (${s.scope === 'global' ? 'Everyone' : 'This browser only'})`])];
  if (ref && !list.some((s) => s.name.toLowerCase() === ref.toLowerCase())) opts.push([ref, `${ref} (not found)`]);
  pick.replaceChildren(...opts.map(option));
  pick.value = ref || '';
}

function drawCell(one) {
  $('ged-cell').hidden = !one;
  if (!one) return;
  const c = one.cell;
  singleKind = c?.kind || '';
  $('ged-kind').value = singleKind;
  const salvo = singleKind === 'salvo';
  const refKind = singleKind === 'src' || singleKind === 'dst';
  $('ged-ref-box').hidden = !(refKind || salvo);
  $('ged-ref-label').textContent = salvo ? 'Salvo' : singleKind === 'src' ? 'Source' : 'Destination';
  refBox.hidden = salvo;
  $('ged-salvo-wrap').hidden = !salvo;
  refBox.placeholder = 'Search by number or name';
  setVal(refBox, '');
  if (salvo) drawSalvoPick(c?.ref || '');
  const t = refKind ? cellText(c) : null;
  $('ged-ref-now').textContent = salvo ? 'Salvos kept for this browser only are not found on other devices.'
    : !refKind ? '' : t?.picked ? `Now: ${t.sub}${t.itemName ? ` ${t.itemName}` : ''}` : 'Nothing picked yet.';
  setVal($('ged-label'), c?.label || '');
  $('ged-cell-title').textContent = `Button at column ${one.x + 1}, row ${one.y + 1}${c && (c.w > 1 || c.h > 1) ? ` (${c.w} x ${c.h})` : ''}`;
}

$('ged-kind').addEventListener('change', (e) => {
  const t = single();
  if (!t) return;
  grid().cells = assign(grid().cells, t.x, t.y, { kind: e.target.value, ref: '' });
  touch();
  drawCanvas();
});
$('ged-salvo').addEventListener('change', (e) => {
  const t = single();
  if (!t) return;
  grid().cells = assign(grid().cells, t.x, t.y, { ref: e.target.value });
  touch();
  drawCanvasKeep();
});
$('ged-label').addEventListener('input', (e) => {
  const t = single();
  if (!t?.cell) return;
  grid().cells = assign(grid().cells, t.x, t.y, { label: e.target.value });
  touch();
  // In place: the text box keeps focus.
  drawCanvasKeep();
});
function drawCanvasKeep() {
  const keep = sel;
  drawCanvas();
  sel = keep;
  paintSel();
}

// ---- following the server

// The grids changed (another browser's save, or ours): follow, keeping
// edits not saved yet. Our own save's list is taken when its answer comes.
onMsg('grids', (m) => {
  if (!S.cfg) return;
  S.cfg.grids = m.grids;
  S.cfg.gridConfirm = m.gridConfirm;
  if (saver.busy()) stale = true;
  else { load(); draw(); }
});

on('hello', () => {
  showLock();
  load();
  draw();
});
on('locked', showLock);
on('salvos', () => { if (grid()) drawCell(single()); });
on('conn', () => {
  const c = connState();
  setDot($('ged-ws-dot'), $('ged-ws-text'), c.ws);
  if (S.wsState !== 'up') showLock();
});
window.addEventListener('beforeunload', (e) => { if (anyEdited()) e.preventDefault(); });

connect();
