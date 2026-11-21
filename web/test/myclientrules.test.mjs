import { test } from 'node:test';
import assert from 'node:assert/strict';
import { OWN_SCOPES, clientNameProblem, saveReq, keyReq } from '../myclientrules.js';

test('own clients offer the scopes of a user key, less the Communication Log and panel management', () => {
  const ids = OWN_SCOPES.map((s) => s.id);
  assert.ok(ids.includes('take') && ids.includes('names'));
  assert.ok(!ids.includes('log') && !ids.includes('panelsmanage'));
});

test('a client name is needed, short and not one of your own clients', () => {
  assert.match(clientNameProblem('  ', []), /Name/);
  assert.match(clientNameProblem('x'.repeat(25), []), /24/);
  assert.match(clientNameProblem('desk', ['Desk']), /already/);
  assert.equal(clientNameProblem('Desk 2', ['Desk']), '');
});

test('a new client leaves its Rights out, so the server copies yours', () => {
  assert.deepEqual(saveReq({ name: ' Desk ', scopes: ['take'], id: 3 }), { t: 'myClientSave', id: 3, client: { name: 'Desk', scopes: ['take'] } });
});

test('a key request names the client and carries your PIN', () => {
  assert.deepEqual(keyReq({ client: 'Desk', name: ' Wall ', pin: '1234', expires: 0, id: 4 }), {
    t: 'myClientKey',
    id: 4,
    name: 'Desk',
    keyName: 'Wall',
    pin: '1234',
    expires: 0,
  });
});
