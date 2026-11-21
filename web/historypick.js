// Pick mode for History (the preset editor's "Add from History..."): rows get check boxes, and a bar says what Add does.
// Mode 'boxes' ("Add from Patch Boxes...") uses the same bar and state, and the patch boxes stay live.
// Each layout asks picking()/pickBox() while it builds rows, redraws on 'pick' and calls bindPick(list) one time.
import { el, on, emit, S } from './core.js';
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
    count: (s) => s.note,
    addText: () => 'Add',
    canAdd: () => true,
    add(s) {
      s.note = s.onAddPatch();
      update();
    },
    // Esc still clears a box or closes its list first.
    escCancels: (e) => !(e.target instanceof HTMLInputElement && (e.target.value || e.target.getAttribute('aria-expanded') === 'true')),
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

// opts: { label (the preset's name), ok(e), onAdd(takes), onCancel() };
// mode 'boxes': { label, onAddPatch() (adds the boxes' patch, returns a line
// to show), onCancel() (Done) }.
export function startPick(opts) {
  if (st) return;
  st = { mode: 'history', ...opts, picked: new Set(), follows: false, note: '' };
  followsIn.checked = false;
  document.documentElement.dataset.sodePick = st.mode === 'boxes' ? 'boxes' : '';
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
const controls = el('div', 'sode__pickbar-controls', followsBox, count, addBtn, cancelBtn);
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
    msg.textContent = m.msg(st.label ? `“${st.label}”` : 'the preset');
    sub.textContent = m.sub || '';
    bar.setAttribute('aria-label', m.aria);
    followsBox.hidden = !m.follows;
    cancelBtn.textContent = m.cancelText;
    clearChip();
    count.textContent = m.count(st, t);
    addBtn.textContent = m.addText(st, t);
    addBtn.disabled = !m.canAdd(st, t);
  }
  emit('pick');
}

function clearChip() {
  const chip = document.querySelector('.sode-lock__who');
  const r = chip && !chip.hidden ? chip.getBoundingClientRect() : null;
  bar.style.paddingLeft = r && r.width ? `calc(${Math.ceil(r.right)}px + var(--pb-space-7))` : '';
}
globalThis.addEventListener?.('resize', () => {
  if (st) clearChip();
});

document.addEventListener(
  'keydown',
  (e) => {
    if (!st || e.key !== 'Escape') return;
    if (!MODES[st.mode].escCancels(e)) return;
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
