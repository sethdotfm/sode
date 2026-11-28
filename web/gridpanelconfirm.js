// A Grid panel's take confirm. It is the main page's churning-letter confirm (keyconfirm.js) over the #confirm markup
// in panel.html. A take asks "Take 2 routes?". Each route is one line of text, in destination number order: the
// destination, the new source, what it replaces and the levels. A level bar shows only when the routes' levels differ.
// An Undo reads as Home's Confirm (planSummary in plans.js): the same question and item heads (confirmtext.js),
// numbered ends and Replacing. Levels and labels come from the adapter's GET /tally,
// since the panel page reads no levels from the hello. Its lockout and Enter come from the panel read. The socket's hello fills
// S.cfg late, or never when the socket is blocked.
import { el, S, labelNode } from './core.js';
import { strong, takeQuestion, planItemHead, nameList } from './confirmtext.js';
import { keyConfirm } from './keyconfirm.js';
import { levelColor } from './levelbox.js';
import { levelTextOf } from './levels.js';

// The main page's level bar (levelview.js levelBar) with the levels passed in: a fixed slot per level.
function levelBar(tally, on) {
  const levels = tally.levels;
  const set = new Set(on);
  const row = el(
    'span',
    'sode__bar',
    ...levels.map((l) => {
      const slot = el('span', 'sode__slot' + (set.has(l.level) ? ' sode__slot--on' : ''), l.label);
      slot.dataset.hint = `Level ${l.label}`;
      return levelColor(slot, l);
    }),
  );
  const odd = levels.length % 2;
  if (odd) row.classList.add('sode__bar--odd');
  row.style.setProperty('--sode-bar-cols', Math.floor(levels.length / 2) + odd);
  row.setAttribute('role', 'img');
  row.setAttribute('aria-label', `Levels ${levelTextOf(on, levels, tally.levelNumbering)}`);
  return row;
}

// An end as Home's Confirm shows it: number and name, or the plain name for an end with no label item.
const endText = (label, name) => (label ? labelNode(label) : name);

function routeRow(tally, g, item) {
  return el(
    'div',
    'sode__route',
    levelBar(tally, g.levels),
    el('span', 'sode__end sode__end--src', endText(g.srcLabel, g.srcName)),
    el('span', 'sode__arrow', '→'),
    el('span', 'sode__end sode__end--dst', endText(item.dstLabel, item.dstName)),
  );
}

// What each destination has before the take, as the main page's confirm shows it under Replacing (itemSummary in
// plans.js). A plan made without the tally has no now.
function nowRows(tally, item, boxed) {
  if (!item.now?.length) return [];
  const rows = [
    el('div', 'sode__label', 'Replacing'),
    ...item.now.map((r) => el('div', 'sode__route', levelBar(tally, r.levels), el('span', 'sode__end sode__end--src', endText(r.srcLabel, r.srcName)))),
  ];
  return boxed ? [el('div', 'sode__plan-before', ...rows)] : rows;
}

const routesText = (n) => (n === 1 ? '1 staged route' : `${n} staged routes`);

// The sources a take replaces on the levels in on. The source being taken is no replacement: a route already there reads
// as no change, not "X replaces X".
export const replacedOn = (now, src, on) => (now || []).filter((r) => r.src !== src && r.levels.some((l) => on.has(l)));

const routeCount = (n) => (n === 1 ? '1 route' : `${n} routes`);
const sameLevels = (a, b) => a.length === b.length && a.every((l, i) => l === b[i]);

// A take: "Take 2 routes?", then per route "A.4 DSM2: A.12 Zoom 2 replaces A.11 Zoom 1 on levels 1–2". A destination
// with two sources on different levels has a line for each. The levels are words; a level bar leads each line only when
// the lines' levels differ. A plan made without the tally says nothing of what it replaces.
function takeRows(tally, plan) {
  const lines = plan.items.flatMap((item) => item.groups.map((g) => ({ item, g })));
  const bars = lines.some(({ g }) => !sameLevels(g.levels, lines[0].g.levels));
  const levelWords = (list) => `${list.length === 1 ? 'level' : 'levels'} ${levelTextOf(list, tally.levels, tally.levelNumbering)}`;
  const row = ({ item, g }) => {
    const on = new Set(g.levels);
    const gone = replacedOn(item.now, g.src, on);
    const before = gone.length ? [' replaces ', ...nameList(gone.map((r) => strong(endText(r.srcLabel, r.srcName))))] : [];
    return el(
      'div',
      'sode-gridpanel__plan-row',
      ...(bars ? [levelBar(tally, g.levels)] : []),
      el('span', null, strong(endText(item.dstLabel, item.dstName)), ': ', strong(endText(g.srcLabel, g.srcName)), ...before, ` on ${levelWords(g.levels)}`),
    );
  };
  return [el('div', null, `Take ${routeCount(lines.length)}?`), ...lines.map(row)];
}

// plan is planOf (gridpanelplan.js); tally() is the last GET /tally answer. A plan with verb 'undo' is this instance's
// Undo; with replace set too, it only asks to drop that many staged routes for the Undo.
export function gridSummary(plan, tally) {
  if (plan.replace) return [el('div', null, 'Drop ', strong(routesText(plan.replace)), " and stage this panel's Undo?")];
  const n = plan.items.length;
  const label = (lbl, name) => lbl || { num: '', name };
  const srcs = new Map();
  for (const g of plan.items.flatMap((i) => i.groups)) if (!srcs.has(g.src)) srcs.set(g.src, label(g.srcLabel, g.srcName));
  // As Home's Confirm asks: "Take Source 13 CAM 1 to 4 destinations: 61 MV 1 PIP 1, ...?". An Undo asks as Home's Retry.
  const question = takeQuestion(
    plan.verb === 'undo' ? 'undo the take of' : 'take',
    [...srcs.values()],
    plan.items.map((i) => label(i.dstLabel, i.dstName)),
  );
  // As the main page's planSummary (plans.js) shows what a plan leaves out.
  const left = plan.warnings?.length
    ? [
        el(
          'div',
          'pb-alert pb-alert--warn',
          el('div', null, el('div', 'pb-alert__title', `${plan.warnings.length} left out`), ...plan.warnings.map((l) => el('div', null, l))),
        ),
      ]
    : [];
  if (plan.verb === 'undo') left.push(el('div', null, "Undo of this instance's last take."));
  // A take across routers names its Reserved tie lines and the Affected destinations (docs/tielines-design.md, Panels).
  if (plan.also?.length) left.push(el('div', 'sode__plan-also', el('div', 'sode__label', 'Tie lines'), ...plan.also.map((l) => el('div', null, l))));
  if (plan.verb !== 'undo') return [...left, ...takeRows(tally, plan)];
  if (n === 1) return [...left, question, ...plan.items[0].groups.map((g) => routeRow(tally, g, plan.items[0])), ...nowRows(tally, plan.items[0], false)];
  return [
    ...left,
    question,
    ...plan.items.map((item, i) =>
      el(
        'section',
        'sode__plan-item',
        planItemHead(`${i + 1}/${n}`, label(item.dstLabel, item.dstName)),
        ...item.groups.map((g) => routeRow(tally, g, item)),
        ...nowRows(tally, item, true),
      ),
    ),
  ];
}

// onTake(plan) sends; onClose(plan) is a cancel. settings() is the panel read's confirm, or null before a read.
export function gridConfirm({ tally, settings, onTake, onClose }) {
  const cfg = () => {
    const c = settings();
    return c ? { confirmLockoutMs: c.lockoutMs, confirmEnter: c.enter } : S.cfg;
  };
  return keyConfirm({
    cfg,
    verb: (plan) => plan.verb,
    summary: (plan) => gridSummary(plan, tally()),
    // As on the main page (desktop.js): Enter also confirms when the setting says so.
    enter: () => !!cfg()?.confirmEnter,
    refocus: true,
    prompt: (key, plan) => ['Press ', key, cfg()?.confirmEnter ? ' or Enter' : '', ` to ${plan?.verb === 'undo' ? 'confirm' : 'take'}. Press Esc to cancel.`],
    onTake: (plan) => onTake(plan),
    onClose: (plan) => onClose(plan),
  });
}
