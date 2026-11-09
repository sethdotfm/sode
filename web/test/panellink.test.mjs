import test from 'node:test';
import assert from 'node:assert/strict';
import { linkFor, pathFor, shortSecret, fullKey, fullSetup, instanceFrom } from '../panellink.js';
import { keyFromHash } from '../panelrules.js';
import { setupFromHash } from '../panelkeyrules.js';

const id = 'p-0123abcd';
const secret = 'sode_' + 'K'.repeat(22);

test('a panel without a short name links by id, and one with a short name links by it', () => {
  assert.equal(pathFor({ id }), `panels/${id}`);
  assert.equal(pathFor({ id, slug: 'studio-a' }), 'panels/studio-a');
  assert.equal(linkFor({ id }), `panels/${id}`);
  assert.equal(linkFor({ id }, { kiosk: true }), `panels/${id}#kiosk=1`);
});

test('the key goes in the fragment without its prefix and the page puts it back', () => {
  const link = linkFor({ id, slug: 'a' }, { key: secret });
  assert.equal(link, `panels/a#key=${'K'.repeat(22)}`);
  assert.equal(keyFromHash(link.slice(link.indexOf('#'))), secret);
  // An old link with the whole secret still opens.
  assert.equal(keyFromHash(`#key=${secret}`), secret);
  assert.equal(keyFromHash('#kiosk=1&key=' + 'K'.repeat(22)), secret);
  assert.equal(keyFromHash(''), '');
});

test('a setup token is shortened and restored the same way', () => {
  const token = 'sode_setup_' + 'T'.repeat(22);
  const link = linkFor({ id }, { setup: token });
  assert.equal(link, `panels/${id}#setup=${'T'.repeat(22)}`);
  assert.equal(setupFromHash('#setup=' + 'T'.repeat(22)), token);
  assert.equal(setupFromHash(`#setup=${token}`), token);
});

test('a key beats a setup token, and kiosk joins the fragment', () => {
  assert.equal(linkFor({ id }, { key: secret, setup: 'x', kiosk: true }), `panels/${id}#key=${'K'.repeat(22)}&kiosk=1`);
});

test('the session name rides the query, encoded, and never the fragment', () => {
  const link = linkFor({ id, slug: 'lobby' }, { instance: ' Studio A & B ', key: secret });
  assert.equal(link, `panels/lobby?instance=Studio%20A%20%26%20B#key=${'K'.repeat(22)}`);
  assert.equal(linkFor({ id }, { instance: '  ' }), `panels/${id}`);
  assert.equal(instanceFrom('?instance=Studio%20A%20%26%20B'), 'Studio A & B');
  assert.equal(instanceFrom(''), '');
  assert.equal(instanceFrom('?x=1'), '');
});

test('a base makes the link absolute, behind a prefix too', () => {
  assert.equal(linkFor({ id, slug: 'a' }, { base: 'http://sode.lan:8080/' }), 'http://sode.lan:8080/panels/a');
  assert.equal(linkFor({ id, slug: 'a' }, { kiosk: true, base: 'http://sode.lan/studio/' }), 'http://sode.lan/studio/panels/a#kiosk=1');
});

test('the shortened forms measure what the issue records', () => {
  const before = `http://sode.lan:8080/panels/${id}#key=sode_${'K'.repeat(43)}`;
  const after = linkFor({ id, slug: 'lobby' }, { key: 'sode_' + 'K'.repeat(22), base: 'http://sode.lan:8080/' });
  assert.equal(before.length, 91);
  assert.equal(after.length, 60);
  assert.equal(shortSecret(fullKey('abc')), 'abc');
  assert.equal(fullSetup('sode_setup_x'), 'sode_setup_x');
});
