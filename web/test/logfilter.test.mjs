import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { logPasses, sortKeys, sortRows } from '../logfilter.js';
import { rowKey } from '../logpager.js';

const data = JSON.parse(readFileSync(new URL('./log_filter.json', import.meta.url), 'utf8'));

for (const c of data.cases) {
  test(c.name, () => {
    const f = { kinds: [], problems: false, src: '', dst: '', from: null, to: null, ...c.filter };
    const rows = data.rows.filter((e) => logPasses(e, f, data.offset));
    const out = sortRows(rows, c.sort || { col: 'time', dir: 'desc' }, sortKeys());
    assert.deepEqual(
      out.map((e) => String(rowKey(e))),
      c.want,
    );
  });
}
