// The take Confirm's question and plan item heads. Home's Confirm (plans.js) and a Grid panel's Confirm
// (gridpanelconfirm.js) both use them, so the two read the same. An item here is a label item { num, name } (labelNode).
import { el, labelNode } from './core.js';

export const strong = (t) => el('strong', null, t);

// The most destinations the question names one by one.
export const NAMED = 8;

// "a, b and c" from nodes.
export const nameList = (nodes) => nodes.flatMap((x, i) => (i === 0 ? [x] : [i === nodes.length - 1 ? ' and ' : ', ', x]));

// "Destination 61 MV 1 PIP 1" or "8 destinations".
export const destPart = (dsts) => (dsts.length === 1 ? ['Destination ', strong(labelNode(dsts[0]))] : [strong(`${dsts.length} destinations`)]);

// "Take Source 13 CAM 1 to 8 destinations: 61 MV 1 PIP 1, ... and 68 MV 4 PIP 2?". ask is the verb's words ("take",
// "retry the take of"). A take to many destinations names each one by number and name. So the count is never all the
// Confirm says, and the operator reads what will change before the take. Past NAMED it names the first NAMED - 1 and
// the count of the rest; the boxes below still name each one.
export function takeQuestion(ask, srcs, dsts) {
  const n = dsts.length;
  const shown = dsts.slice(0, n > NAMED ? NAMED - 1 : NAMED).map((d) => strong(labelNode(d)));
  const more = n - shown.length;
  const names = n > 1 ? [': ', ...nameList(more ? [...shown, `${more} more`] : shown)] : [];
  return el(
    'div',
    null,
    `${ask[0].toUpperCase()}${ask.slice(1)} `,
    ...(srcs.length === 1 ? ['Source ', strong(labelNode(srcs[0]))] : [strong(`${srcs.length} sources`)]),
    ' to ',
    ...destPart(dsts),
    ...names,
    '?',
  );
}

// The head of one destination's box in a plan of many: "1/8 61 MV 1 PIP 1", with an optional badge ("Staged").
export const planItemHead = (count, dst, badge = null) => el('div', 'sode__plan-head', el('span', 'sode__plan-n', count), strong(labelNode(dst)), badge);
