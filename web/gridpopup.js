// The Grid popup (GLOSSARY.md): the Grid panel body (gridpanelrun.js) over the main page, from an adapter with no saved
// panel. It shows every source and destination in the person's own Rights. Its staged routes go through the main page's
// Confirm and take path as one take, so History shows one Group. They stay in the staged list under the patch boxes
// until they are sent or cleared. The popup state and the list are page memory only: a reload or the Lock drops them.
// One Take: a confirm of the staged routes also holds the route in the patch boxes, when they hold one. The rules are in
// gridpoprules.js.
import { $, el, icon, S, on, emit, off, linked, srcItem, dstItem, labelNode } from './core.js';
import { pickOrAdd } from './keys.js';
import { gridBody } from './gridpanelrun.js';
import { levelsBox } from './panelmount.js';
import { boxHost } from './presets.js';
import { confirmShown } from './keyconfirm.js';
import { levelBar } from './levelview.js';
import { routesOf } from './gridpanelstage.js';
import { gridShield } from './gridpopshield.js';
import { tallyReplyOf, protectedReplyOf, namesReplyOf, aimOf, stagedRows, routesWord, withBoxRoute, takeCount } from './gridpoprules.js';
import { gridPlan, patchPlan } from './plans.js';
import { fadeOut, stopFade } from './modal.js';

// The panel the body runs: every item, Take mode, the person's own Rights. It is never saved and has no id.
const PANEL = { id: '', name: 'Grid', type: 'grid', sources: { members: 'all' }, destinations: { members: 'all' }, pages: [], settings: {} };
const FOCUSABLE = 'button:not([disabled]):not([hidden]), input:not([hidden]), [tabindex]:not([tabindex="-1"])';

let pop = null; // { root, shield, ui }, made on the first open
let body = null;
let opened = false; // this tab opened the popup before, so a reopen with no valid pair comes back as left
let shown = false;
let staged = []; // the body's staged entries, { dst, level, src }
let back = null; // what had focus before the popup opened
let scroll = null; // { left, top } of the grid when it closed
let tick = 0;
let frame = 0;
let namesNew = true;
// Counts the changes to what the tally and protected answers carry, so the body need not compare the answers as text.
let version = 0;

const scroller = () => pop?.ui.grid.querySelector('.sode-gridpanel__scroll');

// The popup only fills the staged list under the patch boxes. So on the desktop layout it is the grid and its search
// boxes. A press stages at the main page's levels, and the main page's Take sends it. The phone has no staged list, so
// there the Levels box, Take and Clear show (gridpopup.css).
function build() {
  const close = el('button', { class: 'pb-btn pb-btn--ghost pb-btn--icon', type: 'button', 'aria-label': 'Close', 'data-pb-tip': 'Close (Esc)' }, icon('x'));
  close.classList.add('pb-tip--end', 'sode-gridpop__close');
  close.addEventListener('click', () => hideGrid());
  // The body puts the search boxes in finds and its "N staged" badge before the spacer; gridpopup.css moves the badge
  // next to Close. The take result shows only on the main page, so the popup never changes height when a take runs.
  const finds = el('div', 'sode-gridpanel__finds');
  const help = el('span', 'sode-panels__spacer sode-gridpop__help sode__hint', 'Esc or Ctrl+e closes the grid. Take sends the staged routes.');
  const head = el('div', 'sode-gridpop__band', finds, help, close);
  const status = el('p', { class: 'sode-panel__status', role: 'status', hidden: true });
  const levels = levelsBox();
  const grid = el('div', 'sode-panel__grid');
  const box = el('div', { class: 'pb-modal sode-gridpop__box', role: 'dialog', 'aria-modal': 'true', 'aria-label': 'Grid' }, head, levels, status, grid);
  const root = el('div', { class: 'sode__scrim sode-gridpop', id: 'grid-pop', hidden: true }, box);
  root.addEventListener('keydown', popKey);
  const shield = gridShield(box, grid, (plan) => boxHost()?.confirm(plan));
  // A key move puts the shield on the cursor's destination name.
  root.addEventListener('keyup', (e) => !e.target.closest?.('input') && shield.cursor());
  document.body.append(root);
  return { root, shield, ui: { main: box, head, finds, status, grid, levels } };
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
  body.data({ tally, prot: protectedReplyOf(S.prot), nm, version });
  body.render();
}
// A large router sends many rows at once, so the popup reads them once a frame, and only while it shows.
const later = () => {
  if (shown) frame ||= requestAnimationFrame(() => ((frame = 0), push()));
};
// Only a tally, a protected change or a hello changes the tally, the protected list, the levels or the settings.
on('tally', () => {
  version++;
  later();
});
on('conn', later);
on('names', () => {
  namesNew = true;
  later();
});
on('hello', () => {
  version++;
  namesNew = true;
  later();
});
on('myRights', () => body?.read({ ...PANEL, rights: S.myRights }));
on('lockcover', ({ on: covers }) => {
  if (!covers) return;
  hideGrid(false);
  body?.drop();
});

let asked = false; // the last openConfirm of the body reached confirmRoutes

// The staged routes as the main page's plan, through the layout's own Confirm. It sends them as one take, with the
// patch boxes' route (host.route, desktop only) when the boxes hold one. Then a send clears the boxes too (resets), and
// the boxes' route becomes Last, as a patch of the boxes makes it (lastOf). The staged routes never make a Last: they
// can have many sources, so Last and Next would guess.
function confirmRoutes(routes, { warnings, onSent }) {
  const host = boxHost();
  if (!host) return;
  asked = true;
  const box = host.route?.();
  if (!box) return host.confirm({ ...gridPlan(routes, warnings), onSent });
  const all = withBoxRoute(routes, boxNumbers(box));
  host.confirm({ ...gridPlan(all, warnings), onSent, resets: true, lastOf: patchPlan(box.src, box.dsts, box.levels) });
}

const boxNumbers = (box) => ({ src: box.src.n, dsts: box.dsts.map((d) => d.n), levels: box.levels });

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
    emit('staged');
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
  stopFade(pop.root);
  pop.root.hidden = false;
  shown = true;
  body ||= gridBody(adapter(), { ...PANEL, rights: S.myRights });
  push();
  // The desktop popup has no Levels box of its own: a press stages at the main page's levels.
  if (aim?.levels || document.documentElement.dataset.layout !== 'phone') body.levels(p.levels);
  const sc = scroller();
  if (!aim && scroll && sc) sc.scrollTo(scroll.left, scroll.top);
  opened = true;
  clearInterval(tick);
  // Results and refusals fade on the clock, as on a panel page.
  tick = setInterval(() => body.render(), 1000);
  // The view measures itself in the first frame after it shows (its ResizeObserver). The aim waits one frame more. While
  // the popup was hidden the view measured a crosspoint as 0 px, and an aim with that size scrolls nowhere.
  requestAnimationFrame(() => {
    if (!shown) return;
    scroller()?.focus({ preventScroll: true });
    requestAnimationFrame(() => shown && aim && (aim.src != null || aim.dst != null) && body.aim(aim));
  });
}

export function hideGrid(refocus = true) {
  if (!shown) return;
  const sc = scroller();
  scroll = sc && { left: sc.scrollLeft, top: sc.scrollTop };
  fadeOut(pop.root);
  pop.shield.hide();
  shown = false;
  clearInterval(tick);
  if (refocus && back?.isConnected && !pop.root.contains(back)) back.focus({ preventScroll: true });
  back = null;
}

export const toggleGrid = () => (shown ? hideGrid() : showGrid());

// The staged list under the patch boxes (desktop). The phone has the popup's own Take and Clear. The main page's Take
// and Clear act on the list (desktop.js), so it has no buttons of its own.
function paintList() {
  const box = $('grid-staged');
  if (!box) return;
  const routes = routesOf({ staged });
  const rows = stagedRows(routes);
  box.hidden = !rows.length;
  $('grid-staged-count').textContent = routesWord(routes.length);
  $('grid-staged-list').replaceChildren(...rows.map(stagedRow));
}

// As a History row's route chips: a press puts the end in its patch box, Ctrl+click adds a destination. A chip of
// several destinations adds them all.
function endChip(k, label, tip, use, add) {
  const b = el('button', `sode__end sode__end--${k} sode__end--pick`, label);
  b.type = 'button';
  b.tabIndex = -1;
  b.dataset.pbTip = tip;
  b.addEventListener('mousedown', (e) => e.preventDefault());
  pickOrAdd(b, use, add);
  return b;
}

function stagedRow(row) {
  const find = el(
    'button',
    { class: 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm sode__staged-find', type: 'button', 'aria-label': 'Fill the patch boxes' },
    icon('crosshair'),
  );
  find.dataset.pbTip = 'Fill the patch boxes with this route. Ctrl+e finds it on the grid';
  find.addEventListener('mousedown', (e) => e.preventDefault());
  find.addEventListener('click', () => fillRow(row));
  const host = () => boxHost();
  const src = srcItem(row.src);
  const dsts = row.dsts.map((d) => dstItem(d));
  const addAll = () => dsts.forEach((d) => host()?.addEnd?.(d));
  const to =
    dsts.length === 1
      ? endChip('dst', labelNode(dsts[0]), 'Use as destination', () => host()?.useEnd?.('dst', dsts[0]), addAll)
      : endChip('dst', `${dsts.length} destinations`, 'Add all as destinations', addAll, addAll);
  return el(
    'li',
    'sode__route sode__staged-row',
    find,
    levelBar(row.levels),
    endChip('src', labelNode(src), 'Use as source', () => host()?.useEnd?.('src', src)),
    el('span', 'sode__arrow', '→'),
    to,
  );
}

// Fills the patch boxes from the row. It stages, sends and opens nothing: Ctrl+e then finds the boxes on the grid.
function fillRow(row) {
  boxHost()?.fill?.({ src: srcItem(row.src), dsts: row.dsts.map((d) => dstItem(d)), levels: row.levels });
}

// The main page's Take with staged routes (desktop.js). False when no confirm opened: no staged route applies now.
export const stagedCount = () => routesOf({ staged }).length;
// The routes the main page's Take sends now: box is host.route(), or null. 0 with nothing staged.
export const takeRoutes = (box) => takeCount(routesOf({ staged }), box && boxNumbers(box));
export function takeStaged() {
  if (!body) return false;
  push();
  asked = false;
  body.openConfirm();
  return asked;
}
// The main page's Clear and Esc (desktop.js).
export const dropStaged = () => body?.drop();

for (const [id, fn] of [
  ['grid-open', () => showGrid()],
  ['m-grid', () => showGrid()],
]) {
  const b = $(id);
  if (!b) continue;
  // The patch boxes keep focus, as for Take.
  if (id === 'grid-open') b.addEventListener('mousedown', (e) => e.preventDefault());
  b.addEventListener('click', fn);
}
