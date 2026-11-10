import test from 'node:test';
import assert from 'node:assert/strict';
import {
  MODES,
  INSTANCE_MAX,
  INSTANCE_HOLD,
  SESSION_WARNING,
  instanceCheck,
  keyRow,
  firstShareTab,
  absoluteLink,
  maskLink,
  makeAction,
  confirmKind,
  parseNetworks,
  networksChanged,
  setupFromHash,
  withoutSetup,
  withoutKey,
  storedKeyName,
  pollFail,
  setupFailText,
} from '../panelkeyrules.js';

test('the link resolves the same from /panels and /panels/, at the root or behind a prefix', () => {
  // Both pages set <base href="../">, which resolves to Sode's root.
  assert.equal(absoluteLink('panels/p-1#key=sode_x', 'http://sode.lan:8080/'), 'http://sode.lan:8080/panels/p-1#key=sode_x');
  assert.equal(absoluteLink('panels/p-1#key=sode_x', new URL('../', 'http://sode.lan/panels').href), 'http://sode.lan/panels/p-1#key=sode_x');
  assert.equal(absoluteLink('panels/p-1#key=sode_x', new URL('../', 'http://sode.lan/panels/').href), 'http://sode.lan/panels/p-1#key=sode_x');
  assert.equal(absoluteLink('panels/p-1#key=sode_x', new URL('../', 'http://sode.lan/studio/panels/').href), 'http://sode.lan/studio/panels/p-1#key=sode_x');
});

test('the mask hides the secret and keeps the rest', () => {
  const m = maskLink('http://h/panels/p-1#key=sode_secret');
  assert.ok(m.startsWith('http://h/panels/p-1#key='));
  assert.ok(!m.includes('sode_secret'));
  assert.ok(!maskLink('http://h/panels/p-1#setup=sode_setup_abc&kiosk=1').includes('abc'));
  assert.ok(maskLink('http://h/panels/p-1#setup=sode_setup_abc&kiosk=1').endsWith('&kiosk=1'));
});

test('making a link asks first only when an instance would end', () => {
  assert.deepEqual(makeAction('key', null), { label: 'Make link', confirm: false, verb: 'Make' });
  assert.equal(makeAction('key', { name: 'Lobby' }).confirm, true);
  assert.equal(makeAction('key', { name: 'Lobby' }).label, 'Rotate this instance');
  assert.equal(makeAction('setup', null).confirm, false);
  assert.equal(makeAction('setup', { name: 'Lobby' }).confirm, true);
  assert.equal(confirmKind('key', 'make'), 'rotate');
  assert.equal(confirmKind('setup', 'make'), 'setup');
  assert.equal(confirmKind('setup', 'revoke'), 'revoke');
});

test('the two kinds of link say what they are', () => {
  assert.deepEqual(
    MODES.map(([, text]) => text),
    ['Named instance link', 'One-time setup link'],
  );
  assert.ok(!JSON.stringify(MODES).includes('Standing'));
});

test('an instance name is required, unique ignoring case, and counts against the cap', () => {
  const keys = [{ name: 'Lobby' }, { name: 'Studio' }];
  assert.equal(instanceCheck('  ', keys, 20).error, INSTANCE_HOLD);
  assert.equal(instanceCheck('', keys, 20).name, '');
  assert.equal(instanceCheck(' lobby ', keys, 20).held.name, 'Lobby');
  assert.equal(instanceCheck('lobby', keys, 20).error, '');
  assert.equal(instanceCheck('Gym', keys, 20).held, null);
  assert.equal(instanceCheck('\u{1F3A5}'.repeat(40), keys, 20).error, '');
  assert.ok(instanceCheck('x'.repeat(INSTANCE_MAX + 1), keys, 20).error.includes('40'));
  assert.ok(instanceCheck('Gym', keys, 2).error.includes('at most 2'));
  assert.equal(instanceCheck('lobby', keys, 2).error, '', 'rotating a held name is allowed at the cap');
});

test('a row says online or offline in words, and a pending setup link says when it ends', () => {
  const now = Date.parse('2026-11-08T12:00:00Z');
  const k = { id: 'a', name: 'Lobby', created: '2026-11-08T11:00:00Z', lastUsed: '2026-11-08T11:55:00Z', online: true };
  assert.deepEqual(keyRow(k, now).state, { kind: 'ok', text: 'Online' });
  assert.equal(keyRow(k, now).used, 'Last used 5 minutes ago');
  assert.equal(keyRow(k, now).created, 'Made 1 hour ago');
  assert.deepEqual(keyRow({ ...k, online: false, lastUsed: '0001-01-01T00:00:00Z' }, now).state, { kind: 'idle', text: 'Offline' });
  assert.equal(keyRow({ ...k, online: false, lastUsed: '0001-01-01T00:00:00Z' }, now).used, 'Last used never');
  const s = keyRow({ ...k, setup: true, expires: '2026-11-08T12:40:00Z' }, now);
  assert.deepEqual(s.state, { kind: 'idle', text: 'Setup link waiting' });
  assert.equal(s.used, 'Expires in 40 minutes');
});

test('the Share modal opens on the tab that fits how the panel acts', () => {
  assert.equal(firstShareTab({ acts: 'api' }), 'key');
  assert.equal(firstShareTab({ acts: 'both' }), 'key');
  assert.equal(firstShareTab({ acts: 'session' }), 'people');
  assert.equal(firstShareTab({}), 'people');
  assert.ok(SESSION_WARNING.includes('sign-in'));
});

test('networks are split from lines and commas', () => {
  assert.deepEqual(parseNetworks('10.0.0.0/8, 192.168.1.5\n\n 2001:db8::/32 '), ['10.0.0.0/8', '192.168.1.5', '2001:db8::/32']);
  assert.deepEqual(parseNetworks('  '), []);
  assert.equal(networksChanged(['10.0.0.0/8'], '10.0.0.0/8\n'), false);
  assert.equal(networksChanged(['10.0.0.0/8'], ''), true);
  assert.equal(networksChanged([], ''), false);
});

test('a setup token comes out of the hash and is dropped from it', () => {
  assert.equal(setupFromHash('#setup=sode_setup_abc&kiosk=1'), 'sode_setup_abc');
  assert.equal(setupFromHash('#kiosk=1&setup=t'), 'sode_setup_t');
  assert.equal(setupFromHash('#key=sode_k'), '');
  assert.equal(withoutSetup('#setup=t&kiosk=1'), '#kiosk=1');
  assert.equal(withoutSetup('#setup=t'), '');
  assert.equal(storedKeyName('p-1'), 'sode-panel-key-p-1');
});

test('a link key leaves the hash and every other part stays', () => {
  assert.equal(withoutKey('#key=abc'), '');
  assert.equal(withoutKey('#key=abc&kiosk=1'), '#kiosk=1');
  assert.equal(withoutKey('#kiosk=1&key=abc'), '#kiosk=1');
  assert.equal(withoutKey('#kiosk=1&key=abc&x=2'), '#kiosk=1&x=2');
  assert.equal(withoutKey('#kiosk=1'), '#kiosk=1');
  assert.equal(withoutKey('#monkey=1&key=a'), '#monkey=1');
  assert.equal(withoutKey(''), '');
  assert.equal(withoutSetup('#key=a&setup=t'), '#key=a');
});

test('a polling failure stops the page for a dead key or a refused network, not for a dropped connection', () => {
  assert.equal(pollFail(401, 'invalid_key', '', true).stop, true);
  assert.match(pollFail(401, 'invalid_key', '', true).text, /rotated or revoked/);
  assert.equal(pollFail(401, 'invalid_key', '', false).stop, false);
  const net = pollFail(403, 'network_not_allowed', 'not from here');
  assert.deepEqual(net, { stop: true, text: 'not from here' });
  assert.equal(pollFail(0, '').stop, false);
  assert.equal(pollFail(503, 'api_off').stop, false);
});

test('a session key that ended stops the page and says how to go on; a wait from Sode does not', () => {
  for (const code of ['key_revoked', 'key_expired']) {
    const f = pollFail(401, code, 'ended', false);
    assert.equal(f.stop, true);
    assert.match(f.text, /Reload/);
    assert.match(pollFail(401, code, 'ended', true).text, /ask for a new link/);
  }
  const wait = pollFail(429, 'rate_limited', 'slow down', false);
  assert.equal(wait.stop, false);
  assert.match(wait.text, /wait/);
});

test('a refused setup says why', () => {
  assert.match(setupFailText(401), /already used/);
  assert.equal(setupFailText(403, 'outside'), 'outside');
});
