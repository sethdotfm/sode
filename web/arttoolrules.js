// The Artwork tool in the editor's smart menu, pure: what one choice does to the selected cells. The page side is
// editorarttool.js. Only a cell's own `art`, `artB` and `hideText` change; Default artwork is never written.
import { rectOf, squareOrder, cellAt, setCell } from './gridmodel.js';
import { ART_B_CHOICES } from './panellook.js';

// Icon B runs end at 16 for digits and Z for letters, never wrapping (panellook.js ART_B_CHOICES).
const runEnd = (i) => (i <= ART_B_CHOICES.indexOf('16') ? ART_B_CHOICES.indexOf('16') : ART_B_CHOICES.length - 1);

// choice is { a } (Icon A: an `art` value), { b } (Icon B: the glyph the first cell gets) or { clear: true }; order is
// one of gridmodel's FILL_ORDERS. Slots are { r, c, patch } for each cell in the selection, top left square first;
// empty counts squares with no cell, left counts cells past the last glyph. null when there is nothing to plan.
export function artToolPlan(g, choice, order = 'across') {
  const q = rectOf(g);
  if (!q || !choice) return null;
  let next = -1;
  if ('b' in choice) {
    next = ART_B_CHOICES.indexOf(choice.b);
    if (next < 0) return null;
  } else if (!choice.clear && !choice.a) return null;
  const end = runEnd(next);
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
    if (choice.clear) slots.push({ r, c, patch: { art: undefined, artB: undefined, hideText: undefined } });
    else if (next < 0) slots.push({ r, c, patch: { art: choice.a } });
    else if (next > end) left++;
    else slots.push({ r, c, patch: { artB: ART_B_CHOICES[next++] } });
  }
  return { slots, empty, left };
}

export function applyArtTool(g, plan) {
  if (!plan) return g;
  return plan.slots.reduce((out, s) => setCell(out, s, s.patch), g);
}
