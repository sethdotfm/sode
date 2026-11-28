// The setup wizard (docs/agents/server.md, setup.go): a config file with no users opens it in
// place of the patch page. Nothing is sent but the Router step's address until Finish.
import { $, S, el, on, emit, onMsg, wsSend, icon, LINK_DOWN, routerData, routerSetting } from './core.js';
import { showBrand } from './brand.js';
import { brandCaution, pickerColors, CAUTION_NOTE } from './accent.js';
import { statusText } from './statustext.js';
import { keepSignInToken } from './signin.js';
import { desktopOnly } from './desktoponly.js';
import { layout, onLayout } from './layout.js';
import { restoring, wireRestore } from './setuprestore.js';
import { pinShow } from './pinshow.js';
import { levelPicks } from './levelbox.js';
import {
  STEPS,
  DONE,
  PRESET_ROWS,
  startAnswers,
  pinMax,
  stepProblem,
  stopAt,
  chipGoes,
  numberingChoices,
  finishMessage,
  routerLine,
  guideLevels,
  envHint,
  KEPT_KEY,
  keptAnswers,
  refillAnswers,
  connectionTitle,
  doneParts,
  FIRE_MODE_TIP,
} from './setupwizardrules.js';
import { readTab, writeTab } from './store.js';

const root = document.documentElement;
let d = null; // the answers; null while the wizard is closed
let at = 0;
let reached = 0;
let finished = false;
let finishing = false; // Finish sent, no answer yet: its hello comes first
let seq = 0;
let sentRouter = ''; // the address the server tries now
let routerTimer = 0;
let sheetPresets = []; // the hello's presets, kept: the hello after Finish has none
let filled = {}; // the hello's setup, for the "Filled from" hints.
let mode = ''; // '' closed, 'claim' asked the server, 'run' this browser runs setup, 'wait' the wait page.
let claiming = 0; // the id of the setupClaim with no answer yet.
let taken = false; // another browser took control from this one.
let free = false; // the browser that ran setup closed.
let again = null; // Run setup again (mode 'again'): {finish(d), cancel()} from setupagain.js

const max = () => pinMax(S.cfg || {});
// Run setup again keeps nothing: Cancel drops its answers.
const keep = () => d && !d.again && writeTab(KEPT_KEY, JSON.stringify(keptAnswers(d)));
const dropKept = () => writeTab(KEPT_KEY, null);
function readKept() {
  try {
    return JSON.parse(readTab(KEPT_KEY));
  } catch {
    return null;
  }
}
const hint = (key, ...more) => [...more, envHint(filled, key)].filter(Boolean).join(' ');

// h: el with attributes; a false or null attribute is left out.
function field(id, label, value, attrs = {}, hint = '', tip = '') {
  const input = el('input', { id, class: 'pb-input', autocomplete: 'off', ...attrs });
  input.value = value;
  return el(
    'div',
    { class: 'sode__field' },
    el('label', { for: id, 'data-pb-tip': tip || null }, label),
    attrs.type === 'password' ? pinShow(input) : input,
    hint ? el('span', { class: 'sode__hint' }, hint) : null,
  );
}

// label null: the section heading with id setup-<name>-label names the group.
function choice(name, label, opts, hint = '') {
  const group = el('span', { class: 'pb-btn-group', role: 'radiogroup', 'aria-labelledby': `setup-${name}-label` });
  for (const [v, t] of opts) {
    const id = `setup-${name}-${v}`;
    const input = el('input', { type: 'radio', class: 'pb-btn-check', name, id, value: v });
    input.checked = d[name] === v;
    group.append(input, el('label', { class: 'pb-btn pb-btn--outline', for: id }, t));
  }
  return el(
    'div',
    { class: 'sode__field' },
    label ? el('span', { class: 'sode__field-label', id: `setup-${name}-label` }, label) : null,
    group,
    hint ? el('span', { class: 'sode__hint' }, hint) : null,
  );
}

const section = (title, sub, ...inner) => sectionAs(null, title, sub, ...inner);
// title is the heading text, or [text, a node after the heading] such as a help button.
function sectionAs(id, title, sub, ...inner) {
  const [text, after] = Array.isArray(title) ? title : [title, null];
  const h = el('h3', { class: 'sode-setup__h', id }, text);
  const head = after ? el('div', { class: 'sode-setup__head' }, h, after) : h;
  return el('div', { class: 'sode-setup__section' }, head, sub ? el('p', { class: 'sode-setup__sub' }, sub) : null, ...inner);
}

// A help button: it takes focus, so its tip shows from the keyboard too.
const helpButton = (id, label, tip) =>
  el(
    'button',
    { type: 'button', class: 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm sode__help', id, 'aria-label': label, 'data-pb-tip': tip },
    icon('circle-help'),
  );
const row = (...kids) => el('div', { class: 'sode-setup__row' }, ...kids);
const row3 = (...kids) => el('div', { class: 'sode-setup__row sode-setup__row--3' }, ...kids);
const strong = (text) => el('span', { class: 'pb-text-strong' }, text);

function statusLine() {
  // Run setup again moves no link before the save, so only the live address has a state.
  const line =
    d.again && d.router.trim() !== (S.routers[0]?.address || '')
      ? { ok: false, text: 'Connects on Save' }
      : routerLine(d.router, routerData('').link, routerData('').srcs.length, routerData('').dsts.length);
  if (!line) return [];
  return [statusText(line.ok ? 'ok' : 'warn', el('span', null, line.text))];
}

const presets = () => sheetPresets;
const presetName = () => presets().find((p) => p.key === d.preset)?.name || d.preset;

// Each color has its name under the swatch, so the colors are told apart by name as well.
function swatches() {
  const box = el('span', { class: 'sode-setup__swatches', role: 'radiogroup', 'aria-labelledby': 'setup-brand-label' });
  const item = (cls, chip, label) => el('label', { class: cls }, chip, el('span', { class: 'sode-setup__swatch-name', 'aria-hidden': 'true' }, label));
  for (const c of [{ name: '', label: 'Off' }, ...pickerColors(S.brandColors, d.brandBorder)]) {
    const input = el('input', { type: 'radio', name: 'brandBorder', value: c.name, 'aria-label': c.label });
    input.checked = d.brandBorder === c.name;
    box.append(
      item('sode-setup__swatch', el('span', { class: 'sode-setup__swatch-box', 'data-brand': c.name || null }, input, c.name ? null : icon('ban')), c.label),
    );
  }
  // A custom color is #rrggbb, the form the native picker gives. Run setup again leaves the swatch out: it writes
  // one Settings field per answer, and a custom color there is two fields.
  const own = d.brandBorder.startsWith('#');
  if (!d.again) {
    const pick = el('input', { type: 'color', name: 'brandBorder', 'aria-label': 'Custom color' });
    pick.value = own ? d.brandBorder : '#5fb2e8';
    box.append(item(`sode-setup__swatch sode-setup__swatch--own${own ? ' is-picked' : ''}`, el('span', { class: 'sode-setup__swatch-box' }, pick), 'Custom'));
  }
  const note = el(
    'p',
    { class: 'pb-note pb-note--warn sode__brand-caution', id: 'setup-brand-caution', role: 'status', hidden: !brandCaution(d.brandBorder, S.brandColors) },
    icon('triangle-alert'),
    el('span', null, CAUTION_NOTE),
  );
  return el('div', { class: 'sode__field' }, el('span', { class: 'sode__field-label', id: 'setup-brand-label' }, 'Theme color'), box, note);
}

function sheet() {
  const picked = (p) => (d.preset === p.key ? 'is-picked' : '');
  const head = el('tr', null, el('th', { class: 'sode-setup__corner' }, 'Without sign-in:'));
  for (const p of presets()) {
    const input = el('input', { type: 'radio', class: 'pb-check__input', name: 'preset', value: p.key });
    input.checked = d.preset === p.key;
    head.append(
      el(
        'th',
        { 'data-preset': p.key, class: picked(p) },
        el('label', { class: 'pb-check' }, input, el('span', { class: 'pb-check__label pb-text-strong' }, p.name)),
      ),
    );
  }
  const body = el('tbody');
  for (const [key, text] of PRESET_ROWS) {
    const row = el('tr', null, el('th', { scope: 'row' }, text));
    for (const p of presets()) {
      const mark = p.rows[key] ? el('span', { class: 'sode-setup__yes' }, icon('check'), 'Yes') : el('span', { class: 'sode-setup__no' }, icon('x'), 'No');
      row.append(el('td', { 'data-preset': p.key, class: picked(p) }, mark));
    }
    body.append(row);
  }
  return el('table', { class: 'sode-setup__sheet' }, el('thead', null, head), body);
}

function doneText() {
  const linked = !!routerLine(d.router, routerData('').link, routerData('').srcs.length, routerData('').dsts.length)?.ok;
  return doneParts(d.name.trim(), d.router.trim(), presetName(), linked).map((p) => {
    if (typeof p === 'string') return p;
    if (p.br) return el('br');
    return p.mono != null ? el('span', { class: 'pb-text-mono' }, p.mono) : strong(p.strong);
  });
}

// The Level numbering guide: the main page's level chips for the typed count, display only.
function levelGuide() {
  const levels = guideLevels(d.levels, d.levelNumbering, routerData('').levels);
  if (!levels.length) return null;
  const picks = levelPicks(levels, () => {});
  for (const b of picks) b.setAttribute('aria-pressed', 'true');
  const odd = levels.length % 2;
  const cols = Math.floor(levels.length / 2) + odd;
  // Past 12 columns the chips wrap to more rows, so a first level of 0 or V no longer stands alone.
  const grid = el('div', { class: `sode__levels${odd && cols <= 12 ? ' sode__levels--odd' : ''}`, inert: true }, ...picks);
  grid.style.setProperty('--sode-bar-cols', Math.min(cols, 12));
  const first = levels[0].label;
  const last = levels[levels.length - 1].label;
  return el('div', { class: 'sode__picker sode-setup__guide', role: 'img', 'aria-label': `Levels ${first} to ${last}` }, grid);
}

const nextLink = (text, section, href = '#settings') => el('li', null, el('a', { href, 'data-section': section }, text));

const VIEW = {
  welcome: () => [
    d.again ? 'Run setup again' : 'Set up Sode',
    section('Sode Server Name and Theme Color', '', field('setup-brandText', 'Name', d.brandText, { placeholder: 'Sode', maxlength: '24' }), swatches()),
    // replaceChildren draws a null as text, so Run setup again leaves the section out.
    ...(d.again
      ? []
      : [
          el('hr', { class: 'pb-divider pb-divider--hard sode-setup__hr' }),
          section(
            'Add a first user',
            'This user will be the administrator.',
            row3(
              field('setup-name', 'Name', d.name, { maxlength: '24' }),
              field('setup-pin', 'PIN', d.pin, { type: 'password', inputmode: 'numeric', placeholder: `4 to ${max()} digits` }),
              field('setup-pin2', 'Repeat your PIN', d.pin2, { type: 'password', inputmode: 'numeric' }),
            ),
          ),
        ]),
  ],
  router: () => [
    'Router setup',
    section('Router', '', row(field('setup-routerTitle', 'Router title', d.routerTitle, { maxlength: '32' }), el('div'))),
    section(
      connectionTitle(d.again),
      '',
      field('setup-router', 'Address', d.router, { placeholder: '10.0.0.5:8910' }, hint('router')),
      el('div', { id: 'setup-router-status' }, ...statusLine()),
    ),
    section(
      'Levels and numbers',
      '',
      row(field('setup-levels', 'Levels', d.levels, { type: 'number', min: '1', max: '256' }, hint('levels')), el('div')),
      row(choice('levelNumbering', 'Level numbering', numberingChoices(d.levels), hint('levelNumbering')), levelGuide() || el('div')),
      choice(
        'numberOffset',
        'Source and destination numbers',
        [
          ['1', 'Start at 1'],
          ['0', 'Start at 0'],
        ],
        hint('numberOffset'),
      ),
    ),
    sectionAs(
      'setup-fireMode-label',
      ['Multi destination take behavior', helpButton('setup-fireMode-help', 'About Multi destination take behavior', FIRE_MODE_TIP)],
      'Some routers support staging changes for taking many routes simultaneously.',
      row(
        choice(
          'fireMode',
          null,
          [
            ['sequential', 'One after another'],
            ['native', 'Router salvo'],
          ],
          hint('fireMode'),
        ),
        d.fireMode === 'native'
          ? field('setup-group', 'Router salvo group', d.group, { type: 'number', min: '0', max: '127' }, hint('group'), 'Must be unused on the router')
          : el('div'),
      ),
    ),
  ],
  access: () => ['Access', ...(hint('preset') ? [el('p', { class: 'sode__hint', id: 'setup-preset-hint' }, hint('preset'))] : []), sheet()],
  done: () => [
    'Sode is set up',
    el(
      'div',
      { class: 'sode-setup__done' },
      el(
        'div',
        null,
        el('p', { id: 'setup-done-text' }, ...doneText()),
        el('button', { type: 'button', class: 'pb-btn sode-btn--primary pb-btn--lg', id: 'setup-open' }, 'Open Sode'),
      ),
      el(
        'nav',
        { class: 'sode-setup__next', 'aria-label': 'Next steps' },
        el('h3', null, 'Next steps'),
        el(
          'ul',
          null,
          // Add another router only goes to Routers: Add router there would start a router that is not saved.
          nextLink('Add another router', 'routers'),
          nextLink('Adjust level colors', 'level-colors'),
          nextLink('Add more users', 'users'),
          nextLink('Create your first panel', null, 'panels/editor'),
        ),
      ),
    ),
  ],
};

// Run setup again has no Done: Finish opens the Save review.
const steps = () => (d?.again ? STEPS.slice(0, DONE) : STEPS);

function renderSteps() {
  const list = [];
  const all = steps();
  all.forEach(([, name], i) => {
    const now = i === at;
    const done = !now && (i < at || i <= reached);
    const go = chipGoes(i, at, reached, finished);
    const chip = el(
      'button',
      { type: 'button', class: 'sode-setup__chip', 'data-go': go ? String(i) : null, tabindex: go ? null : '-1', 'aria-current': now ? 'step' : null },
      el('span', { class: 'sode-setup__dot' }, done ? icon('check') : String(i + 1)),
      el('span', { class: 'sode-setup__label' }, name),
    );
    list.push(el('li', { class: now ? 'is-now' : done ? 'is-done' : '' }, chip));
    if (i < all.length - 1) list.push(el('span', { class: `sode-setup__line ${i < Math.max(at, reached) ? 'is-done' : ''}` }));
  });
  $('setup-steps').replaceChildren(...list);
}

const KEYS = {
  'setup-brandText': 'brandText',
  'setup-name': 'name',
  'setup-pin': 'pin',
  'setup-pin2': 'pin2',
  'setup-routerTitle': 'routerTitle',
  'setup-router': 'router',
  'setup-levels': 'levels',
  'setup-group': 'group',
};

function render(focus) {
  const step = STEPS[at][0];
  renderSteps();
  const [title, ...body] = VIEW[step]();
  $('setup-title').textContent = title;
  $('setup-body').replaceChildren(...body);
  $('setup-foot').hidden = step === 'done';
  $('setup-back').hidden = at === 0 && !d.again;
  $('setup-back').textContent = at === 0 ? 'Cancel' : 'Back';
  $('setup-restore').hidden = step !== 'welcome' || !!d.again;
  $('setup-next').hidden = false;
  $('setup-next').textContent = { access: 'Finish' }[step] || 'Next';
  $('setup-err').textContent = '';
  check();
  const f = focus ? $(focus) : $('setup-body').querySelector('input:not([type="radio"])');
  if (f) {
    f.focus();
    if (focus && f.type === 'text') f.setSelectionRange(f.value.length, f.value.length);
  } else if (step === 'done') $('setup-open')?.focus();
}

function check() {
  $('setup-next').disabled = finishing || !!stepProblem(d, at, max());
}

function go(i) {
  at = stopAt(d, i, max());
  render();
}

// The server moves its link to a typed address after a pause, so the status line can say if it answers.
function tryRouter() {
  clearTimeout(routerTimer);
  if (d?.again) return;
  routerTimer = setTimeout(() => {
    const addr = d?.router.trim();
    if (!addr || addr === sentRouter) return;
    sentRouter = addr;
    wsSend({ t: 'setupRouter', id: ++seq, router: addr });
  }, 600);
}

function paintRouter() {
  const box = d && $('setup-router-status');
  if (box) box.replaceChildren(...statusLine());
}

function paintDone() {
  const box = d && finished && $('setup-done-text');
  if (box) box.replaceChildren(...doneText());
}

$('setup-body').addEventListener('input', (e) => {
  const t = e.target;
  const k = KEYS[t.id] || t.name;
  if (!d || !k) return;
  if (t.type === 'radio' && !t.checked) return;
  d[k] = t.value;
  keep();
  if (k === 'brandText' || k === 'brandBorder') showBrand(d.brandText, d.brandBorder);
  // The color input keeps its picker open, so the note changes in place and not by a render.
  if (k === 'brandBorder' && $('setup-brand-caution')) $('setup-brand-caution').hidden = !brandCaution(d.brandBorder, S.brandColors);
  // No render: it would close the open color picker.
  if (t.type === 'color') {
    for (const r of $('setup-body').querySelectorAll('input[name="brandBorder"][type="radio"]')) r.checked = false;
    t.closest('label').classList.add('is-picked');
  }
  if (k === 'router') {
    tryRouter();
    paintRouter();
  }
  if (t.type === 'radio' || k === 'levels') render(t.type === 'radio' ? null : t.id);
  else check();
  if (!finishing) $('setup-err').textContent = stepProblem(d, at, max());
});

$('setup-body').addEventListener('click', (e) => {
  const cell = e.target.closest('[data-preset]');
  if (cell && d && d.preset !== cell.dataset.preset) {
    d.preset = cell.dataset.preset;
    keep();
    render();
    return;
  }
  const next = e.target.closest('.sode-setup__next a[data-section]');
  if (next) {
    e.preventDefault();
    openSode();
    emit('settingsOpen');
    document.querySelector(`fieldset[data-section="${next.dataset.section}"]`)?.scrollIntoView({ block: 'start' });
    return;
  }
  if (e.target.closest('#setup-open')) openSode();
});

$('setup-body').addEventListener('keydown', (e) => {
  if (e.key !== 'Enter' || e.target.tagName !== 'INPUT' || $('setup-next').disabled) return;
  e.preventDefault();
  $('setup-next').click();
});

$('setup-steps').addEventListener('click', (e) => {
  const b = e.target.closest('[data-go]');
  if (b) go(+b.dataset.go);
});

$('setup-back').addEventListener('click', () => (d?.again && at === 0 ? closeAgain(false) : go(Math.max(0, at - 1))));

$('setup-next').addEventListener('click', () => {
  if (stepProblem(d, at, max())) return;
  if (STEPS[at][0] !== 'access') {
    at++;
    reached = Math.max(reached, at);
    render();
    return;
  }
  if (d.again) return closeAgain(true);
  // Finish sends every answer at once, so a wizard left half done leaves nothing behind.
  if (!wsSend(finishMessage(d, ++seq))) {
    $('setup-err').textContent = LINK_DOWN;
    return;
  }
  finishing = true;
  check();
  $('setup-err').textContent = 'Saving…';
});

onMsg('setupFinishResult', (m) => {
  if (!d || m.id !== seq) return;
  finishing = false;
  if (!m.ok && !S.setup) return openSode(); // another browser finished first
  if (!m.ok) {
    $('setup-err').textContent = m.err || 'Setup was refused.';
    check();
    return;
  }
  keepSignInToken(m);
  dropKept();
  finished = true;
  at = reached = DONE;
  render();
});

wireRestore({ max, back: () => render() });

function openSode() {
  d = null;
  mode = '';
  root.removeAttribute('data-sode-setup');
  $('setup').hidden = true;
  if (layout() === 'desktop') $('src')?.focus();
}

// The server gives setup to one browser (setup.go). A phone never asks for it.
function claim(take = false) {
  if (layout() === 'phone' || claiming) return;
  claiming = ++seq;
  if (!wsSend({ t: 'setupClaim', id: claiming, ...(take ? { take: true } : {}) })) claiming = 0;
}

function startWizard() {
  if (d) return;
  // The wizard sets up the first router: its settings come from the hello's routers list.
  const r0 = S.routers[0] || {};
  const router = {
    routerTitle: r0.title,
    routerPrefix: r0.prefix,
    router: r0.address,
    levels: routerSetting(r0, 'levels'),
    levelNumbering: r0.levelNumbering,
    numberOffset: r0.numberOffset,
  };
  d = refillAnswers(startAnswers({ ...S.cfg, ...router, fireMode: r0.fireMode, salvoGroup: r0.salvoGroup }, S.setup), readKept());
  filled = S.setup;
  sentRouter = (r0.address || '').trim();
  // A kept address the server does not try now: move the link to it, as typing it would.
  if (d.router.trim() !== sentRouter) tryRouter();
  showBrand(d.brandText, d.brandBorder);
  at = reached = 0;
  finished = finishing = false;
  render();
}

function waitPage() {
  const phone = layout() === 'phone';
  $('setup-wait-title').textContent = phone ? 'Sode is not set up yet' : 'Setup is running in another browser';
  const kids = [];
  if (taken && !phone)
    kids.push(
      el('p', { class: 'pb-text-strong', id: 'setup-taken' }, 'Another browser took control. Your answers stay in this browser until you take control again.'),
    );
  else if (free && !phone) kids.push(el('p', { class: 'pb-text-strong', id: 'setup-free' }, 'The other browser closed or lost its connection.'));
  if (phone) kids.push(desktopOnly('set it up'));
  else {
    kids.push(
      el('button', { type: 'button', class: 'pb-btn pb-btn--outline', id: 'setup-take', 'data-pb-tip': 'The other browser keeps its answers' }, 'Take control'),
    );
  }
  $('setup-wait-body').replaceChildren(...kids);
}

function show() {
  const wizard = (mode === 'run' || mode === 'again') && layout() !== 'phone';
  const wait = !wizard && (mode === 'wait' || layout() === 'phone');
  $('setup-dialog').hidden = !wizard;
  $('setup-steps').hidden = !wizard;
  $('setup-wait').hidden = !wait;
  if (wait) waitPage();
}

// dropped: another browser took control, so this one's answers go.
function toWait(dropped) {
  d = null;
  mode = 'wait';
  if (dropped) taken = true;
  show();
}

$('setup-wait-body').addEventListener('click', (e) => {
  if (e.target.closest('#setup-take')) claim(true);
});

onMsg('setupClaimResult', (m) => {
  if (m.id !== claiming) return;
  claiming = 0;
  if (!S.setup || finished || finishing) return;
  if (!m.ok) return toWait(mode === 'run');
  mode = 'run';
  taken = free = false;
  // A reconnect lost the hold, and the server put back the address this page had moved.
  if (d && sentRouter && (S.routers[0]?.address || '') !== sentRouter) {
    sentRouter = '';
    tryRouter();
  }
  startWizard();
  show();
});

onMsg('setupTaken', () => {
  if (S.setup && mode === 'run' && !finished) toWait(true);
});

// Take control stays the only way to move setup, since the browser that closed can be a reload.
onMsg('setupFree', () => {
  if (!S.setup || mode !== 'wait') return;
  free = true;
  show();
});

on('hello', () => {
  if (mode === 'again') return;
  if (S.setup) {
    if (!mode) {
      sheetPresets = S.setup.presets || [];
      // Desktop shows nothing until the claim answers, so the first browser never sees the wait page.
      mode = layout() === 'phone' ? 'wait' : 'claim';
      root.setAttribute('data-sode-setup', '');
      $('setup').hidden = false;
      show();
    }
    // Every hello asks again: after a reconnect the server no longer knows this browser runs setup.
    if (!finished && !finishing) claim();
    paintDone();
    return;
  }
  // Another browser finished setup: this one has nothing to save any more.
  dropKept();
  if (mode && !finished && !finishing && !restoring()) openSode();
  paintDone();
});

on('conn', () => {
  paintRouter();
  paintDone();
});
on('names', () => {
  paintRouter();
  paintDone();
});
onLayout(() => {
  if (mode === 'again' && layout() === 'phone') return closeAgain(false);
  if (!mode) return;
  show();
  if (mode === 'wait') claim();
});

// Run setup again (setupagain.js): the same steps over Settings, with no Add a first user and no Done. Nothing is sent here;
// Finish hands the answers back to stage into the one Save.
export function openAgain(answers, presets, hooks) {
  if (mode || layout() === 'phone') return;
  d = answers;
  again = hooks;
  sheetPresets = presets;
  at = reached = 0;
  finished = finishing = false;
  mode = 'again';
  root.setAttribute('data-sode-setup', '');
  $('setup').hidden = false;
  show();
  render();
}

// finish: true hands the answers to Finish, false is Cancel. The brand goes back to the saved one either way,
// since the answers are not saved yet.
export function closeAgain(finish) {
  if (mode !== 'again') return;
  const answers = d;
  const hooks = again;
  d = again = null;
  mode = '';
  root.removeAttribute('data-sode-setup');
  $('setup').hidden = true;
  showBrand(S.cfg.brandText, S.cfg.brandBorder);
  if (finish) hooks.finish(answers);
  else hooks.cancel();
}

// Before settings.js, which leaves Settings on Escape.
document.addEventListener(
  'keydown',
  (e) => {
    if (mode !== 'again' || e.key !== 'Escape') return;
    e.preventDefault();
    e.stopImmediatePropagation();
    closeAgain(false);
  },
  true,
);
