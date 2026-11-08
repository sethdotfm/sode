import { test } from 'node:test';
import assert from 'node:assert/strict';
import { apiSettingsReq, apiProblem, SCOPES } from '../apirules.js';

const cur = { enabled: false, rateLimit: 600, takeCap: 4, guestEnabled: false, logReads: false };

test('apiSettingsReq sends only what changed, so a stale page cannot overwrite the rest', () => {
  assert.deepEqual(apiSettingsReq(cur, cur), { t: 'apiSettings', apiSettings: {} });
  assert.deepEqual(apiSettingsReq(cur, { ...cur, enabled: true, rateLimit: 100 }), { t: 'apiSettings', apiSettings: { enabled: true, rateLimit: 100 } });
});

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
    ['names', 'tally', 'protected', 'status', 'presets', 'history', 'layouts', 'log', 'take'],
  );
});
