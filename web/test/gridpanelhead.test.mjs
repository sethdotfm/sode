// A Grid panel's words and banner rule (gridpanelhead.js).
import test from 'node:test';
import assert from 'node:assert/strict';
import { emptyText, bannerStep } from '../gridpanelhead.js';

test('a grid with no sources or no destinations says so, and how to add them', () => {
  assert.equal(emptyText({ srcs: 3, dsts: 2, canEdit: true }), null);
  assert.deepEqual(emptyText({ srcs: 0, dsts: 0, canEdit: true }), {
    title: 'This panel shows no sources or destinations',
    msg: 'Add them in the editor.',
    edit: true,
  });
  assert.equal(emptyText({ srcs: 0, dsts: 4, canEdit: false }).title, 'This panel shows no sources');
  assert.equal(emptyText({ srcs: 4, dsts: 0, canEdit: false }).msg, 'The owner of the panel adds them in the editor.');
});

test('banner: a result that went well never takes the place of an error, and counts as held back', () => {
  const err = bannerStep(null, { text: 'Rec B is protected', kind: 'error', key: 'Rec B is protected', held: true }).shown;
  const ok = bannerStep(err, { text: 'Taken: 1 route', kind: 'ok', key: '5' });
  assert.equal(ok.shown, err);
  assert.equal(ok.back, true);
  // Once its X closes it, the next result shows.
  const after = bannerStep({ ...err, gone: true }, { text: 'Taken: 1 route', kind: 'ok', key: '6' });
  assert.equal(after.shown.title, 'Taken: 1 route');
});

test('banner: a held state goes when it clears; a closed message does not come back', () => {
  const held = bannerStep(null, { text: 'No levels', kind: 'error', key: 'No levels', held: true }).shown;
  assert.equal(bannerStep(held, { text: '' }).shown, null);
  const note = bannerStep(null, { text: 'Sode did not answer', kind: 'error', key: '1' }).shown;
  assert.equal(bannerStep(note, { text: '' }).shown, note);
  assert.equal(bannerStep({ ...note, gone: true }, { text: 'Sode did not answer', kind: 'error', key: '1' }).changed, false);
});

test('banner: errors of one group join; another error takes the place', () => {
  let s = bannerStep(null, { text: 'Take failed on CAM 1', kind: 'error', key: '1' }).shown;
  s = bannerStep(s, { text: 'Take failed on CAM 2, CAM 3', kind: 'error', key: '2' }).shown;
  assert.deepEqual([s.title, s.line], ['2 takes failed', 'On CAM 1, CAM 2, CAM 3']);
  s = bannerStep(s, { text: 'Sode did not answer', kind: 'error', key: '3' }).shown;
  assert.deepEqual([s.title, s.line], ['Sode did not answer', '']);
  s = bannerStep(s, { text: 'Sode did not answer', kind: 'error', key: '4' }).shown;
  assert.equal(s.title, 'Sode did not answer (2 times)');
  // A result that went well after a result that went well takes its place.
  const a = bannerStep(null, { text: 'Taken: 1 route', kind: 'ok', key: '7' }).shown;
  assert.equal(bannerStep(a, { text: 'Taken: 2 routes', kind: 'ok', key: '8' }).shown.title, 'Taken: 2 routes');
});
