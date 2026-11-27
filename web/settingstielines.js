// Settings > Routers > Tie lines (Administrator): add, edit and remove tie lines. The list is cfg.tieLines and saves whole
// with the `tieLines` message, apart from the one Save. Rules are in tielinerules.js; the server checks them again.
// One-off tie lines in use come from the `oneOffTieLines` message (internal/app/tieoneoff.go). Save as tie line adds
// one to the list. Remove asks the churning-letter Confirm (confirmbox.js) under the row; the change stays instant
// after it, apart from the one Save. The result shows where the row was.

import { $, el, S, on, onMsg, emit, request, routerData } from './core.js';
import { btn, opt } from './formbits.js';
import { confirmBox } from './confirmbox.js';
import { readJSON, writeJSON } from './store.js';
import { suggest, suggestSig, acceptName } from './tielinesuggest.js';
import { listProblem, fillName, upsert, without, wireNumber, otherRouter, movedLines, removeQuestion } from './tielinerules.js';

const list = $('tielines-list');
const form = $('tielines-form');
const status = $('tielines-status');
const suggestBox = $('tielines-suggest');
const suggestList = $('tielines-suggest-list');
const suggestMore = $('tielines-suggest-more');
const fromRouter = $('tieline-from-router');
const toRouter = $('tieline-to-router');
const fromNum = $('tieline-from');
const toNum = $('tieline-to');
const nameBox = $('tieline-name');
const addBtn = $('tieline-add');
const cancelBtn = $('tieline-cancel');
const oneOffBox = $('tielines-oneoffs');
const oneOffList = $('tielines-oneoffs-list');

// editing: the id of the tie line in the form, or '' for a new one. auto: the name that Sode filled last.
let editing = '';
let auto = '';
// The Remove Confirm. asking: the id of the tie line it asks about. removed: {at, text}, the result of the last
// Remove, shown at the place of its row until the next Remove.
const confirm = confirmBox(list.closest('fieldset'));
const confirmRow = el('li', 'sode__tielines-confirm', confirm.node);
let asking = '';
let removed = null;

const tieLines = () => (Array.isArray(S.cfg?.tieLines) ? S.cfg.tieLines : []);
const uids = () => S.routers.map((r) => r.uid);
const shownNum = (uid, n) => n + routerData(uid).off;
const routerName = (uid, kind, n) => (routerData(uid)[kind === 'dst' ? 'dstBy' : 'srcBy'].get(n)?.name || '').trim();

// An end as shown, always with its router prefix: a tie line has its two ends on different routers.
function endText(end, kind) {
  const rd = routerData(end.router);
  const name = routerName(end.router, kind, end.n);
  return `${rd.prefix}.${shownNum(end.router, end.n)}${name ? ` ${name}` : ''}`;
}

function row(t) {
  // A row can be older than the list, so Edit and Remove read the tie line as it is now.
  const now = () => tieLines().find((x) => x.id === t.id) || t;
  const edit = btn('pb-btn--outline pb-btn--sm', 'Edit', () => startEdit(now()));
  edit.setAttribute('aria-label', `Edit ${t.name}`);
  const remove = btn('pb-btn--outline pb-btn--sm', 'Remove', () => askRemove(now()));
  remove.setAttribute('aria-label', `Remove ${t.name}`);
  // Name moved: the tie line works on. Edit and Save tie line store the names now.
  const moved = movedLines(t, (r) => `${routerData(r.router).prefix}.${shownNum(r.router, r.n)}`);
  const marks = moved.length
    ? [el('span', 'pb-badge pb-badge--warn', 'Name moved'), el('span', 'sode__hint', `${moved.join('; ')}. Edit and save to keep the names now.`)]
    : [];
  const li = el(
    'li',
    'sode__tielines-row',
    el('strong', null, t.name),
    el('span', 'sode__hint', `${endText(t.from, 'dst')} feeds ${endText(t.to, 'src')}`),
    ...marks,
    edit,
    remove,
  );
  li.dataset.id = t.id;
  return li;
}

async function askRemove(t) {
  removed = null;
  asking = t.id;
  render();
  const ok = await confirm.ask(removeQuestion(t, `${endText(t.from, 'dst')} feeds ${endText(t.to, 'src')}`, S.cfg?.presets || []), 'Remove');
  if (asking !== t.id) return;
  asking = '';
  const at = tieLines().findIndex((x) => x.id === t.id);
  if (!ok) return render();
  const text = `Removed ${t.name}.`;
  if (await save(without(tieLines(), t.id), text)) removed = { at, text };
  render();
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
    render();
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

function fillRouterSelect(sel, want) {
  sel.replaceChildren(...S.routers.map((r) => opt(r.uid, `${r.prefix} ${r.title}`.trim())));
  sel.value = uids().includes(want) ? want : sel.options[0]?.value || '';
}

// The form's tie line, with wire numbers. A number that is not whole gives NaN, which listProblem refuses.
function fromForm() {
  return {
    id: editing,
    name: nameBox.value.trim(),
    from: { router: fromRouter.value, n: wireNumber(fromNum.value, routerData(fromRouter.value).off) },
    to: { router: toRouter.value, n: wireNumber(toNum.value, routerData(toRouter.value).off) },
  };
}

// The names under the number boxes, and the Tie line name while nobody has typed a different one.
function showNames() {
  const t = fromForm();
  const src = Number.isInteger(t.to.n) && t.to.n >= 0 ? routerName(t.to.router, 'src', t.to.n) : '';
  $('tieline-from-name').textContent = Number.isInteger(t.from.n) && t.from.n >= 0 ? routerName(t.from.router, 'dst', t.from.n) : '';
  $('tieline-to-name').textContent = src;
  const next = fillName({ name: nameBox.value, auto }, src);
  auto = next.auto;
  if (nameBox.value !== next.name) nameBox.value = next.name;
}

function resetForm() {
  editing = '';
  auto = '';
  const first = uids()[0] || '';
  fillRouterSelect(fromRouter, uids().includes(fromRouter.value) ? fromRouter.value : first);
  fillRouterSelect(toRouter, otherRouter(uids(), fromRouter.value));
  fromNum.value = '';
  toNum.value = '';
  nameBox.value = '';
  addBtn.textContent = 'Add tie line';
  cancelBtn.hidden = true;
  showNames();
}

function startEdit(t) {
  editing = t.id;
  fillRouterSelect(fromRouter, t.from.router);
  fillRouterSelect(toRouter, t.to.router);
  fromNum.value = String(shownNum(t.from.router, t.from.n));
  toNum.value = String(shownNum(t.to.router, t.to.n));
  nameBox.value = t.name;
  auto = routerName(t.to.router, 'src', t.to.n);
  addBtn.textContent = 'Save tie line';
  cancelBtn.hidden = false;
  status.textContent = '';
  showNames();
  fromNum.focus();
}

async function save(next, done) {
  const why = listProblem(next, uids());
  if (why) {
    status.textContent = `Not saved: ${why}.`;
    return false;
  }
  status.textContent = 'Saving…';
  const r = await request('tieLinesResult', { t: 'tieLines', tieLines: next });
  status.textContent = r.ok ? done : `Not saved: ${r.err}.`;
  return r.ok;
}

function render() {
  if (!S.cfg) return;
  const two = S.routers.length >= 2;
  form.hidden = !two;
  const rows = tieLines().map(row);
  if (removed) rows.splice(Math.max(0, removed.at), 0, el('li', 'sode__hint sode__tielines-removed', removed.text));
  if (!rows.length) rows.push(el('li', 'sode__hint', two ? 'No tie lines.' : 'Tie lines need two or more routers.'));
  // The Confirm stays under its row while the list draws again.
  const at = asking ? rows.findIndex((r) => r.dataset.id === asking) : -1;
  if (asking && at < 0) {
    asking = '';
    confirm.cancel();
  }
  if (at >= 0) rows.splice(at + 1, 0, confirmRow);
  list.replaceChildren(...rows);
  showSuggestions(two ? suggestions() : { shown: [], more: 0 });
  if (!two) return;
  if (editing && !tieLines().some((t) => t.id === editing)) resetForm();
  else if (!editing) {
    fillRouterSelect(fromRouter, fromRouter.value);
    fillRouterSelect(toRouter, toRouter.value && toRouter.value !== fromRouter.value ? toRouter.value : otherRouter(uids(), fromRouter.value));
    showNames();
  }
}

fromRouter.addEventListener('change', () => {
  if (toRouter.value === fromRouter.value) toRouter.value = otherRouter(uids(), fromRouter.value);
  showNames();
});
for (const x of [toRouter, fromNum, toNum]) x.addEventListener('input', showNames);
toRouter.addEventListener('change', showNames);
cancelBtn.addEventListener('click', () => {
  resetForm();
  status.textContent = '';
});
addBtn.addEventListener('click', async () => {
  const t = fromForm();
  const name = t.name || $('tieline-to-name').textContent.trim();
  const was = editing;
  if (await save(upsert(tieLines(), { ...t, name }), was ? `Saved ${name}.` : `Added ${name}.`)) resetForm();
});

onMsg('tieLines', (m) => {
  S.cfg = { ...S.cfg, tieLines: Array.isArray(m.tieLines) ? m.tieLines : [] };
  emit('tieLines');
});

let frame = 0;
const later = () => {
  frame ||= requestAnimationFrame(() => {
    frame = 0;
    render();
  });
};
for (const ev of ['hello', 'names', 'tieLines']) on(ev, later);
for (const ev of ['hello', 'tally', 'tieLines']) on(ev, askLater);
on('access', (a) => {
  admin = a.role === 'administrator';
  if (!admin) oneOffs = [];
  showOneOffs();
  askLater();
});
