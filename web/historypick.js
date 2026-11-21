// Pick mode for History (the preset editor's "Add from History..."): rows get check boxes, and a bar says what Add does.
// Mode 'boxes' ("Add from Patch Boxes...") uses the same bar and state, and the patch boxes stay live.
// Each layout asks picking()/pickBox() while it builds rows, redraws on 'pick' and calls bindPick(list) one time.
import { el, on, emit, S, icon } from './core.js';
import { histRows } from './history.js';

// Keys: 'e<id>' (an entry, or a stacked row's newest) and 'f<group>' (a Group: all its entries).
export function indexRows(rows) {
  const m = new Map();
  const one = (e) => m.set(`e${e.id}`, { entries: [e], follows: e.follows || [] });
  for (const r of rows) {
    if (r.e) {
      one(r.e);
      continue;
    }
    const f = r.f;
    (f.shown || f.entries).forEach(one);
    (f.followers || []).forEach(one);
    m.set(`f${f.group}`, {
      entries: f.entries,
      follows: [...f.entries.flatMap((e) => e.follows || []), ...(f.followers || [])],
    });
  }
  return m;
}

// Picked keys as routes, oldest first, each entry one time; ok(e): a real source and an existing level.
export function pickedEntries(index, keys, withFollows, ok = () => true) {
  const seen = new Map();
  for (const k of keys) {
    const x = index.get(k);
    if (!x) continue;
    for (const e of x.entries) seen.set(e.id, e);
    if (withFollows) for (const e of x.follows) seen.set(e.id, e);
  }
  return [...seen.values()]
    .filter((e) => e.dst >= 0 && ok(e))
    .sort((a, b) => a.id - b.id)
    .map((e) => ({ src: e.src, dst: e.dst, levels: [...e.levels] }));
}

const plural = (n) => `${n} route${n === 1 ? '' : 's'}`;
// count/addText/canAdd get the state and, in History, the routes ticked now (taken), worked out one time per redraw.
export const MODES = {
  history: {
    msg: (name) => `Choose entries for ${name}`,
    aria: 'Choose entries from History',
    follows: true,
    cancelText: 'Cancel',
    taken: (s) => pickedNow(s),
    count: (s, t) => `${s.picked.size} chosen, ${plural(t.length)}`,
    addText: (s, t) => (t.length ? `Add ${t.length}` : 'Add'),
    canAdd: (s, t) => t.length > 0,
    add(s) {
      const t = pickedNow(s);
      if (t.length) finish((x) => x.onAdd(t));
    },
    // Esc always cancels.
    escCancels: () => true,
  },
  boxes: {
    msg: (name) => `Add patches to ${name}`,
    sub: 'Fill Source, Destination and Levels in the boxes, then Add (Enter).',
    aria: 'Add patches from the patch boxes',
    follows: false,
    cancelText: 'Done',
    // The patch boxes' Take reads Add in this mode (desktop.js, mobile.js), so the bar has no Add of its own.
    addShown: false,
    count: (s) => s.note,
    addText: () => 'Add',
    canAdd: () => true,
    add(s) {
      s.note = s.onAddPatch();
      // onAddPatch says "Added N rows" or why it added nothing.
      s.ok = /^Added /.test(s.note);
      update();
    },
    // Esc still clears a box or closes its list first.
    escCancels: (e) => !(e.target instanceof HTMLInputElement && (e.target.value || e.target.getAttribute('aria-expanded') === 'true')),
    // From the Grid popup (gridpopup.js): items are preset items, the routes a click picked.
    addItems(s, items) {
      const r = s.onAddItems(items);
      s.note = r.count;
      update();
      return r;
    },
  },
};

// { mode, label, onAdd(takes), onAddPatch(): note, onCancel(), picked: Set of keys, follows, ok, note }
let st = null;
export const picking = () => st?.mode === 'history';
export const boxPicking = () => st?.mode === 'boxes';
export const isPicked = (key) => !!st?.picked.has(key);

const pickedNow = (s) => pickedEntries(indexRows(histRows(S.hist)), s.picked, s.follows, s.ok);

export function togglePick(key) {
  if (!st || !/^[ef]/.test(key)) return;
  if (st.picked.has(key)) st.picked.delete(key);
  else st.picked.add(key);
  update();
}

// opts: { label (the preset's name), ok(e), onAdd(takes), onCancel() }.
// Mode 'boxes' has { label, onAddPatch(), onAddItems(items), rows(), onCancel() }. onAddPatch adds the boxes' patch
// and returns a line to show. onAddItems answers { ok, rows, text, count }. rows gives the preset's rows as items.
// onCancel is Done.
export function startPick(opts) {
  if (st) return;
  st = { mode: 'history', ...opts, picked: new Set(), follows: false, note: '' };
  followsIn.checked = false;
  document.documentElement.dataset.sodePick = st.mode === 'boxes' ? 'boxes' : '';
  const chip = document.querySelector('.sode-lock__who');
  if (chip) chipSize?.observe(chip);
  update();
}

function finish(fn) {
  const s = st;
  if (!s) return;
  st = null;
  delete document.documentElement.dataset.sodePick;
  update();
  fn?.(s);
}
export const endPick = () => finish(null);
const cancel = () => finish((s) => s.onCancel?.());
export const addPatch = () => st?.mode === 'boxes' && MODES.boxes.add(st);
// The Grid popup's seams: its clicks add here, its Done ends the mode, and its header names the preset.
export const addItems = (items) => (st?.mode === 'boxes' ? MODES.boxes.addItems(st, items) : { ok: false, text: '' });
export const donePick = () => cancel();
// The rows of the preset being edited, as preset items ([] when not adding from the boxes).
export const pickRows = () => (st?.mode === 'boxes' ? st.rows?.() || [] : []);
// The bar's last line after an Add of the boxes' patch, and whether it added: { text, ok }.
export const pickNote = () => ({ text: st?.note || '', ok: !!st?.ok });
export const pickLabel = () => (st?.label ? `“${st.label}”` : 'the preset');
const add = () => st && MODES[st.mode].add(st);

const msg = el('strong', 'sode__pickbar-msg');
const sub = el('span', 'sode__pickbar-sub');
const count = el('span', 'sode__hint');
count.setAttribute('role', 'status');
const followsIn = el('input', 'pb-check__input');
followsIn.type = 'checkbox';
const followsBox = el('label', 'pb-check', followsIn, el('span', 'pb-check__label', 'Include External follow entries'));
followsBox.setAttribute('data-pb-tip', "Adds each chosen entry's External follow entries");
const addBtn = el('button', 'pb-btn pb-btn--ok pb-btn--sm', 'Add');
addBtn.type = 'button';
const cancelBtn = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Cancel');
cancelBtn.type = 'button';
// The bar covers the page's top bar and its Grid button, so it has its own (gridpopup.js opens on 'gridOpen').
const gridBtn = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode__pickbar-grid', icon('grid-3x3'), 'Switch to grid view');
gridBtn.type = 'button';
gridBtn.addEventListener('mousedown', (e) => e.preventDefault());
gridBtn.addEventListener('click', () => emit('gridOpen'));
const controls = el('div', 'sode__pickbar-controls', followsBox, count, addBtn, gridBtn, cancelBtn);
const bar = el('div', 'sode__pickbar', el('div', 'sode__pickbar-title', msg, sub), controls);
bar.setAttribute('role', 'region');
bar.setAttribute('aria-label', 'Choose entries from History');
bar.hidden = true;
document.body.append(bar);
// The phone makes the bar a row of its view (mobilehistory.js).
export const pickBar = bar;

followsIn.addEventListener('change', () => {
  if (st) {
    st.follows = followsIn.checked;
    update();
  }
});
addBtn.addEventListener('click', add);
cancelBtn.addEventListener('click', cancel);

function update() {
  bar.hidden = !st;
  if (st) {
    const m = MODES[st.mode];
    const t = m.taken?.(st) || [];
    msg.textContent = m.msg(pickLabel());
    sub.textContent = m.sub || '';
    bar.setAttribute('aria-label', m.aria);
    followsBox.hidden = !m.follows;
    cancelBtn.textContent = m.cancelText;
    clearChip();
    count.textContent = m.count(st, t);
    addBtn.textContent = m.addText(st, t);
    addBtn.disabled = !m.canAdd(st, t);
    addBtn.hidden = m.addShown === false;
    gridBtn.hidden = st.mode !== 'boxes';
  }
  emit('pick');
}

// The title starts at the page content's left edge and Done ends at its right edge, as the boxes below them. The lock
// chip at the left can push the title further in.
function clearChip() {
  const chip = document.querySelector('.sode-lock__who');
  const r = chip && !chip.hidden ? chip.getBoundingClientRect() : null;
  const page = document.documentElement.dataset.layout === 'phone' ? null : document.getElementById('patch-view')?.getBoundingClientRect();
  const left = page?.width ? Math.floor(page.left) : 0;
  const past = r && r.width ? `${Math.ceil(r.right)}px + var(--pb-space-7)` : '0px';
  bar.style.paddingLeft = left || (r && r.width) ? `max(var(--pb-space-6), ${left}px, ${past})` : '';
  const right = page?.width ? Math.floor(bar.getBoundingClientRect().right - page.right) : 0;
  bar.style.paddingRight = right > 0 ? `max(var(--pb-space-6), ${right}px)` : '';
}
globalThis.addEventListener?.('resize', () => {
  if (st) clearChip();
});
// The chip moves when the Lock goes and grows on a sign-in, both while the bar can show.
document.addEventListener('transitionend', (e) => st && e.target.closest?.('.sode-lock__who') && clearChip());
const chipSize = globalThis.ResizeObserver ? new ResizeObserver(() => st && clearChip()) : null;

document.addEventListener(
  'keydown',
  (e) => {
    if (!st || e.key !== 'Escape') return;
    // While the Grid popup shows, Esc closes the popup only (gridpopup.js).
    if (document.getElementById('grid-pop')?.hidden === false || !MODES[st.mode].escCancels(e)) return;
    e.preventDefault();
    e.stopImmediatePropagation();
    cancel();
  },
  true,
);
on('locking', () => cancel());
on('wsclose', () => cancel());

export function pickBox(key) {
  const i = el('input', 'pb-check__input');
  i.type = 'checkbox';
  i.checked = isPicked(key);
  i.tabIndex = -1;
  return el('label', 'pb-check sode__pickbox', i, el('span', 'pb-check__label', 'Add'));
}

// A click ticks an entry or a Group, whatever it hit. The exception is the "N takes" disclosure, which still opens the Group.
export function bindPick(list) {
  list.addEventListener(
    'click',
    (e) => {
      if (!picking()) return;
      if (e.target.closest('.sode__hist-toggle')) return;
      const key = e.target.closest('[data-key]')?.dataset.key;
      if (!key || !/^[ef]/.test(key)) return;
      e.preventDefault();
      e.stopPropagation();
      togglePick(key);
    },
    true,
  );
}
