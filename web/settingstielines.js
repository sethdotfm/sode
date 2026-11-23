// Settings > Routers > Tie lines (Administrator): add, edit and remove tie lines. The list is cfg.tieLines and saves whole
// with the `tieLines` message, apart from the one Save. Rules are in tielinerules.js; the server checks them again.

import { $, el, S, on, onMsg, emit, request, routerData } from './core.js';
import { btn, opt } from './formbits.js';
import { listProblem, fillName, upsert, without, wireNumber, otherRouter } from './tielinerules.js';

const list = $('tielines-list');
const form = $('tielines-form');
const status = $('tielines-status');
const fromRouter = $('tieline-from-router');
const toRouter = $('tieline-to-router');
const fromNum = $('tieline-from');
const toNum = $('tieline-to');
const nameBox = $('tieline-name');
const addBtn = $('tieline-add');
const cancelBtn = $('tieline-cancel');

// editing: the id of the tie line in the form, or '' for a new one. auto: the name that Sode filled last.
let editing = '';
let auto = '';

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
  const remove = btn('pb-btn--outline pb-btn--sm', 'Remove', () => save(without(tieLines(), t.id), `Removed ${now().name}.`));
  remove.setAttribute('aria-label', `Remove ${t.name}`);
  const li = el(
    'li',
    'sode__tielines-row',
    el('strong', null, t.name),
    el('span', 'sode__hint', `${endText(t.from, 'dst')} feeds ${endText(t.to, 'src')}`),
    edit,
    remove,
  );
  li.dataset.id = t.id;
  return li;
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
  if (!rows.length) rows.push(el('li', 'sode__hint', two ? 'No tie lines.' : 'Tie lines need two or more routers.'));
  list.replaceChildren(...rows);
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
