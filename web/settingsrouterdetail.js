// Settings > Routers: the routers as one list (settingsrouterlist.js draws it) and the detail view of the selected
// router. Every router has the same settings in the same place (docs/agents/settings-saves.md). The detail view is one
// set of inputs in index.html (cfg-rt-<attribute>). A pick of another router fills them with the text of that router.
// So each router keeps its unsaved edits. A save source of the one Save (savesources.js) for every router key
// router/<uid>/<attribute>, Add router (router/new:<n>/add) and Remove router (router/<uid>/remove). Rules in
// routerdetailrules.js.

import { $, S, el, emit, on, icon, request, linkList, routerData } from './core.js';
import { fmtTime } from './device.js';
import { addSaveSource } from './savesources.js';
import { diffRows } from './savereviewrules.js';
import { namesTimeText } from './settingsrules.js';
import { rereadNames } from './rereadnames.js';
import { everyHold } from './protects.js';
import { levelColor } from './levelbox.js';
import { LIST_KEY, ORDER_KEY, addKey, freePrefix, keyAttr, keyRouter, linkState, listKey, removeKey, routerDisplay, usesNote } from './routerlistrules.js';
import {
  ATTR_LABELS,
  DETAIL_ATTRS,
  NEW_ROUTER,
  RECHECK,
  addText,
  defaultNote,
  fieldErrors,
  levelLabel,
  numberingRange,
  offsetNote,
  protectsText,
  readValues,
  renumberDefault,
  groupNote,
  textsOf,
  tiesText,
  valueText,
} from './routerdetailrules.js';

const MODES = ['1', '0', 'va'];
// The most level chips the detail view draws.
const CHIPS_MAX = 64;

let admin = false;
// One item per router, in list order: the routers of the hello, then the routers to add. text: the input text of each
// setting. removed: Remove on Save. uses: the routerUses answer of a removal, note: what the remove note shows.
let items = [];
let base = {}; // {key: value} as loaded or saved
let vers = {};
let sel = ''; // the UID of the selected router (new:<n> for a router to add)
let addSeq = 0;
const shownErr = new Set(); // `${uid}/${attr}`: errors that show, after the field was left once

const inputOf = (attr) => (attr === 'levelNumbering' ? null : $(`cfg-rt-${attr}`));
const radio = (mode) => $(`cfg-rt-lvlnum-${mode}`);
const itemOf = (uid) => items.find((x) => x.uid === uid) || null;
const selected = () => itemOf(sel);
// The first router as loaded: the default router now, the router of a blank routerData UID.
const isFirst = (x) => !x.isNew && x.uid === S.routers[0]?.uid;
// The default router after the save: the first router of the list that stays.
const defaultAfter = () => items.find((x) => !x.isNew && !x.removed) || null;
// The routers that stay after the save, new ones too.
const staying = () => items.filter((x) => !x.removed);
const loadedOrder = () => items.filter((x) => !x.isNew).map((x) => x.uid);
const valuesOf = (x) => readValues(x.text);
// The shown name: prefix and Router title with two or more routers, else the title.
export const many = () => items.length > 1;
const nameOf = (x) => (many() ? routerDisplay(valuesOf(x)) : valuesOf(x).title || 'Router');
// The name of a key's router as loaded, for the review: a rename shows under the old name.
function loadedName(uid) {
  const r = S.routers.find((x) => x.uid === uid);
  if (!r) return 'The new router';
  return S.routers.length > 1 ? routerDisplay(r) : r.title || 'Router';
}

// The shown name of a router now, by UID.
const nameOfUid = (uid) => {
  const x = itemOf(uid);
  return x ? nameOf(x) : loadedName(uid);
};

// The routerData UID: blank for the first router.
const dataUid = (x) => (isFirst(x) ? '' : x.uid);
const linkOf = (x) => (x.isNew ? null : linkList().find((r) => r.uid === x.uid) || null);
export const linkOfRouter = (x) => {
  const r = linkOf(x);
  return linkState(r, !r?.never);
};

const otherPrefixes = (x) => items.filter((y) => y !== x && !y.removed).map((y) => valuesOf(y).prefix);
export const errorsOf = (x) => (x.removed ? {} : fieldErrors(valuesOf(x), otherPrefixes(x)));

function now() {
  const out = {};
  for (const x of items) {
    if (x.isNew) {
      out[addKey(x.n)] = valuesOf(x);
      continue;
    }
    const v = valuesOf(x);
    // A staged removal sends no edits of that router.
    for (const a of DETAIL_ATTRS) out[listKey(x.uid, a)] = x.removed ? base[listKey(x.uid, a)] : v[a];
    out[removeKey(x.uid)] = x.removed;
  }
  // The order holds the routers that Sode has. A new router comes after them; a removed one keeps its place until
  // the save removes it.
  if (base[ORDER_KEY]) out[ORDER_KEY] = loadedOrder();
  return out;
}
const edited = () => items.some((x) => x.isNew) || diffRows(base, now()).length > 0;
// An edited router: a setting differs from the loaded one, or it has another place in the order.
export const routerEdited = (x) =>
  !x.isNew &&
  (loadedOrder().indexOf(x.uid) !== (base[ORDER_KEY] || []).indexOf(x.uid) ||
    DETAIL_ATTRS.some((a) => JSON.stringify(valuesOf(x)[a] ?? null) !== JSON.stringify(base[listKey(x.uid, a)] ?? null)));

// routerItems: the routers in list order, for the list (settingsrouterlist.js), each {uid, isNew, removed, first, text}.
// first: the default router after the save, which the list marks Default.
export const routerItems = () => items.map((x) => ({ ...x, first: x === defaultAfter() }));

// moveRouter moves router uid by step places among the routers that Sode has (a new router stays last). The first
// router of the list is the default router. Gives true when it moved.
export function moveRouter(uid, step) {
  const loaded = items.filter((x) => !x.isNew);
  const i = loaded.findIndex((x) => x.uid === uid);
  const j = i + step;
  if (i < 0 || j < 0 || j >= loaded.length) return false;
  const [x] = loaded.splice(i, 1);
  loaded.splice(j, 0, x);
  items = [...loaded, ...items.filter((y) => y.isNew)];
  paint();
  changed();
  emit('saveSourceChanged');
  return true;
}

// moveRouterTo puts router uid before router to (after it with after), for a drag in the list.
export function moveRouterTo(uid, to, after) {
  const loaded = items.filter((x) => !x.isNew).map((x) => x.uid);
  const i = loaded.indexOf(uid);
  let j = loaded.indexOf(to);
  if (i < 0 || j < 0 || uid === to) return false;
  if (after) j++;
  if (i < j) j--;
  return moveRouter(uid, j - i);
}
export const selectedRouter = () => sel;

// The list draws again on routerRows; the detail view only when its router changed.
const changed = () => emit('routerRows');

// ---- The detail view

function fieldOf(attr) {
  const n = attr === 'levelNumbering' ? $('cfg-rt-lvlnum') : inputOf(attr);
  return n?.closest('.sode__field') || null;
}

// The error under a field. It shows once the person left the field, or for a refused Save.
function paintError(attr, text) {
  const f = fieldOf(attr);
  if (!f) return;
  let note = f.querySelector(':scope > .sode__field-err');
  if (!text) {
    note?.remove();
    inputOf(attr)?.classList.remove('pb-input--invalid');
    return;
  }
  if (!note) {
    note = el('p', { class: 'sode__field-err', role: 'alert' });
    f.append(note);
  }
  note.replaceChildren(icon('circle-x'), el('span', null, text));
  inputOf(attr)?.classList.add('pb-input--invalid');
}

function paintErrors(x) {
  const e = errorsOf(x);
  for (const a of DETAIL_ATTRS) paintError(a, shownErr.has(`${x.uid}/${a}`) ? e[a] || '' : '');
}

function paintLevels(x) {
  const v = valuesOf(x);
  for (const m of MODES) {
    const lab = document.querySelector(`label[for="cfg-rt-lvlnum-${m}"]`);
    lab.textContent = numberingRange(v.levels, m);
  }
  $('cfg-rt-offset-note').textContent = offsetNote(x.text.levelWireOffset, v.levelNumbering);
  $('cfg-rt-default-note').textContent = v.defaultLevels === null ? '' : defaultNote(v);
  $('cfg-rt-salvo-note').textContent = groupNote(v.fireMode);
  const count = Number.isInteger(v.levels) ? Math.min(Math.max(v.levels, 0), CHIPS_MAX) : 0;
  const on = new Set(v.defaultLevels === '' ? Array.from({ length: count }, (_, i) => i + 1) : levelsIn(v));
  const colors = S.cfg?.levelColors || [];
  const chips = Array.from({ length: count }, (_, i) => {
    const ui = i + 1;
    const chip = el('span', `sode__slot${on.has(ui) ? ' sode__slot--on' : ''}`, levelLabel(ui, v.levelNumbering));
    const c = /^#[0-9a-f]{6}$/i.test(colors[i] || '') ? colors[i] : S.levelDefaults?.[i] || '';
    return levelColor(chip, { swatch: ui === 1 ? 'video' : String(((ui - 2) % 16) + 1).padStart(2, '0'), color: c });
  });
  const box = $('router-chips');
  box.replaceChildren(...chips);
  box.setAttribute('aria-label', `Levels of this router: ${numberingRange(v.levels, v.levelNumbering)}`);
}

// The UI levels of the default levels of values v (saved in 1 numbering).
function levelsIn(v) {
  const out = [];
  for (const part of String(v.defaultLevels || '').split(',')) {
    const [a, b = a] = part.split('-').map(Number);
    for (let i = a; Number.isInteger(i) && i <= b; i++) out.push(i);
  }
  return out;
}

function paintHead(x) {
  $('router-detail-name').textContent = x.isNew ? `${nameOf(x)} (new)` : nameOf(x);
  const link = $('router-detail-link');
  if (x.isNew) {
    link.replaceChildren(el('span', 'sode__hint', 'Sode connects to this router after you save.'));
  } else {
    const s = linkOfRouter(x);
    const r = linkOf(x);
    link.replaceChildren(...(s ? [stateNode(s)] : []), ...(s && s.key !== 'connected' && r?.err ? [el('span', 'sode__hint sode__code', r.err)] : []));
  }
  $('router-swap').hidden = !admin || x.isNew || x.removed;
  $('router-log').hidden = x.isNew;
  const rm = $('router-remove');
  // Every router can go while another stays (the last router stays).
  rm.hidden = !admin || x.removed || (!x.isNew && staying().filter((y) => !y.isNew).length < 2);
  const loaded = loadedOrder();
  const at = loaded.indexOf(x.uid);
  $('router-up').hidden = $('router-down').hidden = !admin || x.isNew || x.removed || loaded.length < 2;
  $('router-up').disabled = at <= 0;
  $('router-down').disabled = at < 0 || at >= loaded.length - 1;
  rm.textContent = x.isNew ? 'Cancel' : 'Remove…';
  $('router-detail').classList.toggle('sode__rdetail--removed', !!x.removed);
  for (const f of $('router-detail').querySelectorAll('.sode__rgroup input, .sode__rgroup select')) f.disabled = !!x.removed;
  paintNote(x);
}

// The link state as an icon and words (linkState). Connected has no color.
export function stateNode(s) {
  const n = el('span', 'sode__rstate', icon(s.glyph), el('span', null, s.word));
  if (s.state) n.dataset.state = s.state;
  return n;
}

function paintFacts(x) {
  const one = items.length < 2;
  const rd = x.isNew ? null : routerData(dataUid(x));
  const up = rd?.link === 'up';
  const reread = $('router-names-reread');
  reread.hidden = x.isNew;
  $('router-names-at').hidden = x.isNew;
  if (x.isNew) {
    $('router-names-count').textContent = 'Sode reads the names after you save.';
    $('router-names-why').textContent = '';
  } else {
    $('router-names-count').textContent = `${rd.srcs.length} sources, ${rd.dsts.length} destinations.`;
    $('router-names-at').textContent = namesTimeText({ at: rd.namesAt, savedAt: rd.namesSavedAt }, fmtTime);
    if (reread.textContent !== 'Re-reading…') reread.disabled = !up;
    $('router-names-why').textContent = up ? 'Re-reads the names of every router.' : 'Sode cannot read the names while the Router link is down.';
  }
  const v = valuesOf(x);
  const pc = $('router-protects-count');
  if (!Number.isInteger(v.protectDevice) || v.protectDevice <= 0) {
    pc.replaceChildren(icon('shield-off'), el('span', null, 'Protect is off for this router. Sode cannot set a Protect here.'));
  } else if (x.isNew) {
    pc.replaceChildren(el('span', null, 'Sode reads the Protects after you save.'));
  } else {
    const holds = everyHold().filter((h) => h.uid === dataUid(x));
    const sode = holds.filter((h) => h.hold.sode).length;
    pc.replaceChildren(icon('shield'), el('span', null, protectsText(v.protectDevice, sode, holds.length - sode)));
  }
  $('router-protects-show').hidden = x.isNew;
  $('router-ties-group').hidden = one;
  if (!one) {
    const first = S.routers[0]?.uid;
    const end = (e) => e?.router || first;
    const ties = S.cfg?.tieLines || [];
    const from = ties.filter((t) => end(t.from) === x.uid).length;
    const to = ties.filter((t) => end(t.to) === x.uid).length;
    $('router-ties-count').replaceChildren(icon('cable'), el('span', null, tiesText(from, to)));
    $('router-ties-show').hidden = x.isNew;
  }
}

// The remove note: what Remove router breaks, and the buttons that stage or drop it.
function paintNote(x) {
  const box = $('router-remove-note');
  const prefix = valuesOf(x).prefix;
  if (x.removed) {
    const keep = button(`Keep router ${prefix}`, 'pb-btn--outline', () => setRemoved(x, false));
    const { head, lines } = usesNote(prefix, x.uses || []);
    box.replaceChildren(
      el(
        'div',
        'pb-note pb-note--warn',
        el('p', null, `Router ${prefix} goes when you save. The save asks for your PIN again.`),
        el('p', null, head),
        ...(lines.length ? [el('ul', 'sode__routers-uses', ...lines.map((t) => el('li', null, t)))] : []),
        el('div', 'sode__racts', keep),
      ),
    );
    return;
  }
  if (x.note === 'asking') return void box.replaceChildren(el('p', 'sode__hint', `Looking for what names router ${prefix}…`));
  if (x.note === 'error') return void box.replaceChildren(el('div', 'pb-note pb-note--warn', x.err || 'The list did not load.'));
  if (x.note === 'list') {
    const { head, lines } = usesNote(prefix, x.uses || []);
    const go = button(`Remove router ${prefix}`, 'pb-btn--outline', () => setRemoved(x, true));
    const keep = button('Keep', 'pb-btn--ghost', () => {
      const now = itemOf(x.uid);
      if (!now) return;
      now.note = '';
      paintNote(now);
    });
    box.replaceChildren(
      el(
        'div',
        'pb-note pb-note--warn',
        el('p', null, head),
        ...(lines.length ? [el('ul', 'sode__routers-uses', ...lines.map((t) => el('li', null, t)))] : []),
        el('div', 'sode__racts', go, keep),
      ),
    );
    return;
  }
  box.replaceChildren();
}

function button(text, cls, fn) {
  const b = el('button', `pb-btn pb-btn--sm ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
}

// x can be from before a hello: the item of its UID now takes the change.
function setRemoved(was, on) {
  const x = itemOf(was.uid);
  if (!x) return;
  x.removed = on;
  if (!on) x.note = '';
  paint();
  changed();
  emit('saveSourceChanged');
}

async function askRemove(x) {
  if (x.isNew) {
    const i = items.indexOf(x);
    items = items.filter((y) => y !== x);
    sel = (items[i - 1] || items[0])?.uid || '';
    paint();
    changed();
    emit('saveSourceChanged');
    return;
  }
  x.note = 'asking';
  paintNote(x);
  const m = await request('routerUsesResult', { t: 'routerUses', router: x.uid });
  const now = itemOf(x.uid);
  if (!now) return;
  now.note = m.ok ? 'list' : 'error';
  now.uses = m.items || [];
  now.err = m.err || '';
  if (sel === now.uid) paintNote(now);
}

// paint fills the detail view with the selected router's text. The inputs that the person is in keep their text.
function paint() {
  const x = selected();
  $('router-detail').hidden = !x;
  if (!x) return;
  for (const a of DETAIL_ATTRS) {
    const f = inputOf(a);
    if (f && f.value !== x.text[a]) f.value = x.text[a];
  }
  for (const m of MODES) radio(m).checked = x.text.levelNumbering === m;
  // With one router Sode hides the prefix (GLOSSARY.md).
  $('cfg-rt-prefix-field').hidden = !many();
  paintHead(x);
  paintLevels(x);
  paintFacts(x);
  paintErrors(x);
}

// selectRouter shows router uid in the detail view. A UID that is not in the list selects the first router.
export function selectRouter(uid) {
  const x = itemOf(uid) || items[0];
  if (!x) return;
  const was = sel;
  sel = x.uid;
  if (was !== sel) {
    paint();
    changed();
    // The refused marks of the one Save follow the router on show.
    emit('saveSourceChanged');
  }
}

// ---- Edits

function attrOfTarget(t) {
  if (t?.name === 'rt-lvlnum') return 'levelNumbering';
  const m = t?.id?.match(/^cfg-rt-([A-Za-z]+)$/);
  return m && DETAIL_ATTRS.includes(m[1]) ? m[1] : '';
}

function onInput(e) {
  const x = selected();
  const attr = attrOfTarget(e.target);
  if (!x || !attr) return;
  if (attr === 'levelNumbering') {
    if (!e.target.checked) return;
    x.text.defaultLevels = renumberDefault(x.text.defaultLevels, x.text.levelNumbering, e.target.value);
    $('cfg-rt-defaultLevels').value = x.text.defaultLevels;
    x.text.levelNumbering = e.target.value;
  } else x.text[attr] = attr === 'prefix' ? e.target.value.toUpperCase() : e.target.value;
  if (attr === 'prefix' && e.target.value !== x.text.prefix) e.target.value = x.text.prefix;
  // A select or the numbering is left at once: check it and what depends on it (Matrix against Name requests).
  if (e.target.tagName === 'SELECT' || attr === 'levelNumbering') check(x, attr);
  else for (const a of RECHECK[attr] || []) if (shownErr.has(`${x.uid}/${a}`)) paintError(a, errorsOf(x)[a] || '');
  if (shownErr.has(`${x.uid}/${attr}`) && !errorsOf(x)[attr]) paintError(attr, '');
  paintHead(x);
  paintLevels(x);
  paintFacts(x);
  changed();
}

// check shows the error of a setting the person left, and of the settings that depend on it.
function check(x, attr) {
  const e = errorsOf(x);
  for (const a of [attr, ...(RECHECK[attr] || [])]) {
    if (a === attr || e[a]) shownErr.add(`${x.uid}/${a}`);
    if (shownErr.has(`${x.uid}/${a}`)) paintError(a, e[a] || '');
  }
}

const detail = $('router-detail');
detail.addEventListener('input', onInput);
detail.addEventListener('change', onInput);
detail.addEventListener('focusout', (e) => {
  const x = selected();
  const attr = attrOfTarget(e.target);
  if (x && attr) check(x, attr);
});

$('router-up').addEventListener('click', () => moveRouter(sel, -1));
$('router-down').addEventListener('click', () => moveRouter(sel, 1));
$('router-swap').addEventListener('click', () => emit('openSwap', { mine: false, router: sel }));
$('router-remove').addEventListener('click', () => selected() && askRemove(selected()));
$('router-names-reread').addEventListener('click', (e) => {
  if (rereadNames()) {
    e.currentTarget.disabled = true;
    e.currentTarget.textContent = 'Re-reading…';
  }
});
$('router-protects-show').addEventListener('click', () => {
  const x = selected();
  if (x) emit('protectsFor', dataUid(x));
});
$('router-ties-show').addEventListener('click', () => emit('tieLinesFor', sel));

// Add router: a row marked New, with every setting, selected for the person to fill.
export function addRouter() {
  const n = ++addSeq;
  const prefix = freePrefix(items.map((x) => valuesOf(x).prefix));
  const x = { uid: `new:${n}`, n, isNew: true, text: { ...NEW_ROUTER, prefix, title: `Router ${prefix}` }, removed: false, note: '' };
  items.push(x);
  sel = x.uid;
  paint();
  changed();
  emit('saveSourceChanged');
  $('cfg-rt-address').focus();
}

// stageFirstRouter: Run setup again writes the first router's settings as hand edits (setupagain.js).
export function stageFirstRouter(attr, value) {
  const x = items.find(isFirst);
  if (!x) return;
  if (attr === 'levelNumbering') x.text.defaultLevels = renumberDefault(x.text.defaultLevels, x.text.levelNumbering, value);
  x.text[attr] = String(value);
  if (sel === x.uid) paint();
  changed();
  emit('saveSourceChanged');
}

// ---- Loading

// A hello moves only a baseline with no edits on it, so another person's save shows as a conflict.
function load(force = false) {
  if (!S.cfg || (!force && items.length && edited())) return;
  base = { [ORDER_KEY]: S.routers.map((r) => r.uid) };
  vers = { [ORDER_KEY]: S.versions?.[ORDER_KEY] };
  // A remove note that is open stays open.
  const was = force ? new Map() : new Map(items.map((x) => [x.uid, x]));
  items = S.routers.map((r) => {
    const text = textsOf(r);
    const v = readValues(text);
    for (const a of DETAIL_ATTRS) {
      base[listKey(r.uid, a)] = v[a];
      vers[listKey(r.uid, a)] = S.versions?.[listKey(r.uid, a)];
    }
    base[removeKey(r.uid)] = false;
    vers[removeKey(r.uid)] = S.versions?.[removeKey(r.uid)];
    const old = was.get(r.uid);
    return {
      uid: r.uid,
      isNew: false,
      text,
      removed: false,
      note: old?.note === 'list' || old?.note === 'error' ? old.note : '',
      uses: old?.uses,
      err: old?.err,
    };
  });
  if (!itemOf(sel)) sel = items[0]?.uid || '';
  for (const k of [...shownErr]) if (!itemOf(k.split('/')[0])) shownErr.delete(k);
  paint();
  changed();
}

on('hello', () => load());
on('conn', () => {
  const x = selected();
  if (x) {
    paintHead(x);
    paintFacts(x);
  }
  changed();
});
for (const ev of ['names', 'namesChecked', 'protectedRows', 'tieLines'])
  on(ev, () => {
    const x = selected();
    if (!x) return;
    if (ev !== 'protectedRows' && ev !== 'tieLines') $('router-names-reread').textContent = 'Re-read now';
    paintFacts(x);
  });
on('access', (a) => {
  admin = a.role === 'administrator';
  $('router-add').hidden = !admin;
  const x = selected();
  if (x) paintHead(x);
});

// ---- The one Save

const attrOf = (key) => keyAttr(key);

addSaveSource({
  owns: (key) => LIST_KEY.test(key) || key === ORDER_KEY,
  base: () => base,
  now,
  version: (key) => vers[key],
  label(r) {
    if (r.key === ORDER_KEY) {
      const [was, now] = [r.was?.[0], r.now?.[0]];
      return was !== now ? `Router order. Default router: ${loadedName(was)} → ${nameOfUid(now)}` : 'Router order';
    }
    const attr = attrOf(r.key);
    const uid = keyRouter(r.key);
    if (attr === 'add') return `New router ${now()[r.key]?.prefix || ''}`.trim();
    if (attr === 'remove') return `Remove ${loadedName(uid)}`;
    // With one router a row needs no router name.
    return S.routers.length + items.filter((x) => x.isNew).length > 1 ? `${loadedName(uid)}: ${ATTR_LABELS[attr]}` : ATTR_LABELS[attr];
  },
  show(r, v) {
    if (r.key === ORDER_KEY) return (v || []).map(loadedName).join(', ');
    const attr = attrOf(r.key);
    const x = itemOf(keyRouter(r.key));
    if (attr === 'add') return v ? addText(v) : 'Blank';
    if (attr === 'remove') {
      if (!v) return 'Kept';
      const head = usesNote(x ? valuesOf(x).prefix : '', x?.uses || []).head;
      const next = defaultAfter();
      // The first router goes: the next router that stays is the default router.
      return x && isFirst(x) && next ? `${head} ${nameOfUid(next.uid)} becomes the default router.` : head;
    }
    return valueText(attr, v, x ? valuesOf(x) : {});
  },
  // A key of the router in view marks its field; a key of another router marks the section.
  node(key) {
    const attr = attrOf(key);
    const uid = keyRouter(key);
    if (attr === 'add') return sel === uid ? $('cfg-rt-address') : null;
    if (key === ORDER_KEY) return null;
    if (uid !== sel) return null;
    if (attr === 'remove') return $('router-remove');
    if (attr === 'levelNumbering') return $('cfg-rt-lvlnum');
    return inputOf(attr);
  },
  // Every error of every router: Save names each router and its setting, and shows the first one.
  problem() {
    const bad = [];
    for (const x of items) {
      const e = errorsOf(x);
      for (const a of DETAIL_ATTRS) if (e[a]) bad.push({ x, a, text: e[a] });
    }
    if (!bad.length) return '';
    for (const b of bad) shownErr.add(`${b.x.uid}/${b.a}`);
    const first = bad[0];
    if (sel !== first.x.uid) selectRouter(first.x.uid);
    else paintErrors(first.x);
    const f = first.a === 'levelNumbering' ? radio(first.x.text.levelNumbering) : inputOf(first.a);
    requestAnimationFrame(() => f?.focus());
    const lines = bad.map((b) => `${nameOf(b.x)}: ${ATTR_LABELS[b.a]}. ${b.text}`);
    return `${bad.length === 1 ? 'A setting has an error' : `${bad.length} settings have an error`}. ${lines.join(' ')}`;
  },
  saved(key, value, r) {
    const attr = attrOf(key);
    if (attr === 'add') {
      // The saved router stays selected under its new UID.
      if (sel === keyRouter(key) && r.value?.id) sel = r.value.id;
      items = items.filter((x) => !(x.isNew && addKey(x.n) === key));
    } else {
      base[key] = value;
      if (r.version) vers[key] = r.version;
    }
    // The hello with the new list came before this answer, so the rows follow it once the other fields are saved.
    if (attr === 'add' || attr === 'remove') queueMicrotask(() => load());
    else queueMicrotask(changed);
  },
  conflict(key, r) {
    vers[key] = r.version;
    base[key] = r.value;
    queueMicrotask(changed);
  },
  reset() {
    shownErr.clear();
    items = [];
    load(true);
  },
});

// Search (settingsfind.js): one entry per router for each setting of the detail view, named by the router.
export const searchRouters = () => (items.length > 1 ? items.map((x) => ({ uid: x.uid, name: nameOf(x) })) : []);
