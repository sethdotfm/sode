// The editor grid's Cmd/Ctrl commands (gridchords.js). Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { GRID_CHORDS, gridCommand } from '../gridchords.js';
import { chord } from '../keys.js';

const k = (key, code, mods = {}, target = null) => ({ key, code, target, ctrlKey: false, altKey: false, shiftKey: false, metaKey: false, ...mods });

test('Ctrl+A and Cmd+A both select all, on any platform', () => {
  assert.equal(gridCommand(k('a', 'KeyA', { ctrlKey: true })), 'selectAll');
  assert.equal(gridCommand(k('a', 'KeyA', { metaKey: true })), 'selectAll');
});

test('the physical key counts, so another keyboard layout still selects all', () => {
  assert.equal(gridCommand(k('ф', 'KeyA', { ctrlKey: true })), 'selectAll');
  assert.equal(gridCommand(k('q', 'KeyA', { metaKey: true })), 'selectAll');
});

test('an extra modifier or no modifier is not the command', () => {
  for (const mods of [
    {},
    { shiftKey: true },
    { ctrlKey: true, shiftKey: true },
    { ctrlKey: true, altKey: true },
    { metaKey: true, altKey: true },
    { ctrlKey: true, metaKey: true },
  ]) {
    assert.equal(gridCommand(k('a', 'KeyA', mods)), null, JSON.stringify(mods));
  }
  assert.equal(gridCommand(k('b', 'KeyB', { ctrlKey: true })), null);
});

test('a focused text field keeps its own keys', () => {
  const ctrlA = (target) => gridCommand(k('a', 'KeyA', { ctrlKey: true }, target));
  assert.equal(ctrlA({ tagName: 'INPUT', type: 'text' }), null);
  assert.equal(ctrlA({ tagName: 'INPUT', type: 'search' }), null);
  assert.equal(ctrlA({ tagName: 'TEXTAREA' }), null);
  assert.equal(ctrlA({ tagName: 'DIV', isContentEditable: true }), null);
  assert.equal(ctrlA({ tagName: 'INPUT', type: 'checkbox' }), 'selectAll');
  assert.equal(ctrlA({ tagName: 'DIV' }), 'selectAll');
});

test('every table key is spelled the way chord() names it', () => {
  const mods = { ctrl: 'ctrlKey', alt: 'altKey', shift: 'shiftKey', meta: 'metaKey' };
  for (const name of Object.keys(GRID_CHORDS)) {
    const parts = name.split('+');
    const letter = parts.pop();
    const e = k(letter, `Key${letter.toUpperCase()}`, Object.fromEntries(parts.map((p) => [mods[p], true])));
    assert.equal(chord(e), name);
  }
});

test('Cmd or Ctrl+X is cut on the grid, and a focused text field keeps its own', () => {
  assert.equal(gridCommand(k('x', 'KeyX', { ctrlKey: true })), 'cut');
  assert.equal(gridCommand(k('x', 'KeyX', { metaKey: true })), 'cut');
  assert.equal(gridCommand(k('x', 'KeyX', { metaKey: true, shiftKey: true })), null);
  assert.equal(gridCommand({ ...k('x', 'KeyX', { ctrlKey: true }), target: { tagName: 'INPUT', type: 'text' } }), null);
});

test('Cmd or Ctrl+Z undoes and Cmd or Ctrl+Shift+Z redoes; Ctrl+Y is not redo', () => {
  assert.equal(gridCommand(k('z', 'KeyZ', { ctrlKey: true })), 'undo');
  assert.equal(gridCommand(k('z', 'KeyZ', { metaKey: true })), 'undo');
  assert.equal(gridCommand(k('Z', 'KeyZ', { ctrlKey: true, shiftKey: true })), 'redo');
  assert.equal(gridCommand(k('Z', 'KeyZ', { metaKey: true, shiftKey: true })), 'redo');
  assert.equal(gridCommand(k('y', 'KeyY', { ctrlKey: true })), null);
  assert.equal(gridCommand(k('z', 'KeyZ', { ctrlKey: true, altKey: true })), null);
  assert.equal(gridCommand(k('z', 'KeyZ', { metaKey: true }, { tagName: 'INPUT', type: 'text' })), null);
  assert.equal(gridCommand(k('Z', 'KeyZ', { metaKey: true, shiftKey: true }, { tagName: 'TEXTAREA' })), null);
});
