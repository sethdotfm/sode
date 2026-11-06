// chord() names a keystroke the same way everywhere.
import test from 'node:test';
import assert from 'node:assert/strict';
import { chord } from '../keys.js';

const k = (key, code, mods = {}) => ({ key, code, ctrlKey: false, altKey: false, shiftKey: false, metaKey: false, ...mods });

test('letters and digits go by the physical key, so Option does not rename them', () => {
  assert.equal(chord(k('π', 'KeyP', { altKey: true })), 'alt+p');
  assert.equal(chord(k('¡', 'Digit1', { altKey: true })), 'alt+1');
});

test('modifiers come in a fixed order', () => {
  assert.equal(chord(k('j', 'KeyJ', { metaKey: true, altKey: true })), 'alt+meta+j');
  assert.equal(chord(k('j', 'KeyJ', { ctrlKey: true, altKey: true })), 'ctrl+alt+j');
  assert.equal(chord(k('Enter', 'Enter', { shiftKey: true })), 'shift+enter');
});

test('arrows are up/down/left/right; other keys are their lowercase name', () => {
  assert.equal(chord(k('ArrowUp', 'ArrowUp', { altKey: true })), 'alt+up');
  assert.equal(chord(k('Escape', 'Escape')), 'escape');
});
