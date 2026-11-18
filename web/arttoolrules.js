// The Artwork tool in the editor's smart menu, pure: what one choice does to the selected cells. The page side is
// editorarttool.js. Only a cell's own `art`, `artB` and `hideText` change; Default artwork is never written.
import { rectOf, squareOrder, cellAt, setCell } from './gridmodel.js';
import { ART_B_CHOICES } from './panellook.js';
import { plural } from './gridclip.js';

// Icon B runs end at 16 for digits and Z for letters, never wrapping (panellook.js ART_B_CHOICES).
const runEnd = (i) => (i <= ART_B_CHOICES.indexOf('16') ? ART_B_CHOICES.indexOf('16') : ART_B_CHOICES.length - 1);

// The patch one choice gives each target in turn, for the smart menu, the Cell tab and a Grid panel's rows alike.
// choice is { a }, { b } or { clear: true }. a is Icon A, an `art` value. b is Icon B, the glyph the first target
// gets; each next target gets the next glyph. An undefined a or b clears it. The function gives null past the end of an Icon
// B run. null when the choice is no choice or an unknown glyph.
export function artRun(choice) {
  if (!choice) return null;
  if (choice.clear) return () => ({ art: undefined, artB: undefined, hideText: undefined });
  if ('b' in choice) {
    if (choice.b === undefined) return () => ({ artB: undefined });
    let next = ART_B_CHOICES.indexOf(choice.b);
    if (next < 0) return null;
    const end = runEnd(next);
    return () => (next > end ? null : { artB: ART_B_CHOICES[next++] });
  }
  return 'a' in choice ? () => ({ art: choice.a }) : null;
}

// order is one of gridmodel's FILL_ORDERS. Slots are { r, c, patch } for each cell in the selection, top left square
// first. empty counts squares with no cell, and left counts cells past the last glyph. null when there is nothing to plan.
export function artToolPlan(g, choice, order = 'across') {
  const q = rectOf(g);
  const run = artRun(choice);
  if (!q || !run) return null;
  const slots = [];
  let empty = 0;
  let left = 0;
  for (const [r, c] of squareOrder(q, order)) {
    const cell = cellAt(g, r, c);
    if (!cell) {
      empty++;
      continue;
    }
    if (cell.r !== r || cell.c !== c) continue; // a merged cell's other squares
    const patch = run();
    if (patch) slots.push({ r, c, patch });
    else left++;
  }
  return { slots, empty, left };
}

// What an Icon B run could not reach; unit is 'cell' or 'row'. '' when it reached every target.
export const leftNote = (left, unit) => (left ? `${plural(left, unit)} left without Icon B.` : '');

export function applyArtTool(g, plan) {
  if (!plan) return g;
  return plan.slots.reduce((out, s) => setCell(out, s, s.patch), g);
}
