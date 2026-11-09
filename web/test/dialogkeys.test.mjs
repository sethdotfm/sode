import test from 'node:test';
import assert from 'node:assert/strict';
import { topDialog } from '../dialogkeys.js';

const d = (order, hidden = false, isConnected = true) => ({ order, root: { hidden, isConnected } });

test('topDialog: none open', () => {
  assert.equal(topDialog([]), null);
  assert.equal(topDialog([d(1, true), d(2, false, false)]), null);
});

test('topDialog: the most recently shown open one wins, whatever the list order', () => {
  const a = d(1),
    b = d(3),
    c = d(2);
  assert.equal(topDialog([a, b, c]), b);
  assert.equal(topDialog([b, a, c]), b);
});

test('topDialog: one hidden by someone else is skipped', () => {
  const a = d(1),
    b = d(2);
  b.root.hidden = true;
  assert.equal(topDialog([a, b]), a);
  b.root.hidden = false;
  assert.equal(topDialog([a, b]), b);
});

import { backdropClose, bringToFront } from '../dialogkeys.js';

test('backdropClose: only when mousedown and mouseup were both on the backdrop', () => {
  const root = {},
    inner = {};
  assert.equal(backdropClose(root, root, root), true);
  assert.equal(backdropClose(inner, root, root), false);
  assert.equal(backdropClose(root, inner, root), false);
  assert.equal(backdropClose(inner, inner, root), false);
  assert.equal(backdropClose(null, root, root), false);
  assert.equal(backdropClose(root, null, root), false);
});

test('bringToFront: moves a scrim to the end of its parent, keeps focus', () => {
  const kids = [];
  const parent = {
    get lastElementChild() {
      return kids[kids.length - 1];
    },
    appendChild(n) {
      const i = kids.indexOf(n);
      if (i >= 0) kids.splice(i, 1);
      kids.push(n);
      active = null;
    },
  };
  let active = null;
  const field = {
    focus() {
      active = field;
    },
  };
  const a = { parentNode: parent, contains: (x) => x === field };
  const b = { parentNode: parent, contains: () => false };
  kids.push(a, b);
  globalThis.document = {
    get activeElement() {
      return active;
    },
  };
  active = field;
  bringToFront(a);
  assert.deepEqual(kids, [b, a]);
  assert.equal(active, field);
  bringToFront(a);
  assert.deepEqual(kids, [b, a]);
  bringToFront({ parentNode: null });
  delete globalThis.document;
});

import { trapDialog, heldDialogs } from '../dialogkeys.js';

// A modal that is built per open (Share, Templates) is removed from the page on close; the Esc stack must let go of
// it too, or every open keeps a whole modal tree (and the icon sprite its <use> elements reference) alive.
test('trapDialog: hide drops the dialog from the Esc stack, show puts it back', () => {
  globalThis.document ??= { addEventListener() {} };
  const node = () => ({ hidden: false, isConnected: true, addEventListener() {}, querySelectorAll: () => [] });
  const before = heldDialogs();
  const a = trapDialog(node(), node(), () => {});
  const b = trapDialog(node(), node(), () => {});
  assert.equal(heldDialogs(), before + 2);
  a.hide();
  assert.equal(heldDialogs(), before + 1);
  a.show();
  assert.equal(heldDialogs(), before + 2);
  a.show(); // shown twice holds it once
  assert.equal(heldDialogs(), before + 2);
  a.hide();
  b.hide();
  assert.equal(heldDialogs(), before);
});
