// A Grid panel's take confirm. It is the main page's churning-letter confirm (keyconfirm.js) over the #confirm markup
// in panel.html, with a summary in the main page's plan look. Levels and labels come from the adapter's GET /tally,
// since the panel page has no S.levels. Its lockout and Enter come from the panel read. The socket's hello fills
// S.cfg late, or never when the socket is blocked.
import { el, S } from './core.js';
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

const strong = (t) => el('strong', null, t);

function routeRow(tally, g, dstName) {
  return el(
    'div',
    'sode__route',
    levelBar(tally, g.levels),
    el('span', 'sode__end sode__end--src', g.srcName),
    el('span', 'sode__arrow', '→'),
    el('span', 'sode__end sode__end--dst', dstName),
  );
}

// What each destination has before the take, as the main page's confirm shows it (itemSummary in plans.js). A plan
// made without the tally has no now.
function nowRows(tally, item, boxed) {
  if (!item.now?.length) return [];
  const rows = [
    el('div', 'sode__label', 'Currently'),
    ...item.now.map((r) => el('div', 'sode__route', levelBar(tally, r.levels), el('span', 'sode__end sode__end--src', r.srcName))),
  ];
  return boxed ? [el('div', 'sode__plan-before', ...rows)] : rows;
}

const routesText = (n) => (n === 1 ? '1 staged route' : `${n} staged routes`);

// plan is planOf (gridpanelplan.js); tally() is the last GET /tally answer. A plan with verb 'undo' is this instance's
// Undo; with replace set too, it only asks to drop that many staged routes for the Undo.
export function gridSummary(plan, tally) {
  if (plan.replace) return [el('div', null, 'Are you sure you want to drop ', strong(routesText(plan.replace)), " and stage this panel's Undo?")];
  const n = plan.items.length;
  const srcs = new Set(plan.items.flatMap((i) => i.groups.map((g) => g.src)));
  const firstSrc = plan.items[0].groups[0].srcName;
  const question = el(
    'div',
    null,
    'Are you sure you want to take ',
    ...(srcs.size === 1 ? ['Source ', strong(firstSrc)] : [strong(`${srcs.size} sources`)]),
    ' to ',
    ...(n === 1 ? ['Destination ', strong(plan.items[0].dstName)] : [strong(`${n} destinations`)]),
    '?',
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
  if (plan.verb === 'undo') left.push(el('div', null, 'This undoes the last take of this panel instance.'));
  if (n === 1)
    return [...left, question, ...plan.items[0].groups.map((g) => routeRow(tally, g, plan.items[0].dstName)), ...nowRows(tally, plan.items[0], false)];
  return [
    ...left,
    question,
    ...plan.items.map((item, i) =>
      el(
        'section',
        'sode__plan-item',
        el('div', 'sode__plan-head', el('span', 'sode__plan-n', `${i + 1}/${n}`), strong(item.dstName)),
        ...item.groups.map((g) => routeRow(tally, g, item.dstName)),
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
    prompt: (key) => ['Press ', key, cfg()?.confirmEnter ? ' or Enter' : '', ' to confirm, or escape to cancel.'],
    onTake: (plan) => onTake(plan),
    onClose: (plan) => onClose(plan),
  });
}
