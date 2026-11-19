import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { parseFilter } from '../filter.js';

// Shared with filterterms_test.go, whose Go reader judges filter sets for the Broken check.
const cases = JSON.parse(readFileSync(new URL('./filter_terms.json', import.meta.url), 'utf8'));

const term = (r) => {
  const t = { not: r.not, kind: r.kind };
  if (r.kind === 'num') t.a = r.a;
  if (r.kind === 'exact' || r.kind === 'name') t.text = r.text;
  return t;
};

test('filter terms read as the server reads them', () => {
  for (const c of cases) {
    const p = parseFilter(c.text);
    assert.deepEqual(p.rules ? p.rules.map(term) : null, c.terms, c.text);
  }
});
