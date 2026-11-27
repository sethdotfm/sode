import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import {
  DETAIL_ATTRS,
  NEW_ROUTER,
  textsOf,
  readValues,
  fieldErrors,
  numberingRange,
  offsetNote,
  defaultNote,
  renumberDefault,
  levelsText,
  valueText,
  addText,
  countLine,
  findMatch,
  hasProblem,
  rowMarks,
  protectsText,
  tiesText,
} from '../routerdetailrules.js';
import { settingKeyNow, rowByKey } from '../palettecatalogue.js';
import { ROUTER_ATTRS } from '../savereviewrules.js';

const levels = (n, offset) => Array.from({ length: n }, (_, i) => ({ ui: i + 1, wire: i + 1 + offset }));
const hello = {
  uid: 'ub',
  prefix: 'B',
  title: 'Stage 2',
  address: '10.0.0.9:8910',
  matrix: 2,
  levels: levels(17, -1),
  levelWireOffset: -1,
  levelNumbering: 'va',
  defaultLevels: '1-3',
  numberOffset: 1,
  nameRereadSeconds: 60,
  nameLength: 12,
  nameRequests: 'std',
  tallyResyncSeconds: 300,
  fireMode: 'sequential',
  salvoGroup: 127,
  protectDevice: 201,
};

test('the detail view has every router key, and the review section of each is Routers', () => {
  assert.deepEqual([...DETAIL_ATTRS].sort(), Object.keys(ROUTER_ATTRS).sort());
  assert.deepEqual(Object.keys(NEW_ROUTER).sort(), [...DETAIL_ATTRS].sort());
});

test('a router of the hello reads back as the values it has', () => {
  const t = textsOf(hello);
  assert.equal(t.levels, '17');
  assert.equal(t.defaultLevels, 'V, 1-2', 'in the numbering of the router; V never starts a range');
  const v = readValues(t);
  assert.equal(v.levels, 17);
  assert.equal(v.defaultLevels, '1-3');
  assert.equal(v.prefix, 'B');
  assert.equal(v.protectDevice, 201);
  assert.deepEqual(fieldErrors(v, ['A']), {});
});

test('a blank Default levels stays blank: every level', () => {
  const v = readValues({ ...textsOf({ ...hello, defaultLevels: '' }) });
  assert.equal(v.defaultLevels, '');
  assert.equal(defaultNote(v), 'Blank is every level, also after the level count changes.');
  assert.equal(valueText('defaultLevels', ''), 'Every level');
});

test('fieldErrors: one message for each setting, Matrix against Name requests on the page', () => {
  const v = readValues(textsOf(hello));
  assert.match(fieldErrors({ ...v, prefix: 'A' }, ['A']).prefix, /Another router has the prefix A/);
  assert.match(fieldErrors({ ...v, prefix: 'B1' }).prefix, /one or two letters/);
  assert.match(fieldErrors({ ...v, matrix: 20 }).matrix, /Standard name requests allow matrix 0 to 15/);
  assert.equal(fieldErrors({ ...v, matrix: 20, nameRequests: 'ext' }).matrix, undefined);
  assert.match(fieldErrors({ ...v, matrix: 20, nameRequests: 'ext', fireMode: 'native' }).matrix, /Router salvo/);
  assert.match(fieldErrors({ ...v, address: 'nohost' }).address, /host:port/);
  assert.equal(fieldErrors({ ...v, address: '' }).address, undefined);
  assert.equal(fieldErrors({ ...v, address: '[::1]:8910' }).address, undefined);
  assert.match(fieldErrors({ ...v, levels: NaN }).levels, /1 to 256/);
  assert.match(fieldErrors({ ...v, levelWireOffset: -5 }).levelWireOffset, /0 to 255/);
  assert.match(fieldErrors({ ...v, defaultLevels: null }).defaultLevels, /V, 1–16/);
  assert.match(fieldErrors({ ...v, nameRereadSeconds: 5 }).nameRereadSeconds, /10 to 86400/);
  assert.match(fieldErrors({ ...v, tallyResyncSeconds: 20 }).tallyResyncSeconds, /30 to 86400/);
  assert.match(fieldErrors({ ...v, protectDevice: 5000 }).protectDevice, /4095/);
  assert.match(fieldErrors({ ...v, salvoGroup: 128 }).salvoGroup, /0 to 127/);
});

test('Level numbering shows the real range, and the offset hint and the default levels follow it', () => {
  assert.deepEqual(
    ['1', '0', 'va'].map((m) => numberingRange(17, m)),
    ['1–17', '0–16', 'V, 1–16'],
  );
  assert.equal(numberingRange(1, 'va'), 'V');
  assert.equal(offsetNote('-1', '1'), 'Level 1 = level 0 on the router.');
  assert.equal(offsetNote('0', 'va'), 'Level V = level 1 on the router.');
  assert.equal(offsetNote('', '1'), '');
  assert.equal(renumberDefault('1-3', '1', 'va'), 'V, 1-2');
  assert.equal(renumberDefault('x', '1', 'va'), 'x');
});

test('review words', () => {
  assert.equal(valueText('levelNumbering', 'va', { levels: 17 }), 'V, 1–16');
  assert.equal(valueText('nameRequests', 'ext'), 'Extended');
  assert.equal(valueText('fireMode', 'native'), 'Router salvo');
  assert.equal(valueText('protectDevice', 0), '0');
  const v = readValues({ ...NEW_ROUTER, prefix: 'C', title: 'Router C', address: '10.0.0.3:8910' });
  assert.equal(addText(v), 'Router C at 10.0.0.3:8910, matrix 0, 17 levels (1–17), Protect is off');
});

test('the count line, find and the problem filter of the list', () => {
  assert.equal(countLine(['connected']), '');
  assert.equal(countLine(['connected', 'connected', 'down', 'never']), '4 routers. 2 connected, 2 down.');
  assert.equal(countLine(['connected', 'connecting']), '2 routers. 1 connected, 1 connecting.');
  assert.equal(countLine([null, null]), '2 routers.');
  assert.ok(findMatch({ prefix: 'B', title: 'Stage 2', address: '10.0.0.9:8910' }, 'stage'));
  assert.ok(findMatch({ prefix: 'B', title: 'Stage 2', address: '10.0.0.9:8910' }, '0.0.9'));
  assert.ok(!findMatch({ prefix: 'B', title: 'Stage 2', address: '10.0.0.9:8910' }, 'studio'));
  assert.ok(hasProblem('down', 0) && hasProblem('connected', 1) && !hasProblem('connected', 0) && !hasProblem(null, 0));
});

test('row marks: Default on the first row only with two or more, the others are caution', () => {
  assert.deepEqual(rowMarks({ first: true, many: false }), []);
  assert.deepEqual(rowMarks({ first: true, many: true, edited: true }), [
    { text: 'Default', cls: 'pb-badge' },
    { text: 'Edited', cls: 'pb-badge pb-badge--warn' },
  ]);
  assert.deepEqual(rowMarks({ many: true, isNew: true }), [{ text: 'New', cls: 'pb-badge pb-badge--warn' }]);
  assert.deepEqual(rowMarks({ many: true, removed: true, edited: true }), [{ text: 'Remove on Save', cls: 'pb-badge pb-badge--warn' }]);
});

test('facts of the Protects and Tie lines groups', () => {
  assert.equal(protectsText(201, 1, 3), 'Sode has 1 Protect on this router. Other devices have 3.');
  assert.equal(tiesText(2, 1), '3 tie lines: 2 from this router, 1 to this router.');
});

test('a pin or Bookmark of a setting that moved finds its new place under its old key', () => {
  assert.equal(settingKeyNow('setting:router-matrix/cfg-router'), 'setting:routers/cfg-rt-address');
  assert.equal(settingKeyNow('setting:takes/cfg-firemode'), 'setting:routers/cfg-rt-fireMode');
  assert.equal(settingKeyNow('setting:protects/cfg-protectdevice'), 'setting:routers/cfg-rt-protectDevice');
  assert.equal(settingKeyNow('setting:router-levels/cfg-rl-00ab-levels'), 'setting:routers/cfg-rt-levels');
  assert.equal(settingKeyNow('setting:routers/cfg-rt-00ab-matrix'), 'setting:routers/cfg-rt-matrix');
  assert.equal(settingKeyNow('setting:routers/cfg-rt-matrix'), 'setting:routers/cfg-rt-matrix');
  assert.equal(settingKeyNow('page:log'), 'page:log');
  const rows = [{ key: 'setting:routers/cfg-rt-address', label: 'Address' }];
  assert.deepEqual(rowByKey(rows, 'setting:router-matrix/cfg-router'), { key: 'setting:router-matrix/cfg-router', label: 'Address' });
  assert.equal(rowByKey(rows, 'setting:router-matrix/cfg-gone'), null);
});

// The same cases as TestLevelsText in internal/config/router_test.go: the review shows what the Settings saves record
// writes.
const shared = JSON.parse(readFileSync(new URL('./levels_text_cases.json', import.meta.url), 'utf8'));

test('levelsText: saved levels in the numbering of the router, as the server writes them', () => {
  assert.ok(shared.cases.length);
  for (const c of shared.cases) assert.equal(levelsText(c.in, c.levels, c.numbering), c.want, `${c.numbering} ${c.in}`);
});

test('valueText: the review shows Default levels in the numbering of the router (LP-14)', () => {
  assert.equal(valueText('defaultLevels', '1-3', { levels: 17, levelNumbering: 'va' }), 'V, 1-2');
  assert.equal(valueText('defaultLevels', '1-17', { levels: 17, levelNumbering: '0' }), '0-16');
  assert.equal(valueText('defaultLevels', '2,4', { levels: 17, levelNumbering: '1' }), '2, 4');
  assert.equal(valueText('defaultLevels', '', { levels: 17, levelNumbering: 'va' }), 'Every level');
});
