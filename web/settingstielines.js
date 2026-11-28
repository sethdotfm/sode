// Settings > Routers > Tie lines (Administrator): a table of the tie lines, grouped by router pair. A find box and a
// router filter are above it. The list has a bounded height and a big scroll bar. Add tie line opens the editor at the
// top; Edit opens it in the row (tielineedit.js). The list is cfg.tieLines and saves whole with the `tieLines`
// message, apart from the one Save. Rules are in tielinerules.js; the server checks them again. Remove asks the
// churning-letter Confirm (confirmbox.js) under the row; the result shows where the row was. One-off tie lines in use
// come from the `oneOffTieLines` message (internal/app/tieoneoff.go). Save as tie line adds one to the list.
// Entry point: showTieLinesFor(routerUid), or the bus event 'tieLinesFor', opens the section filtered to one router.

import { $, el, S, on, onMsg, emit, request, routerData, icon, tallyText } from './core.js';
import { btn, opt } from './formbits.js';
import { confirmBox } from './confirmbox.js';
import { readJSON, writeJSON } from './store.js';
import { suggest, suggestSig, acceptName } from './tielinesuggest.js';
import { tieEditor } from './tielineedit.js';
import {
  listProblem,
  upsert,
  without,
  movedLines,
  removeQuestion,
  ALL_ROUTERS,
  ROUTER_BUTTONS_MAX,
  pairGroups,
  pairHeading,
  countLine,
  routerCounts,
  carries,
} from './tielinerules.js';

const section = $('tielines-app').closest('fieldset');
const app = $('tielines-app');
const one = $('tielines-one');
const weight = $('tielines-weight');
const list = $('tielines-list');
const tools = $('tielines-tools');
const countEl = $('tielines-count');
const addBox = $('tielines-add-box');
const newBtn = $('tieline-new');
const status = $('tielines-status');
const suggestBox = $('tielines-suggest');
const suggestList = $('tielines-suggest-list');
const suggestMore = $('tielines-suggest-more');
const oneOffBox = $('tielines-oneoffs');
const oneOffList = $('tielines-oneoffs-list');

const F = { q: '', router: ALL_ROUTERS };
// The open editor: { id, ed }. id is '' for Add tie line at the top.
let editor = null;
// The Remove Confirm. asking: the id of the tie line it asks about. removed: {after, text}, the result of the last
// Remove. It shows where its row was, over the row with id after (else last), until the next Remove.
const confirm = confirmBox(section);
const confirmRow = el('div', 'sode__tielines-confirm', confirm.node);
let asking = '';
let removed = null;

const tieLines = () => (Array.isArray(S.cfg?.tieLines) ? S.cfg.tieLines : []);
const uids = () => S.routers.map((r) => r.uid);
const shownNum = (uid, n) => n + routerData(uid).off;
const routerName = (uid, kind, n) => (routerData(uid)[kind === 'dst' ? 'dstBy' : 'srcBy'].get(n)?.name || '').trim();
const shownEnd = (e) => `${routerData(e.router).prefix}.${shownNum(e.router, e.n)}`;

// An end as shown, always with its router prefix: a tie line has its two ends on different routers.
function endText(end, kind) {
  const name = routerName(end.router, kind, end.n);
  return `${shownEnd(end)}${name ? ` ${name}` : ''}`;
}

const rowOf = (t) => ({
  id: t.id,
  name: t.name,
  t,
  from: { router: t.from.router, n: t.from.n, num: shownEnd(t.from), name: routerName(t.from.router, 'dst', t.from.n) },
  to: { router: t.to.router, n: t.to.n, num: shownEnd(t.to), name: routerName(t.to.router, 'src', t.to.n) },
});

// The cell of one end: the number and the name, and the Router title under them.
// A long name ends in an ellipsis, so its tip gives the whole text.
function endCell(e) {
  const rd = routerData(e.router);
  const title = rd.title || `Router ${rd.prefix}`;
  return el(
    'span',
    'sode__tend',
    el('span', { class: 'sode__tend-line', 'data-pb-tip': `${e.num}${e.name ? ` ${e.name}` : ''}` }, el('span', 'sode__tend-n', e.num), e.name ? ` ${e.name}` : ''),
    el('span', { class: 'sode__tend-title', 'data-pb-tip': title }, title),
  );
}

// The State cell: what the upstream destination carries now, the routers that are not connected, and Name moved.
function stateCell(r) {
  const parts = [];
  const lost = [r.from.router, r.to.router].filter((u) => routerData(u).link !== 'up');
  if (lost.length) {
    const words = `${lost.map((u) => routerData(u).prefix).join(' and ')} not connected`;
    parts.push(el('span', 'sode__tstate-lost', icon('circle-x'), el('span', null, words)));
  } else {
    const c = carries(routerData(r.from.router).tally.get(r.from.n));
    // The carried number and name wrap as one unit below "Carries", never one character alone.
    const text =
      c.kind === 'one'
        ? ['Carries ', el('span', 'sode__tstate-src', tallyText(c.src, r.from.router))]
        : c.kind === 'nothing'
          ? 'Carries nothing'
          : c.kind === 'mixed'
            ? 'Carries mixed sources'
            : 'Not known';
    parts.push(el('span', 'sode__tstate-carries', ...[text].flat()));
  }
  // Name moved: the tie line works on. Edit and Save tie line store the names now.
  const moved = movedLines(r.t, shownEnd);
  if (moved.length) {
    parts.push(el('span', 'pb-badge pb-badge--warn', 'Name moved'));
    parts.push(el('span', 'sode__hint', `${moved.join('; ')}. Edit and save to keep the names now.`));
  }
  return el('span', 'sode__tstate', ...parts);
}

function rowNode(r) {
  // A row can be older than the list, so Edit and Remove read the tie line as it is now.
  const now = () => tieLines().find((x) => x.id === r.id) || r.t;
  if (editor && editor.id === r.id) {
    const node = el('div', { class: 'sode__trow sode__trow--edit', role: 'listitem' }, editor.ed.node);
    node.dataset.id = r.id; // not data-key: the Settings form reads each [data-key] as a field.
    return node;
  }
  const edit = btn('pb-btn--outline pb-btn--sm', 'Edit', () => openEditor(now()));
  edit.setAttribute('aria-label', `Edit ${r.name}`);
  edit.dataset.act = 'edit';
  const remove = btn('pb-btn--outline pb-btn--sm', 'Remove', () => askRemove(now()));
  remove.setAttribute('aria-label', `Remove ${r.name}`);
  remove.dataset.act = 'remove';
  const node = el(
    'div',
    { class: 'sode__trow', role: 'listitem' },
    el('strong', 'sode__trow-name', r.name),
    endCell(r.from),
    endCell(r.to),
    stateCell(r),
    el('span', 'sode__prow-act', edit, remove),
  );
  node.dataset.id = r.id;
  return node;
}

function groupNode(g) {
  const titles = `${g.from.title || `Router ${g.from.prefix}`} to ${g.to.title || `Router ${g.to.prefix}`}`;
  const head = el('div', 'sode__pgroup-head', el('strong', 'sode__pgroup-name', pairHeading(g)), el('span', 'sode__hint', titles));
  const sec = el('section', { class: 'sode__tgroup', 'aria-label': `${g.from.prefix} to ${g.to.prefix}` }, head);
  sec.dataset.pair = g.key;
  const body = [];
  for (const r of g.rows) {
    if (removed && removed.after === r.id) body.push(removedNode());
    body.push(rowNode(r));
    if (asking === r.id) body.push(confirmRow);
  }
  sec.append(el('div', { role: 'list' }, ...body));
  return sec;
}

const removedNode = () => el('div', 'sode__hint sode__tielines-removed', removed.text);

async function askRemove(t) {
  removed = null;
  asking = t.id;
  closeEditor(false);
  draw();
  const ok = await confirm.ask(removeQuestion(t, `${endText(t.from, 'dst')} feeds ${endText(t.to, 'src')}`, S.cfg?.presets || []), 'Remove');
  if (asking !== t.id) return;
  asking = '';
  if (!ok) {
    draw();
    list.querySelector(`[data-id="${CSS.escape(t.id)}"] [data-act="remove"]`)?.focus({ preventScroll: true });
    return;
  }
  const text = `Removed ${t.name}.`;
  // The row before it in the drawn table: the result shows where the row was.
  const drawn = [...list.querySelectorAll('.sode__trow[data-id]')].map((n) => n.dataset.id);
  const at = drawn.indexOf(t.id);
  if (await save(without(tieLines(), t.id), text)) return draw();
  // The removed row's place: after the next row, which takes its place, else as the last row of its group.
  removed = { after: drawn[at + 1] || '', text };
  draw();
}

// Dismissed suggestions stay in this browser, by their two ends. A dismissal is no setting of the server.
const DISMISS_KEY = 'sode-tieline-dismissed';
const dismissed = () => new Set(Array.isArray(readJSON(DISMISS_KEY, [])) ? readJSON(DISMISS_KEY, []) : []);

function suggestions() {
  const names = (m) => [...m].map(([n, v]) => [n, v?.name || '']);
  const routers = S.routers.map((r) => ({ uid: r.uid, dst: names(routerData(r.uid).dstBy), src: names(routerData(r.uid).srcBy) }));
  return suggest(routers, tieLines(), dismissed());
}

function suggestRow(s) {
  const accept = btn('sode-btn--primary pb-btn--sm', 'Accept', async () => {
    const name = acceptName(s.name, tieLines());
    await save(upsert(tieLines(), { id: '', name, from: s.from, to: s.to }), `Added ${name}.`);
  });
  accept.setAttribute('aria-label', `Accept ${s.name}`);
  const dismiss = btn('pb-btn--outline pb-btn--sm', 'Dismiss', () => {
    writeJSON(DISMISS_KEY, [...dismissed(), s.key]);
    draw();
  });
  dismiss.setAttribute('aria-label', `Dismiss ${s.name}`);
  const li = el(
    'li',
    'sode__tielines-row',
    el('strong', null, s.name),
    el('span', 'sode__hint', `${endText(s.from, 'dst')} feeds ${endText(s.to, 'src')}`),
    accept,
    dismiss,
  );
  li.dataset.ends = s.key;
  return li;
}

// One-off tie lines in use: the server keeps each one while its downstream source is routed. Save as tie line fills
// the Tie line name from the name of the downstream source, as Accept does.
let admin = false;
let oneOffs = [];

function oneOffRow(o) {
  const save1 = btn('sode-btn--primary pb-btn--sm', 'Save as tie line', async () => {
    const name = acceptName(o.to.name || o.name, tieLines());
    await save(upsert(tieLines(), { id: '', name, from: { router: o.from.router, n: o.from.n }, to: { router: o.to.router, n: o.to.n } }), `Added ${name}.`);
    askOneOffs();
  });
  save1.setAttribute('aria-label', `Save ${o.name} as tie line`);
  const li = el(
    'li',
    'sode__tielines-row',
    el('strong', null, o.name),
    el('span', 'sode__hint', `${endText(o.from, 'dst')} feeds ${endText(o.to, 'src')}${o.by ? `, stated by ${o.by}` : ''}`),
    save1,
  );
  li.dataset.oneoff = o.id;
  return li;
}

function showOneOffs() {
  oneOffBox.hidden = !admin || !oneOffs.length;
  oneOffList.replaceChildren(...oneOffs.map(oneOffRow));
}

let oneOffSeq = 0;
async function askOneOffs() {
  if (!admin) return;
  const seq = ++oneOffSeq;
  const m = await request('oneOffTieLines', { t: 'oneOffTieLines' });
  if (seq !== oneOffSeq) return;
  const next = Array.isArray(m.oneOffs) ? m.oneOffs : [];
  if (JSON.stringify(next) === JSON.stringify(oneOffs)) return;
  oneOffs = next;
  showOneOffs();
}

// A take moves many crosspoints: one ask after the burst.
let oneOffWait = 0;
const askLater = () => {
  if (!admin || oneOffWait) return;
  oneOffWait = setTimeout(() => {
    oneOffWait = 0;
    askOneOffs();
  }, 300);
};

// A names or hello event that changes no suggestion keeps the rows, and the focus on them.
let shownSig = '';
function showSuggestions(found) {
  const sig = suggestSig(found, (s) => `${endText(s.from, 'dst')}|${endText(s.to, 'src')}`);
  if (sig === shownSig) return;
  shownSig = sig;
  suggestBox.hidden = !found.shown.length;
  suggestList.replaceChildren(...found.shown.map(suggestRow));
  suggestMore.hidden = !found.more;
  suggestMore.textContent = `${found.more} more ${found.more === 1 ? 'suggestion' : 'suggestions'}. Accept or dismiss a suggestion to show the next one.`;
}

// Saves the whole list. Gives '' when it saved, else why not, also in the status line. own: the caller shows why not
// (the open editor), so the status line stays blank and the error shows once.
async function save(next, done, { changed = '', own = false } = {}) {
  const fail = (why) => {
    status.textContent = own ? '' : `Not saved: ${why}.`;
    return `Not saved: ${why}.`;
  };
  const why = listProblem(next, uids(), { shown: shownEnd, changed });
  if (why) return fail(why);
  status.textContent = 'Saving…';
  const r = await request('tieLinesResult', { t: 'tieLines', tieLines: next });
  if (!r.ok) return fail(r.err);
  status.textContent = done;
  return '';
}

// The editor: Add tie line (id '') at the top, or Edit in the row of t.
function openEditor(t) {
  closeEditor(false);
  removed = null;
  if (asking) {
    asking = '';
    confirm.cancel();
  }
  const id = t?.id || '';
  const ed = tieEditor(t, {
    defRouter: F.router === ALL_ROUTERS ? '' : F.router,
    onSave: async (next) => {
      const name = next.name;
      const why = await save(upsert(tieLines(), next), id ? `Saved ${name}.` : `Added ${name}.`, { changed: id, own: true });
      if (why) return why;
      closeEditor(true);
      const saved = id || tieLines().find((x) => x.name === name)?.id;
      if (saved) list.querySelector(`[data-id="${CSS.escape(saved)}"]`)?.scrollIntoView({ block: 'nearest' });
      return '';
    },
    onClose: () => closeEditor(true),
  });
  editor = { id, ed };
  status.textContent = '';
  if (!id) {
    addBox.replaceChildren(ed.node);
    addBox.hidden = false;
    newBtn.disabled = true;
  }
  draw();
  ed.focus();
}

// focusBack: put focus back on the button that opened the editor.
function closeEditor(focusBack) {
  if (!editor) return;
  const id = editor.id;
  editor = null;
  addBox.hidden = true;
  addBox.replaceChildren();
  newBtn.disabled = false;
  draw();
  if (!focusBack) return;
  const back = id ? list.querySelector(`[data-id="${CSS.escape(id)}"] [data-act="edit"]`) : newBtn;
  back?.focus({ preventScroll: true });
}

function filterButton(text, count, on, act, fn) {
  const b = btn(`pb-btn--sm${on ? ' pb-btn--selected' : ''}`, [text, ' ', el('span', 'sode__pcount', String(count))], fn);
  b.setAttribute('aria-pressed', String(on));
  b.dataset.act = act;
  return b;
}

const routerList = () => S.routers.map((r) => ({ uid: r.uid, prefix: r.prefix, title: r.title }));

function drawTools(rows, rts) {
  const rc = routerCounts(rows, rts, F);
  const pick = (uid) => () => {
    F.router = uid;
    draw();
  };
  let filter;
  if (rts.length <= ROUTER_BUTTONS_MAX) {
    filter = el(
      'span',
      { class: 'sode__pseg', role: 'group', 'aria-label': 'Router' },
      filterButton('All routers', rc[ALL_ROUTERS], F.router === ALL_ROUTERS, 'router-all', pick(ALL_ROUTERS)),
      ...rts.map((rt) => filterButton(rt.prefix, rc[rt.uid], F.router === rt.uid, `router-${rt.uid}`, pick(rt.uid))),
    );
  } else {
    const sel = el('select', { class: 'pb-select', id: 'tielines-router', 'aria-label': 'Router' });
    sel.append(opt(ALL_ROUTERS, `All routers (${rc[ALL_ROUTERS]})`), ...rts.map((rt) => opt(rt.uid, `${rt.prefix} ${rt.title} (${rc[rt.uid]})`)));
    sel.value = F.router;
    sel.addEventListener('change', () => pick(sel.value)());
    filter = el('span', 'pb-select-wrap', sel);
  }
  tools.replaceChildren(filter);
}

// While the find box has focus, its row of tools stays; only the counts and the pressed buttons change.
function updateCounts(rows, rts) {
  const rc = routerCounts(rows, rts, F);
  for (const b of tools.querySelectorAll('[data-act^="router-"]')) {
    const uid = b.dataset.act === 'router-all' ? ALL_ROUTERS : b.dataset.act.slice(7);
    const c = b.querySelector('.sode__pcount');
    if (c) c.textContent = String(rc[uid] ?? 0);
    const on = F.router === uid;
    b.classList.toggle('pb-btn--selected', on);
    b.setAttribute('aria-pressed', String(on));
  }
}

const find = el('input', {
  class: 'pb-input sode__pfind',
  type: 'search',
  id: 'tielines-find',
  placeholder: 'Find a tie line by name or number',
  'aria-label': 'Find a tie line',
});
find.autocomplete = 'off';
// The find box is on the row of Add tie line; the router filter is on the next row, with the count.
newBtn.after(find);
find.addEventListener('input', () => {
  F.q = find.value;
  draw();
});
find.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') e.preventDefault();
  if (e.key === 'Escape' && find.value) {
    e.preventDefault();
    e.stopPropagation();
    find.value = F.q = '';
    draw();
  }
});

// A redraw waits while focus is in an open editor, so typed text and the match list stay. It runs when focus leaves.
let waiting = false;
section.addEventListener('focusout', () => {
  setTimeout(() => {
    if (waiting && !editor?.ed.active) draw();
  });
});

function draw() {
  if (!S.cfg) return;
  if (editor?.ed.active && editor.id && list.contains(editor.ed.node)) {
    waiting = true;
    return;
  }
  waiting = false;
  const two = S.routers.length >= 2;
  one.hidden = two;
  app.hidden = !two;
  weight.hidden = !two;
  if (!two) {
    closeEditor(false);
    return;
  }
  const rts = routerList();
  if (F.router !== ALL_ROUTERS && !rts.some((r) => r.uid === F.router)) F.router = ALL_ROUTERS;
  if (editor?.id && !tieLines().some((t) => t.id === editor.id)) {
    editor = null;
  }
  const rows = tieLines().map(rowOf);
  // An open editor's row shows whatever the filter: a person must see what they edit.
  const groups = pairGroups(rows, rts, F);
  if (editor?.id && !groups.some((g) => g.rows.some((r) => r.id === editor.id))) {
    F.q = find.value = '';
    F.router = ALL_ROUTERS;
    return draw();
  }
  const focus = document.activeElement;
  const toolAct = tools.contains(focus) && focus !== find ? focus.dataset.act : null;
  const focusAct = list.contains(focus) ? focus.dataset.act : null;
  const focusId = focusAct ? focus.closest('[data-id]')?.dataset.id : null;
  const top = list.scrollTop;
  const nodes = groups.map(groupNode);
  if (!nodes.length) nodes.push(el('div', 'sode__pempty', rows.length ? 'No tie line matches.' : 'No tie lines. To add one, use Add tie line.'));
  // The result of a Remove whose next row is not drawn goes last.
  if (removed && !groups.some((g) => g.rows.some((r) => r.id === removed.after))) nodes.push(removedNode());
  if (asking && !rows.some((r) => r.id === asking)) {
    asking = '';
    confirm.cancel();
  }
  list.replaceChildren(
    el(
      'div',
      { class: 'sode__trow sode__trow--head', 'aria-hidden': 'true' },
      el('span', null, 'Tie line name'),
      el('span', null, 'From (destination)'),
      el('span', null, 'To (source)'),
      el('span', null, 'State'),
      el('span'),
    ),
    ...nodes,
  );
  list.scrollTop = top;
  if (document.activeElement !== find) drawTools(rows, rts);
  else updateCounts(rows, rts);
  countEl.textContent = countLine(rows, F);
  if (toolAct) tools.querySelector(`[data-act="${toolAct}"]`)?.focus({ preventScroll: true });
  if (focusAct && !asking) list.querySelector(`[data-id="${CSS.escape(focusId || '')}"] [data-act="${focusAct}"]`)?.focus({ preventScroll: true });
  showSuggestions(suggestions());
}

// Only the State cells change on a tally or a link change: the editor and the focus stay.
function drawStates() {
  if (!S.cfg || S.routers.length < 2) return;
  const by = new Map(tieLines().map((t) => [t.id, t]));
  for (const n of list.querySelectorAll('.sode__trow[data-id]')) {
    const t = by.get(n.dataset.id);
    const cell = n.querySelector(':scope > .sode__tstate');
    if (t && cell) cell.replaceWith(stateCell(rowOf(t)));
  }
}

newBtn.addEventListener('click', () => openEditor(null));

// showTieLinesFor opens Settings > Routers > Tie lines with the router filter on one router (its own UID; blank is
// the first router). The detail view of a router links here.
export function showTieLinesFor(routerUid = '') {
  if (document.documentElement.dataset.view !== 'settings') emit('settingsOpen');
  F.router = routerUid || S.routers[0]?.uid || ALL_ROUTERS;
  F.q = find.value = '';
  draw();
  requestAnimationFrame(() => section.scrollIntoView({ block: 'start' }));
}
on('tieLinesFor', (uid) => showTieLinesFor(uid || ''));

onMsg('tieLines', (m) => {
  S.cfg = { ...S.cfg, tieLines: Array.isArray(m.tieLines) ? m.tieLines : [] };
  emit('tieLines');
});

let frame = 0;
const later = () => {
  frame ||= setTimeout(() => {
    frame = 0;
    draw();
  }, 30);
};
let stateFrame = 0;
const statesLater = () => {
  stateFrame ||= setTimeout(() => {
    stateFrame = 0;
    drawStates();
  }, 100);
};
for (const ev of ['hello', 'names', 'tieLines']) on(ev, later);
for (const ev of ['tally', 'conn']) on(ev, statesLater);
for (const ev of ['hello', 'tally', 'tieLines']) on(ev, askLater);
on('access', (a) => {
  admin = a.role === 'administrator';
  if (!admin) oneOffs = [];
  showOneOffs();
  askLater();
});
