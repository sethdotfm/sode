// The setup wizard's step rules and its Finish message.
import test from 'node:test';
import assert from 'node:assert/strict';
import { STEPS, startAnswers, stepProblem, stopAt, chipGoes, numberingChoices, finishMessage, routerLine } from '../setupwizardrules.js';

const filled = () => ({
  ...startAnswers({ router: '10.0.0.5:8910', levels: 17, numberOffset: 1 }, { preset: 'standard' }),
  name: ' Ada ',
  pin: '2468',
  pin2: '2468',
});

test('the hello config fills the fields, and Standard access is the default', () => {
  const d = startAnswers({ brandText: 'B', router: '10.0.0.5:8910', levels: 4, levelNumbering: 'va', numberOffset: 0 }, {});
  assert.equal(d.router, '10.0.0.5:8910');
  assert.equal(d.levels, '4');
  assert.equal(d.levelNumbering, 'va');
  assert.equal(d.numberOffset, '0');
  assert.equal(d.preset, 'standard');
  assert.equal(d.fireMode, 'sequential');
  assert.equal(startAnswers({}, { preset: 'secured' }).preset, 'secured');
});

test('Welcome needs a name and the same PIN twice', () => {
  const d = filled();
  assert.equal(stepProblem(d, 0, 8), '');
  assert.match(stepProblem({ ...d, name: '  ' }, 0, 8), /name/);
  assert.match(stepProblem({ ...d, name: 'x'.repeat(25) }, 0, 8), /24/);
  assert.match(stepProblem({ ...d, pin: '12', pin2: '12' }, 0, 8), /4 to 8 digits/);
  assert.match(stepProblem({ ...d, pin: '12ab', pin2: '12ab' }, 0, 8), /digits/);
  assert.match(stepProblem({ ...d, pin: '123456789', pin2: '123456789' }, 0, 8), /4 to 8/);
  assert.match(stepProblem({ ...d, pin2: '2469' }, 0, 8), /not the same/);
});

test('Router needs an address and levels; a router salvo needs a group', () => {
  const d = filled();
  assert.equal(stepProblem(d, 1), '');
  assert.match(stepProblem({ ...d, router: '' }, 1), /address/);
  assert.match(stepProblem({ ...d, levels: '0' }, 1), /1 to 256/);
  assert.match(stepProblem({ ...d, levels: '2.5' }, 1), /1 to 256/);
  assert.equal(stepProblem({ ...d, fireMode: 'native' }, 1), '');
  assert.match(stepProblem({ ...d, fireMode: 'native', group: '128' }, 1), /0 to 127/);
});

test('a jump forward stops at the first step that cannot go on', () => {
  const d = filled();
  assert.equal(stopAt(d, 2), 2);
  assert.equal(stopAt({ ...d, router: '' }, 2), 1);
  assert.equal(stopAt({ ...d, pin2: '' }, 2), 0);
});

test('chips move between visited steps until Finish', () => {
  assert.equal(chipGoes(0, 2, 2, false), true);
  assert.equal(chipGoes(2, 0, 2, false), true);
  assert.equal(chipGoes(3, 1, 2, false), false);
  assert.equal(chipGoes(1, 1, 2, false), false);
  assert.equal(chipGoes(0, 3, 3, true), false);
  assert.equal(STEPS.length, 4);
});

test('level numbering reads from the level count', () => {
  assert.deepEqual(numberingChoices('17'), [
    ['1', '1 to 17'],
    ['0', '0 to 16'],
    ['va', 'V + 1 to 16'],
  ]);
});

test('Finish sends the answers once, without PIN again', () => {
  const m = finishMessage({ ...filled(), fireMode: 'native', group: '120' }, 4);
  assert.equal(m.t, 'setupFinish');
  assert.equal(m.id, 4);
  assert.equal(m.setup.name, 'Ada');
  assert.equal(m.setup.levels, 17);
  assert.equal(m.setup.numberOffset, 1);
  assert.equal(m.setup.group, 120);
  assert.equal(m.setup.preset, 'standard');
  assert.equal('pin2' in m.setup, false);
  assert.equal(finishMessage(filled(), 5).setup.group, 127);
});

test('the router line has two states', () => {
  assert.equal(routerLine('', 'up', 3), null);
  assert.deepEqual(routerLine('10.0.0.5:8910', 'up', 3), { ok: true, text: 'Connected. Names read.' });
  assert.equal(routerLine('10.0.0.5:8910', 'up', 0).ok, false);
  assert.equal(routerLine('10.0.0.5:8910', 'connecting', 0).text, 'No answer yet. You can go on, and Sode keeps trying.');
});
