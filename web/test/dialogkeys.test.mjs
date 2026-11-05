import test from 'node:test';
import assert from 'node:assert/strict';
import { topDialog } from '../dialogkeys.js';

const d = (order, hidden = false, isConnected = true) => ({ order, root: { hidden, isConnected } });

test('topDialog: none open', () => {
  assert.equal(topDialog([]), null);
  assert.equal(topDialog([d(1, true), d(2, false, false)]), null);
});

test('topDialog: the most recently shown open one wins, whatever the list order', () => {
  const a = d(1), b = d(3), c = d(2);
  assert.equal(topDialog([a, b, c]), b);
  assert.equal(topDialog([b, a, c]), b);
});

test('topDialog: one hidden by someone else is skipped', () => {
  const a = d(1), b = d(2);
  b.root.hidden = true;
  assert.equal(topDialog([a, b]), a);
  b.root.hidden = false;
  assert.equal(topDialog([a, b]), b);
});

import { backdropClose, bringToFront } from '../dialogkeys.js';

// The targets are those of mousedown and mouseup. A press on the backdrop
// released inside the dialog (or the reverse, or a selection dragged out)
// still fires a click on the common ancestor, root, so the click's target says
// nothing: only where the mouse went down and came up counts.
test('backdropClose: only when mousedown and mouseup were both on the backdrop', () => {
  const root = {}, inner = {};
  assert.equal(backdropClose(root, root, root), true);
  assert.equal(backdropClose(inner, root, root), false); // selection dragged out
  assert.equal(backdropClose(root, inner, root), false); // pressed outside, released in
  assert.equal(backdropClose(inner, inner, root), false);
  assert.equal(backdropClose(null, root, root), false);
  assert.equal(backdropClose(root, null, root), false); // no mouseup seen
});

test('bringToFront: moves a scrim to the end of its parent, keeps focus', () => {
  const kids = [];
  const parent = {
    get lastElementChild() { return kids[kids.length - 1]; },
    appendChild(n) { const i = kids.indexOf(n); if (i >= 0) kids.splice(i, 1); kids.push(n); active = null; },
  };
  let active = null;
  const field = { focus() { active = field; } };
  const a = { parentNode: parent, contains: (x) => x === field };
  const b = { parentNode: parent, contains: () => false };
  kids.push(a, b);
  globalThis.document = { get activeElement() { return active; } };
  active = field;
  bringToFront(a);
  assert.deepEqual(kids, [b, a]);
  assert.equal(active, field);
  bringToFront(a); // already last: nothing moves
  assert.deepEqual(kids, [b, a]);
  bringToFront({ parentNode: null }); // detached: no error
  delete globalThis.document;
});
