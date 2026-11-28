import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import {
  STEPS,
  startAnswers,
  stepProblem,
  problemIsMistake,
  pinMistake,
  stopAt,
  chipGoes,
  numberingChoices,
  finishMessage,
  routerLine,
  guideLevels,
  envHint,
  restorePinProblem,
  keptAnswers,
  refillAnswers,
  connectionTitle,
  doneParts,
  FIRE_MODE_TIP,
} from '../setupwizardrules.js';

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
  assert.equal(d.routerTitle, 'Router A', 'the default title of Add router');
  assert.equal(startAnswers({ routerTitle: 'MCR', routerPrefix: 'A' }, {}).routerTitle, 'MCR');
  assert.equal(startAnswers({}, { preset: 'secured' }).preset, 'secured');
});

test('SODE_* fills Multi destination and its group, and each filled field names its variable', () => {
  const env = { router: 'SODE_ROUTER', fireMode: 'SODE_FIRE_MODE', group: 'SODE_SALVO_GROUP' };
  const d = startAnswers({ router: '10.0.0.5:8910', fireMode: 'native', salvoGroup: 120 }, { env });
  assert.equal(d.fireMode, 'native');
  assert.equal(d.group, '120');
  // Without the variable the wizard keeps its own choice, whatever the config says.
  const plain = startAnswers({ fireMode: 'native', salvoGroup: 3 }, {});
  assert.equal(plain.fireMode, 'sequential');
  assert.equal(plain.group, '127');
  assert.equal(envHint({ env }, 'router'), 'Filled from SODE_ROUTER.');
  assert.equal(envHint({ env }, 'levels'), '');
  assert.equal(envHint({}, 'router'), '');
  assert.equal(envHint(null, 'router'), '');
});

test('Welcome needs a name and the same PIN twice', () => {
  const d = filled();
  assert.equal(stepProblem(d, 0, 8), '');
  assert.match(stepProblem({ ...d, name: '  ' }, 0, 8), /name/);
  assert.match(stepProblem({ ...d, name: 'x'.repeat(25) }, 0, 8), /24/);
  assert.match(stepProblem({ ...d, pin: '12', pin2: '12' }, 0, 8), /4 to 8 digits/);
  assert.match(stepProblem({ ...d, pin: '12ab', pin2: '12ab' }, 0, 8), /digits/);
  assert.match(stepProblem({ ...d, pin: '123456789', pin2: '123456789' }, 0, 8), /4 to 8/);
  assert.match(stepProblem({ ...d, pin2: '2469' }, 0, 8), /differ/);
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

test('a problem is a mistake only once its field has text; before that it is a hint', () => {
  const d = filled();
  const blank = { ...d, name: '', pin: '', pin2: '' };
  assert.equal(problemIsMistake(blank, 0, 8), false, 'no name yet');
  assert.equal(problemIsMistake({ ...blank, name: 'Ada' }, 0, 8), false, 'no PIN yet');
  assert.equal(problemIsMistake({ ...blank, name: 'Ada', pin: '24' }, 0, 8), false, 'a PIN still being typed');
  assert.equal(problemIsMistake({ ...blank, name: 'Ada', pin: '24a' }, 0, 8), true, 'a letter in the PIN');
  assert.equal(problemIsMistake({ ...blank, name: 'Ada', pin: '123456789' }, 0, 8), true, 'too many digits');
  assert.equal(problemIsMistake({ ...d, pin2: '2469' }, 0, 8), true, 'PINs that differ');
  assert.equal(problemIsMistake({ ...d, pin2: '2' }, 0, 8), true, 'Repeat your PIN has text');
  assert.equal(problemIsMistake({ ...d, name: 'x'.repeat(25) }, 0, 8), true, 'a name too long');
  assert.equal(problemIsMistake({ ...d, router: '' }, 1), false, 'no address yet');
  assert.equal(problemIsMistake({ ...d, levels: '' }, 1), false, 'no levels yet');
  assert.equal(problemIsMistake({ ...d, levels: '0' }, 1), true);
  assert.equal(problemIsMistake({ ...d, routerTitle: 'x'.repeat(33) }, 1), true);
  assert.equal(problemIsMistake({ ...d, fireMode: 'native', group: '' }, 1), false);
  assert.equal(problemIsMistake({ ...d, fireMode: 'native', group: '128' }, 1), true);
  assert.equal(problemIsMistake(d, 0, 8), false, 'no problem');
  assert.equal(pinMistake('', '', 8), false);
  assert.equal(pinMistake('2468', '2469', 8), true);
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
    ['1', '1–17'],
    ['0', '0–16'],
    ['va', 'V, 1–16'],
  ]);
  assert.deepEqual(numberingChoices(''), [
    ['1', '1'],
    ['0', '0'],
    ['va', 'V'],
  ]);
});

test('Finish sends the answers once, without Repeat your PIN', () => {
  const m = finishMessage({ ...filled(), fireMode: 'native', group: '120' }, 4);
  assert.equal(m.t, 'setupFinish');
  assert.equal(m.id, 4);
  assert.equal(m.setup.name, 'Ada');
  assert.equal(m.setup.title, 'Router A');
  assert.equal(m.setup.levels, 17);
  assert.equal(m.setup.numberOffset, 1);
  assert.equal(m.setup.group, 120);
  assert.equal(m.setup.preset, 'standard');
  assert.equal('pin2' in m.setup, false);
  assert.equal(finishMessage(filled(), 5).setup.group, 127);
});

test('the router line has two states and counts what it found', () => {
  assert.equal(routerLine('', 'up', 3, 2), null);
  assert.deepEqual(routerLine('10.0.0.5:8910', 'up', 1234, 123), { ok: true, text: 'Connected: 1234 sources and 123 destinations discovered.' });
  assert.equal(routerLine('10.0.0.5:8910', 'up', 1, 1).text, 'Connected: 1 source and 1 destination discovered.');
  assert.equal(routerLine('10.0.0.5:8910', 'up', 0, 0).ok, false);
  assert.equal(routerLine('10.0.0.5:8910', 'connecting', 0, 0).text, 'No answer yet. Sode keeps trying.');
});

test('the level guide labels the typed count in the picked numbering and keeps the router colors', () => {
  assert.deepEqual(guideLevels('', '1'), []);
  assert.deepEqual(guideLevels('300', '1'), []);
  assert.deepEqual(
    guideLevels('3', 'va').map((l) => l.label),
    ['V', '1', '2'],
  );
  assert.deepEqual(
    guideLevels('2', '0').map((l) => l.label),
    ['0', '1'],
  );
  const l = guideLevels('19', '1', [{ ui: 1, color: '#7ae85a' }]);
  assert.equal(l[0].color, '#7ae85a');
  assert.equal(l[1].swatch, '01');
  assert.equal(l[17].swatch, '01'); // 16 audio swatches, then round again
});

test('a restore without PINs takes a new PIN only when both entries agree', () => {
  assert.equal(restorePinProblem('2468', '2468', 8), '');
  assert.match(restorePinProblem('24', '24', 8), /4 to 8 digits/);
  assert.match(restorePinProblem('24a8', '24a8', 8), /4 to 8 digits/);
  assert.match(restorePinProblem('2468', '2469', 8), /differ/);
});

test('the Router step refuses a Router title longer than 32 characters', () => {
  const at = STEPS.findIndex(([k]) => k === 'router');
  assert.equal(stepProblem({ ...filled(), routerTitle: 'x'.repeat(33) }, at), 'A Router title has at most 32 characters.');
  assert.equal(stepProblem({ ...filled(), routerTitle: '' }, at), '');
});

test('a tab keeps its answers but never the PINs, and gets them back after Take control', () => {
  const d = { ...filled(), pin2: '2468', routerTitle: 'Studio Ultrix', brandText: 'Studio B' };
  const kept = keptAnswers(d);
  assert.equal('pin' in kept, false);
  assert.equal('pin2' in kept, false);
  assert.equal(kept.name, ' Ada ');
  const back = refillAnswers(startAnswers({ router: '10.0.0.9:8910' }), JSON.parse(JSON.stringify(kept)));
  assert.equal(back.name, ' Ada ');
  assert.equal(back.routerTitle, 'Studio Ultrix');
  assert.equal(back.router, '10.0.0.5:8910');
  assert.equal(back.pin, '');
  assert.equal(back.pin2, '');
});

test('kept answers that are missing or not strings change nothing', () => {
  const start = startAnswers({ router: '10.0.0.9:8910', levels: 4 });
  assert.deepEqual(refillAnswers(start, null), start);
  assert.deepEqual(refillAnswers(start, 'x'), start);
  assert.deepEqual(refillAnswers(start, { levels: 9, pin: '1111', again: true, router: '10.0.0.1:1' }), { ...start, router: '10.0.0.1:1' });
});

test('the Connection heading names the first router on first run only', () => {
  assert.equal(connectionTitle(false), 'Connection: Add your first router');
  assert.equal(connectionTitle(undefined), 'Connection: Add your first router');
  assert.equal(connectionTitle(true), 'Connection');
});

const plain = (parts) => parts.map((p) => (typeof p === 'string' ? p : p.br ? '\n' : (p.strong ?? p.mono))).join('');

test('Done says Congratulations, then who, which router and which preset', () => {
  const parts = doneParts('Seth', '127.0.0.1:8910', 'Open access', true);
  assert.equal(plain(parts), 'Congratulations!\nYou are signed in as Seth and connected to the router at 127.0.0.1:8910 with Open access permissions.');
  assert.deepEqual(
    parts.filter((p) => typeof p !== 'string'),
    [{ br: true }, { strong: 'Seth' }, { mono: '127.0.0.1:8910' }],
  );
});

test('Done before the router answers says Sode keeps trying', () => {
  assert.equal(
    plain(doneParts('Seth', '10.0.0.5:8910', 'Secured access', false)),
    'Congratulations!\nYou are signed in as Seth with Secured access permissions. The router at 10.0.0.5:8910 has not answered yet, and Sode keeps trying.',
  );
});

test('Settings > Routers has the same Multi destination help tip as the wizard', () => {
  const html = readFileSync(new URL('../index.html', import.meta.url), 'utf8');
  const m = html.match(/id="cfg-rt-fireMode-help"[^>]*data-pb-tip="([^"]*)"/);
  assert.ok(m, 'no help button by Multi destination');
  assert.equal(m[1], FIRE_MODE_TIP);
  assert.ok(FIRE_MODE_TIP.includes('One after another'));
  assert.ok(FIRE_MODE_TIP.includes('Router salvo'));
});
