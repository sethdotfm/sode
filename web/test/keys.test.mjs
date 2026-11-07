import test from 'node:test';
import assert from 'node:assert/strict';
import { chord, mac, listStep, resultStep } from '../keys.js';

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

test('Ctrl+J/K step a list; Shift and Alt do not; Cmd only on a Mac', () => {
  assert.equal(listStep(k('j', 'KeyJ', { ctrlKey: true })), 1);
  assert.equal(listStep(k('k', 'KeyK', { ctrlKey: true })), -1);
  assert.equal(listStep(k('j', 'KeyJ', { ctrlKey: true, shiftKey: true })), 0);
  assert.equal(listStep(k('j', 'KeyJ', { ctrlKey: true, altKey: true })), 0);
  assert.equal(listStep(k('j', 'KeyJ', { ctrlKey: true, metaKey: true })), 0);
  assert.equal(listStep(k('j', 'KeyJ')), 0);
  assert.equal(listStep(k('j', 'KeyJ', { metaKey: true })), mac ? 1 : 0);
});

test('search results take the arrows and Ctrl+J/K, and only when there are results', () => {
  assert.equal(resultStep(k('j', 'KeyJ', { ctrlKey: true }), true), 1);
  assert.equal(resultStep(k('k', 'KeyK', { ctrlKey: true }), true), -1);
  assert.equal(resultStep(k('ArrowDown', 'ArrowDown'), true), 1);
  assert.equal(resultStep(k('ArrowUp', 'ArrowUp'), true), -1);
  assert.equal(resultStep(k('ArrowDown', 'ArrowDown', { ctrlKey: true }), true), 0);
  // No results: the key falls through, so Settings steps sections.
  assert.equal(resultStep(k('j', 'KeyJ', { ctrlKey: true }), false), 0);
  assert.equal(resultStep(k('ArrowDown', 'ArrowDown'), false), 0);
});
