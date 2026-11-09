import test from 'node:test';
import assert from 'node:assert/strict';
import {
  absoluteLink,
  maskLink,
  makeAction,
  confirmKind,
  parseNetworks,
  networksChanged,
  setupFromHash,
  withoutSetup,
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

test('making a link asks first only when a key would end', () => {
  assert.deepEqual(makeAction('key', false), { label: 'Make link', confirm: false, verb: 'Make' });
  assert.equal(makeAction('key', true).confirm, true);
  assert.equal(makeAction('key', true).label, 'Rotate key');
  assert.equal(makeAction('setup', false).confirm, false);
  assert.equal(makeAction('setup', true).confirm, true);
  assert.equal(confirmKind('key', 'make'), 'rotate');
  assert.equal(confirmKind('setup', 'make'), 'setup');
  assert.equal(confirmKind('setup', 'revoke'), 'revoke');
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

test('a polling failure stops the page for a dead key or a refused network, not for a dropped connection', () => {
  assert.equal(pollFail(401, 'invalid_key', '', true).stop, true);
  assert.match(pollFail(401, 'invalid_key', '', true).text, /rotated or revoked/);
  assert.equal(pollFail(401, 'invalid_key', '', false).stop, false);
  const net = pollFail(403, 'network_not_allowed', 'not from here');
  assert.deepEqual(net, { stop: true, text: 'not from here' });
  assert.equal(pollFail(0, '').stop, false);
  assert.equal(pollFail(503, 'api_off').stop, false);
});

test('a refused setup says why', () => {
  assert.match(setupFailText(401), /already used/);
  assert.equal(setupFailText(403, 'outside'), 'outside');
});
