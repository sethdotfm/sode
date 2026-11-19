import { test } from 'node:test';
import assert from 'node:assert/strict';
import { apiProblem, SCOPES } from '../apirules.js';

const cur = { rateLimit: 600, takeCap: 4, guestEnabled: false, logReads: false, guestScopes: ['names', 'tally'] };

test('apiProblem uses the server limits: rate limit 0-100000, take cap 1-64', () => {
  assert.equal(apiProblem(cur), '');
  assert.match(apiProblem({ ...cur, rateLimit: -1 }), /rate limit/i);
  assert.match(apiProblem({ ...cur, rateLimit: 100001 }), /rate limit/i);
  assert.match(apiProblem({ ...cur, takeCap: 0 }), /take cap/i);
  assert.match(apiProblem({ ...cur, takeCap: 65 }), /take cap/i);
  assert.match(apiProblem({ ...cur, takeCap: 1.5 }), /take cap/i);
});

test('the scope list matches the server', () => {
  assert.deepEqual(
    SCOPES.map((s) => s.id),
    ['names', 'tally', 'protected', 'status', 'presets', 'history', 'layouts', 'log', 'take', 'panels', 'panelsmanage'],
  );
});
