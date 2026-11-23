import test from 'node:test';
import assert from 'node:assert/strict';
import { changeText, leftText, endText, approverKey } from '../approvalcardrules.js';

test('changeText says old to new, secrets as changed, lists as added and removed', () => {
  assert.equal(changeText({ label: 'Confirm lockout', old: 500, new: 900 }), 'Confirm lockout: 500 to 900');
  assert.equal(changeText({ label: 'Router', old: 'a:1', new: 'b:2' }), 'Router: a:1 to b:2');
  assert.equal(changeText({ label: 'PIN', changed: true }), 'PIN: changed');
  assert.equal(changeText({ label: 'Proxies', added: ['10.0.0.1'], removed: ['10.0.0.2'] }), 'Proxies: added 10.0.0.1; removed 10.0.0.2');
});

test('leftText counts down in minutes and seconds', () => {
  assert.equal(leftText(120000), '2:00');
  assert.equal(leftText(61001), '1:02');
  assert.equal(leftText(-5), '0:00');
});

test('endText names who answered', () => {
  assert.equal(endText('approved', 'Ada'), 'Approved by Ada');
  assert.equal(endText('denied', 'Ada'), 'Denied by Ada');
  assert.equal(endText('timeout', ''), 'No one answered');
  assert.equal(endText('cancelled', ''), 'Cancelled');
  assert.equal(endText('', ''), '');
});

test('approverKey maps the who text to the approvers field', () => {
  assert.equal(approverKey('an Administrator'), 'administrator');
  assert.equal(approverKey('a Trusted user or an Administrator'), 'trusted');
});
