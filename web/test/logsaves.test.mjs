import test from 'node:test';
import assert from 'node:assert/strict';
import { fieldText, saveSummary } from '../logsaves.js';

const base = { topic: 'Look and feel', section: 'Branding', label: 'Logo text & Page title' };

test('a field reads with its section, label, old and new value', () => {
  assert.equal(fieldText({ ...base, old: 'Sode', new: 'Studio B' }), 'Branding: Logo text & Page title: from "Sode" to "Studio B"');
  assert.equal(fieldText({ ...base, old: false, new: true }), 'Branding: Logo text & Page title: from Off to On');
  assert.equal(fieldText({ ...base, new: 'x' }), 'Branding: Logo text & Page title: from Blank to "x"');
});

test('a list reads as added and removed, a secret or hidden value as changed', () => {
  assert.equal(
    fieldText({ ...base, added: ['10.1.0.0/16'], removed: ['10.2.0.0/16', '10.3.0.0/16'] }),
    'Branding: Logo text & Page title: added 10.1.0.0/16; removed 10.2.0.0/16, 10.3.0.0/16',
  );
  assert.equal(fieldText({ ...base, changed: true }), 'Branding: Logo text & Page title: changed');
  assert.equal(
    fieldText({ ...base, changed: true, note: 'panel clients they created: P' }),
    'Branding: Logo text & Page title: changed (panel clients they created: P)',
  );
});

test('the summary counts fields and names the Critical mark, approver and preset', () => {
  assert.equal(saveSummary({ fields: [base] }), '1 field');
  assert.equal(
    saveSummary({ fields: [base, base], critical: true, approver: 'Ada', preset: 'Secured' }),
    '2 fields, Critical, approved by Ada, preset Secured',
  );
});

test('the summary counts the Interruptions, the hidden ones too', () => {
  assert.equal(saveSummary({ fields: [base], interruptions: ['Public panel Lobby stops'] }), '1 field, 1 Interruption');
  assert.equal(saveSummary({ fields: [base], hiddenInterruptions: 2 }), '1 field, 2 Interruptions');
});

test('an import row reads as counts per category, never values', () => {
  const im = {
    mode: 'add',
    categories: [
      { category: 'presets', added: 2, updated: 1, broken: 1 },
      { category: 'filterSets', kept: 3 },
      { category: 'users', refused: true },
    ],
  };
  assert.equal(
    saveSummary({ fields: [], critical: true, import: im }),
    'Add import: Presets 2 added, 1 updated, 1 Broken; Filter sets 3 kept; Users not imported, Critical',
  );
  const rep = { mode: 'replace', categories: [{ category: 'presets', updated: 1, removed: 2 }] };
  assert.equal(saveSummary({ fields: [], critical: true, import: rep }), 'Replace import: Presets 1 updated, 2 removed, Critical');
});
