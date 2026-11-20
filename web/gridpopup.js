// The Grid popup (GLOSSARY.md): the Grid panel body (gridpanelrun.js) over the main page, from an adapter with no saved
// panel. It shows every source and destination in the person's own Rights. Its staged routes go through the main page's
// Confirm and take path as one take, so History shows one Group. They stay in the staged list under the patch boxes
// until they are sent or cleared. The popup state and the list are page memory only: a reload or the Lock drops them.
// The rules are in gridpoprules.js.
import { $, el, icon, S, on, emit, off, linked, srcItem, dstItem, labelNode } from './core.js';
import { gridBody } from './gridpanelrun.js';
import { levelsBox } from './panelmount.js';
import { boxHost } from './presets.js';
import { confirmShown } from './keyconfirm.js';
import { resultNode } from './banner.js';
import { levelBar } from './levelview.js';
import { routesOf } from './gridpanelstage.js';
import { tallyReplyOf, protectedReplyOf, namesReplyOf, aimOf, stagedRows, routesWord, planRoutes } from './gridpoprules.js';

// The panel the body runs: every item, Take mode, the person's own Rights. It is never saved and has no id.
const PANEL = { id: '', name: 'Grid', type: 'grid', sources: { members: 'all' }, destinations: { members: 'all' }, pages: [], settings: {} };
const FOCUSABLE = 'button:not([disabled]):not([hidden]), input:not([hidden]), [tabindex]:not([tabindex="-1"])';

let pop = null; // { root, msg, ui }, made on the first open
let body = null;
let opened = false; // this tab opened the popup before, so a reopen with no valid pair comes back as left
let shown = false;
let staged = []; // the body's staged entries, { dst, level, src }
let back = null; // what had focus before the popup opened
let scroll = null; // { left, top } of the grid when it closed
let tick = 0;
let frame = 0;
let namesNew = true;

const scroller = () => pop?.ui.grid.querySelector('.sode-gridpanel__scroll');

function build() {
  const title = el('span', { class: 'pb-modal__title', id: 'grid-pop-title' }, 'Grid');
  const close = el('button', { class: 'pb-btn pb-btn--ghost pb-btn--icon', type: 'button', 'aria-label': 'Close', 'data-pb-tip': 'Close (Esc)' }, icon('x'));
  close.addEventListener('click', () => hideGrid());
  const head = el('div', 'pb-modal__header sode-gridpop__head', title, el('span', 'sode-panels__spacer'), close);
  const status = el('p', { class: 'sode-panel__status', role: 'status', hidden: true });
  const msg = el('div', { class: 'sode-gridpop__msg', 'aria-live': 'polite' });
  const levels = levelsBox();
  const grid = el('div', 'sode-panel__grid');
  const box = el(
    'div',
    { class: 'pb-modal sode-gridpop__box', role: 'dialog', 'aria-modal': 'true', 'aria-labelledby': 'grid-pop-title' },
    head,
    levels,
    msg,
    status,
    grid,
  );
  const root = el('div', { class: 'sode__scrim sode-gridpop', id: 'grid-pop', hidden: true }, box);
  root.addEventListener('keydown', popKey);
  document.body.append(root);
  return { root, msg, ui: { main: box, head, status, grid, levels } };
}

// After the grid's own keys (a capture listener on document), so its search boxes, line and picks take Esc first.
function popKey(e) {
  if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey) return;
  if (e.key === 'Escape') {
    e.preventDefault();
    e.stopPropagation();
    hideGrid();
  } else if (e.key === 'Tab') {
    // Focus stays in the popup, as in a dialog.
    const all = [...pop.root.querySelectorAll(FOCUSABLE)].filter((n) => n.offsetParent);
    const at = all.indexOf(document.activeElement);
    const next = e.shiftKey ? all[(at <= 0 ? all.length : at) - 1] : all[(at + 1) % all.length];
    if (!next) return;
    e.preventDefault();
    next.focus();
  }
}

// The main page's state as the /api/v1 answers that the body reads.
function push() {
  if (!body || !S.cfg) return;
  const nm = namesNew ? namesReplyOf({ srcs: S.srcs, dsts: S.dsts, numberOffset: off() }) : null;
  namesNew = false;
  const tally = tallyReplyOf({ levels: S.levels, levelNumbering: S.cfg.levelNumbering, defaultLevels: S.cfg.defaultLevels, tally: S.tally });
  body.data({ tally, prot: protectedReplyOf(S.prot), nm });
  body.render();
}
// A large router sends many rows at once, so the popup reads them once a frame, and only while it shows.
const later = () => {
  if (shown) frame ||= requestAnimationFrame(() => ((frame = 0), push()));
};
on('tally', later);
on('conn', later);
for (const ev of ['names', 'hello'])
  on(ev, () => {
    namesNew = true;
    later();
  });
on('myRights', () => body?.read({ ...PANEL, rights: S.myRights }));
on('result', (r) => shown && pop.msg.replaceChildren(resultNode(r)));
on('lockcover', ({ on: covers }) => {
  if (!covers) return;
  hideGrid(false);
  body?.drop();
});

// The staged routes as the main page's plan, through the layout's own Confirm. It sends them as one take.
function confirmRoutes(routes, { warnings, onSent }) {
  const host = boxHost();
  if (!host) return;
  const items = planRoutes(routes).map((it) => ({
    dst: dstItem(it.dst),
    groups: it.groups.map((g) => ({ src: srcItem(g.src), levels: g.levels })),
    skipped: [],
    untouched: [],
    undo: null,
    retry: null,
  }));
  host.confirm({ verb: 'grid', warnings, items, onSent });
}

const adapter = () => ({
  ui: pop.ui,
  art: null,
  embedded: false,
  page: '',
  id: '',
  ready: linked,
  touched() {},
  poll: later,
  confirm: confirmRoutes,
  // The phone's Confirm is its slide sheet.
  asking: () => confirmShown() || !$('m-sheet').hidden,
  shown: () => shown,
  undo: () => emit('undoLast'),
  onStage(list) {
    staged = list;
    paintList();
  },
});

export const gridShown = () => shown;

// Opens the popup over the layout that shows. A valid pair in the patch boxes centres it; with none it comes back as left.
export function showGrid() {
  const host = boxHost();
  if (!host || !S.cfg || document.documentElement.dataset.sodeLock) return;
  pop ||= build();
  const p = host.picks();
  const aim = aimOf({ src: p.src?.n ?? null, dst: p.dsts[0]?.n ?? null, first: !opened });
  if (!shown) back = document.activeElement;
  pop.msg.replaceChildren();
  pop.root.hidden = false;
  shown = true;
  body ||= gridBody(adapter(), { ...PANEL, rights: S.myRights });
  push();
  if (aim?.levels) body.levels(p.levels);
  const sc = scroller();
  if (!aim && scroll && sc) sc.scrollTo(scroll.left, scroll.top);
  opened = true;
  clearInterval(tick);
  // Results and refusals fade on the clock, as on a panel page.
  tick = setInterval(() => body.render(), 1000);
  // The view measures itself in the first frame after it shows.
  requestAnimationFrame(() => {
    if (!shown) return;
    if (aim && (aim.src != null || aim.dst != null)) body.aim(aim);
    scroller()?.focus({ preventScroll: true });
  });
}

export function hideGrid(refocus = true) {
  if (!shown) return;
  const sc = scroller();
  scroll = sc && { left: sc.scrollLeft, top: sc.scrollTop };
  pop.root.hidden = true;
  shown = false;
  clearInterval(tick);
  if (refocus && back?.isConnected && !pop.root.contains(back)) back.focus({ preventScroll: true });
  back = null;
}

export const toggleGrid = () => (shown ? hideGrid() : showGrid());

// The staged list under the patch boxes (desktop). The phone has the popup's own Take and Clear.
function paintList() {
  const box = $('grid-staged');
  if (!box) return;
  const routes = routesOf({ staged });
  const rows = stagedRows(routes);
  box.hidden = !rows.length;
  $('grid-staged-take').textContent = `Take ${routesWord(routes.length)}`;
  $('grid-staged-list').replaceChildren(...rows.map(stagedRow));
}

function stagedRow(row) {
  const find = el(
    'button',
    { class: 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm sode__staged-find', type: 'button', 'aria-label': 'Find on the grid' },
    icon('crosshair'),
  );
  find.dataset.pbTip = 'Fill the patch boxes and find it on the grid';
  find.addEventListener('click', () => findRow(row));
  const to = row.dsts.length === 1 ? labelNode(dstItem(row.dsts[0])) : `${row.dsts.length} destinations`;
  return el(
    'li',
    'sode__route sode__staged-row',
    find,
    levelBar(row.levels),
    el('span', 'sode__end sode__end--src', labelNode(srcItem(row.src))),
    el('span', 'sode__arrow', '→'),
    el('span', 'sode__end sode__end--dst', to),
  );
}

// Fills the patch boxes from the row, then opens the grid on its first staged crosspoint. It stages and sends nothing.
function findRow(row) {
  const host = boxHost();
  if (!host?.fill) return;
  host.fill({ src: srcItem(row.src), dsts: row.dsts.map((d) => dstItem(d)), levels: row.levels });
  showGrid();
}

function listTake() {
  if (!body) return;
  push();
  body.openConfirm();
}

for (const [id, fn] of [
  ['grid-staged-take', listTake],
  ['grid-staged-clear', () => body?.drop()],
  ['grid-open', () => showGrid()],
  ['m-grid', () => showGrid()],
]) {
  const b = $(id);
  if (!b) continue;
  // The patch boxes keep focus, as for Take.
  if (id === 'grid-open') b.addEventListener('mousedown', (e) => e.preventDefault());
  b.addEventListener('click', fn);
}
