import test from 'node:test';
import assert from 'node:assert/strict';
import { chord, mac, listStep, resultStep, addClick, browseStep, primary, modChord, keyLabel, ariaKeys } from '../keys.js';

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

test('a closed patch box list opens on the arrows and on Ctrl/Cmd+J/K alike', () => {
  assert.equal(browseStep(k('ArrowDown', 'ArrowDown')), 1);
  assert.equal(browseStep(k('ArrowUp', 'ArrowUp')), -1);
  assert.equal(browseStep(k('j', 'KeyJ', { ctrlKey: true })), 1);
  assert.equal(browseStep(k('k', 'KeyK', { ctrlKey: true })), -1);
  assert.equal(browseStep(k('k', 'KeyK', { metaKey: true })), mac ? -1 : 0);
  assert.equal(browseStep(k('ArrowDown', 'ArrowDown', { altKey: true })), 0);
  assert.equal(browseStep(k('j', 'KeyJ')), 0);
});

test('Ctrl-click adds a destination; Cmd-click only on a Mac; Shift and Alt do not', () => {
  assert.equal(addClick(k('', '', { ctrlKey: true })), true);
  assert.equal(addClick(k('', '')), false);
  assert.equal(addClick(k('', '', { ctrlKey: true, shiftKey: true })), false);
  assert.equal(addClick(k('', '', { ctrlKey: true, altKey: true })), false);
  assert.equal(addClick(k('', '', { ctrlKey: true, metaKey: true })), false);
  assert.equal(addClick(k('', '', { metaKey: true })), mac);
});

test('the primary modifier is Ctrl, or Cmd on a Mac, never both and never the Windows key', () => {
  for (const isMac of [true, false]) {
    assert.equal(primary(k('', '', { ctrlKey: true }), isMac), true);
    assert.equal(primary(k('', '', { metaKey: true }), isMac), isMac);
    assert.equal(primary(k('', '', { ctrlKey: true, metaKey: true }), isMac), false);
    assert.equal(primary(k('', ''), isMac), false);
    assert.equal(listStep(k('j', 'KeyJ', { metaKey: true }), isMac), isMac ? 1 : 0);
  }
});

test('modChord names the primary modifier mod, with one name for each command', () => {
  assert.equal(modChord(k('z', 'KeyZ', { ctrlKey: true }), false), 'mod+z');
  assert.equal(modChord(k('z', 'KeyZ', { metaKey: true }), true), 'mod+z');
  assert.equal(modChord(k('Z', 'KeyZ', { ctrlKey: true, shiftKey: true }), true), 'mod+shift+z');
  assert.equal(modChord(k('Z', 'KeyZ', { metaKey: true, shiftKey: true }), true), 'mod+shift+z');
  assert.equal(modChord(k('Enter', 'Enter', { ctrlKey: true }), false), 'mod+enter');
  assert.equal(modChord(k('Home', 'Home', { metaKey: true }), true), 'mod+home');
  assert.equal(modChord(k('ArrowUp', 'ArrowUp', { ctrlKey: true, altKey: true }), false), 'mod+alt+up');
  assert.equal(modChord(k('z', 'KeyZ', { metaKey: true }), false), '', 'the Windows key');
  assert.equal(modChord(k('z', 'KeyZ', { ctrlKey: true, metaKey: true }), true), '');
  assert.equal(modChord(k('z', 'KeyZ'), true), '');
});

test('key labels: words off a Mac, glyphs on one, lower-case letters, ⇧ for Shift alone', () => {
  const both = (spec, pc, apple) => {
    assert.equal(keyLabel(spec, false), pc, spec);
    assert.equal(keyLabel(spec, true), apple, spec);
  };
  both('ctrl+e', 'Ctrl+e', '⌃e');
  both('ctrl+shift+s', 'Ctrl+Shift+s', '⌃⇧s');
  both('mod+enter', 'Ctrl+Enter', '⌘Enter');
  both('mod+shift+z', 'Ctrl+Shift+z', '⌘⇧z');
  both('alt+up', 'Alt+↑', '⌥↑');
  both('alt+shift+h', 'Alt+Shift+h', '⌥⇧h');
  both('alt+1', 'Alt+1', '⌥1');
  both('shift+enter', '⇧Enter', '⇧Enter');
  both('shift+j', '⇧j', '⇧j');
  both('shift', '⇧', '⇧');
  both('alt', 'Alt', '⌥');
  both('mod', 'Ctrl', '⌘');
  both('ctrl+?', 'Ctrl+?', '⌃?');
  both('ctrl+,', 'Ctrl+,', '⌃,');
  both('ctrl+/', 'Ctrl+/', '⌃/');
  both('mod+home', 'Ctrl+Home', '⌘Home');
  both('alt+shift+pageup', 'Alt+Shift+PgUp', '⌥⇧PgUp');
  both('escape', 'Esc', 'Esc');
});

test('aria-keyshortcuts names', () => {
  assert.equal(ariaKeys('ctrl+e'), 'Control+E');
  assert.equal(ariaKeys('alt+shift+h'), 'Alt+Shift+H');
  assert.equal(ariaKeys('alt+1'), 'Alt+1');
  assert.equal(ariaKeys('mod+g', true), 'Meta+G');
  assert.equal(ariaKeys('mod+g', false), 'Control+G');
  assert.equal(ariaKeys('escape'), 'Escape');
  assert.equal(ariaKeys('f'), 'F');
});
