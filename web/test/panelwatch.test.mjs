// Monitoring text and timing. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  durationText,
  noticeText,
  badgeText,
  deviceFrom,
  shouldNotify,
  keepaliveBody,
  KEEPALIVE_MS,
  permissionText,
  streamPath,
  retryMs,
  bannerText,
  clearNote,
  lostText,
  renameTo,
} from '../panelwatchrules.js';

test('durations read in the largest sensible unit', () => {
  assert.equal(durationText(1), '1 second');
  assert.equal(durationText(46), '46 seconds');
  assert.equal(durationText(60), '1 minute');
  assert.equal(durationText(125), '2 minutes 5 seconds');
  assert.equal(durationText(3600 + 300), '1 hour 5 minutes');
  assert.equal(durationText(2 * 86400), '2 days');
});

test('a notice names the panel, the device and how long', () => {
  const lost = noticeText({ state: 'lost', name: 'Studio wall', device: 'Lobby', secs: 50, offline: true });
  assert.equal(lost.kind, 'error');
  assert.match(lost.title, /Studio wall is offline/);
  assert.match(lost.text, /Lobby/);
  assert.match(lost.text, /50 seconds/);
  const one = noticeText({ state: 'lost', name: 'Studio wall', device: 'Lobby', secs: 50, offline: false });
  assert.equal(one.kind, 'warn');
  assert.match(one.title, /lost an instance/);
  const back = noticeText({ state: 'back', name: 'Studio wall', device: 'Lobby', secs: 130, offline: false });
  assert.equal(back.kind, 'ok');
  assert.match(back.text, /Lobby is back after 2 minutes 10 seconds/);
});

test('the badge counts panels', () => {
  assert.equal(badgeText([{}]), '1 panel offline');
  assert.equal(badgeText([{}, {}]), '2 panels offline');
});

test('a device name is trimmed and capped as the server caps it', () => {
  assert.equal(deviceFrom('  Lobby '), 'Lobby');
  assert.equal(deviceFrom('x'.repeat(60)).length, 40);
  assert.equal(deviceFrom(null), '');
  assert.deepEqual(JSON.parse(keepaliveBody(' Lobby ')), { device: 'Lobby' });
});

test('the keepalive is 15 seconds', () => assert.equal(KEEPALIVE_MS, 15000));

test('a browser notification fires whenever the person allowed them', () => {
  assert.equal(shouldNotify('granted'), true);
  assert.equal(shouldNotify('denied'), false);
  assert.equal(shouldNotify('default'), false);
});

test('the notification state tells the person what the browser allows', () => {
  assert.equal(permissionText('granted').state, 'on');
  assert.equal(permissionText('default').state, 'ask');
  assert.equal(permissionText('denied').state, 'blocked');
  assert.match(permissionText('denied').text, /blocked/);
  assert.equal(permissionText('unsupported').state, 'none');
});

test('the stream path carries the device and a retry backs off to 10 seconds', () => {
  assert.equal(streamPath('p-1', ' Lobby tablet '), 'api/v1/panels/p-1/instances/stream?device=Lobby%20tablet');
  assert.deepEqual([0, 1, 2, 3, 4, 9].map(retryMs), [1000, 2000, 4000, 8000, 10000, 10000]);
});

test('the banner names one panel, or counts several, and the Clear menu words follow', () => {
  const one = [{ panel: 'p-1', name: 'Wall', device: 'Lobby' }];
  assert.equal(bannerText(one).title, 'Wall is offline');
  assert.match(bannerText(one).text, /Lobby/);
  const two = [...one, { panel: 'p-2', name: 'Door', device: 'Hall' }];
  assert.equal(bannerText(two).title, '2 panels are offline');
  assert.equal(clearNote(one).stop, 'Stop monitoring this panel');
  assert.equal(clearNote(two).stop, 'Stop monitoring these panels');
});

test('a watched instance reads as an instance, and a mix is counted plainly', () => {
  const scr = { panel: 'p-1', name: 'Wall', device: 'Studio A', instance: 'studio a' };
  assert.equal(bannerText([scr]).title, 'Wall lost Studio A');
  assert.equal(badgeText([scr]), '1 instance offline');
  const mix = [scr, { panel: 'p-2', name: 'Door', device: 'Hall' }];
  assert.equal(badgeText(mix), '2 offline');
  assert.equal(bannerText(mix).title, '2 watched panels and instances are offline');
  assert.equal(bannerText(mix).text, 'Wall: Studio A, Door');
});

test('the lost line counts the silence up from the notice', () => {
  assert.equal(lostText('Lobby', 8), 'Lobby has not been heard from for 8 seconds.');
  assert.equal(lostText('Lobby', 8 + 55.4), 'Lobby has not been heard from for 1 minute 3 seconds.');
});

test('a rename is the trimmed name when it is new and not empty, else nothing', () => {
  assert.equal(renameTo('Lobby', '  Bar '), 'Bar');
  assert.equal(renameTo('Lobby', 'lobby'), 'lobby');
  assert.equal(renameTo('Lobby', ' Lobby '), '');
  assert.equal(renameTo('Lobby', '   '), '');
});
