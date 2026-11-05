// Pick mode for Recent: the preset editor's "Add from Recent…" steps aside,
// Recent's rows get a check box instead of their buttons, and a bar says what
// the next click does. Takes and folders are ticked (any number), "Include
// follows" also adds what followed them (changes made elsewhere right after:
// likely router automation), Add hands the takes back, Cancel/Esc drops them.
// A second mode, 'boxes' ("Add from Patch Boxes…"), uses the same bar and state
// but leaves the patch boxes live: the Take button gives way to Add (the boxes'
// patch becomes a row, the boxes clear) and Done; Recent is left alone.
// One state, two layouts: each layout asks picking()/pickBox() while building
// its rows, redraws on the 'pick' event and calls bindPick(list) once.
import { el, on, emit, S } from './core.js';
import { histRows } from './history.js';

// ---- pure

// Every pickable row of histRows' result by its key: 'e<id>' (a take, a
// follower, a merged row's newest) and 'f<group>' (a folder: all its takes).
// entries: what picking it adds; follows: what "Include follows" adds.
export function indexRows(rows) {
  const m = new Map();
  const one = (e) => m.set(`e${e.id}`, { entries: [e], follows: e.follows || [] });
  for (const r of rows) {
    if (r.e) { one(r.e); continue; }
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

// The picked keys as takes [{ src, dst, levels }], oldest first, each entry
// once (a folder and one of its takes together count once). ok(e) says
// whether an entry can be a take (a real source and a level that exists).
export function pickedTakes(index, keys, withFollows, ok = () => true) {
  const seen = new Map();
  for (const k of keys) {
    const x = index.get(k);
    if (!x) continue;
    for (const e of x.entries) seen.set(e.id, e);
    if (withFollows) for (const e of x.follows) seen.set(e.id, e);
  }
  return [...seen.values()].filter((e) => e.dst >= 0 && ok(e)).sort((a, b) => a.id - b.id)
    .map((e) => ({ src: e.src, dst: e.dst, levels: [...e.levels] }));
}

// ---- the two modes

const plural = (n) => `${n} take${n === 1 ? '' : 's'}`;
// What each mode says and does. Recent: tick takes and folders, Add hands
// them back. Boxes: Add turns the patch in the boxes into a row, again and
// again, until Done. count/addText/canAdd take the state and, in Recent, the
// takes ticked now (taken), worked out once per redraw.
export const MODES = {
  recent: {
    msg: (name) => `Choose takes for ${name}`,
    sub: 'Click takes or folders in Recent to select them.',
    aria: 'Choose takes from Recent',
    follows: true,
    cancelText: 'Cancel',
    taken: (s) => pickedNow(s),
    count: (s, t) => `${s.picked.size} chosen · ${plural(t.length)}`,
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

// ---- state

// { mode, label, onAdd(takes), onAddPatch(): note, onCancel(), picked: Set of keys, follows, ok, note }
let st = null;
// Recent's rows get check boxes only in the Recent mode.
export const picking = () => st?.mode === 'recent';
export const boxPicking = () => st?.mode === 'boxes';
export const isPicked = (key) => !!st?.picked.has(key);

const pickedNow = (s) => pickedTakes(indexRows(histRows(S.hist)), s.picked, s.follows, s.ok);

export function togglePick(key) {
  if (!st || !/^[ef]/.test(key)) return;
  if (st.picked.has(key)) st.picked.delete(key); else st.picked.add(key);
  update();
}

// opts: { label (the preset's name), ok(e), onAdd(takes), onCancel() };
// mode 'boxes': { label, onAddPatch() (adds the boxes' patch, returns a line
// to show), onCancel() (Done) }.
export function startPick(opts) {
  if (st) return;
  st = { mode: 'recent', ...opts, picked: new Set(), follows: false, note: '' };
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
// Add (button or Enter in the boxes): whatever the mode adds.
export const addPatch = () => st?.mode === 'boxes' && MODES.boxes.add(st);
const add = () => st && MODES[st.mode].add(st);

// ---- the bar

const msg = el('strong', 'sode__pickbar-msg');
const sub = el('span', 'sode__pickbar-sub');
const count = el('span', 'sode__hint');
count.setAttribute('role', 'status');
const followsIn = el('input', 'pb-check__input');
followsIn.type = 'checkbox';
const followsBox = el('label', 'pb-check', followsIn, el('span', 'pb-check__label', 'Include follows'));
followsBox.title = 'Also add changes made elsewhere right after a chosen take (likely router automation)';
const addBtn = el('button', 'pb-btn pb-btn--ok pb-btn--sm', 'Add');
addBtn.type = 'button';
const cancelBtn = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Cancel');
cancelBtn.type = 'button';
const controls = el('div', 'sode__pickbar-controls', followsBox, count, addBtn, cancelBtn);
const bar = el('div', 'sode__pickbar', el('div', 'sode__pickbar-title', msg, sub), controls);
bar.setAttribute('role', 'region');
bar.setAttribute('aria-label', 'Choose takes from Recent');
bar.hidden = true;
document.body.append(bar);

followsIn.addEventListener('change', () => { if (st) { st.follows = followsIn.checked; update(); } });
addBtn.addEventListener('click', add);
cancelBtn.addEventListener('click', cancel);

function update() {
  bar.hidden = !st;
  if (st) {
    const m = MODES[st.mode];
    const t = m.taken?.(st) || [];
    msg.textContent = m.msg(st.label ? `“${st.label}”` : 'the preset');
    sub.textContent = m.sub;
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

// The lock chip sits at the top left over the bar: start the bar's content after it.
function clearChip() {
  const chip = document.querySelector('.sode-lock__who');
  const r = chip && !chip.hidden ? chip.getBoundingClientRect() : null;
  bar.style.paddingLeft = r && r.width ? `${Math.ceil(r.right) + 16}px` : '';
}
globalThis.addEventListener?.('resize', () => { if (st) clearChip(); });

// Esc cancels, ahead of everything else on the page.
document.addEventListener('keydown', (e) => {
  if (!st || e.key !== 'Escape') return;
  if (!MODES[st.mode].escCancels(e)) return;
  e.preventDefault();
  e.stopImmediatePropagation();
  cancel();
}, true);
// A lock or a dropped link ends it (the page is covered, or the rows may change).
on('locking', () => cancel());
on('wsclose', () => cancel());

// ---- for the layouts

// The check box that stands where a row's buttons are.
export function pickBox(key) {
  const i = el('input', 'pb-check__input');
  i.type = 'checkbox';
  i.checked = isPicked(key);
  i.tabIndex = -1;
  return el('label', 'pb-check sode__pickbox', i, el('span', 'pb-check__label', 'Add'));
}

// While picking, a click on a take or folder ticks it, whatever it hit, but
// the "N takes" disclosure still opens a folder so its takes can be picked
// one by one.
export function bindPick(list) {
  list.addEventListener('click', (e) => {
    if (!picking()) return;
    if (e.target.closest('.sode__hist-toggle')) return;
    const key = e.target.closest('[data-key]')?.dataset.key;
    if (!key || !/^[ef]/.test(key)) return;
    e.preventDefault();
    e.stopPropagation();
    togglePick(key);
  }, true);
}
