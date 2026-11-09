import { test } from 'node:test';
import assert from 'node:assert/strict';
import { apiSettingsReq, apiProblem, apiEdited, apiRepaints, SCOPES } from '../apirules.js';

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
    ['names', 'tally', 'protected', 'status', 'presets', 'history', 'layouts', 'log', 'take', 'panels', 'panelsmanage'],
  );
});

test('apiEdited is false while the form equals what was saved, so a hello repaint may refill it', () => {
  assert.equal(apiEdited(cur, { ...cur }), false);
  assert.equal(apiEdited(cur, { ...cur, takeCap: 5 }), true);
  assert.equal(apiEdited(cur, { ...cur, logReads: true }), true);
});

test('apiRepaints lets a hello through once the form matches it, so a second save diffs against what the server holds', () => {
  const incoming = { ...cur, enabled: true };
  assert.equal(apiRepaints(cur, { ...cur, enabled: true }, incoming), true);
});

test('apiRepaints keeps unsaved edits that match neither the saved nor the incoming settings', () => {
  const incoming = { ...cur, enabled: true };
  assert.equal(apiRepaints(cur, { ...cur, takeCap: 7 }, incoming), false);
  assert.equal(apiRepaints(cur, { ...cur }, incoming), true);
});
