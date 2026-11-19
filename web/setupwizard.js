// The setup wizard (docs/agents/server.md, setup.go): a config file with no users opens it in
// place of the patch page. Nothing is sent but the Router step's address until Finish.
import { $, S, el, on, onMsg, wsSend, icon, LINK_DOWN } from './core.js';
import { showBrand } from './brand.js';
import { keepSignInToken } from './signin.js';
import { desktopOnly } from './desktoponly.js';
import { layout, onLayout } from './layout.js';
import { restoring, wireRestore } from './setuprestore.js';
import {
  STEPS,
  DONE,
  PRESET_ROWS,
  PRESET_WHO,
  startAnswers,
  pinMax,
  stepProblem,
  stopAt,
  chipGoes,
  numberingChoices,
  finishMessage,
  routerLine,
  envHint,
} from './setupwizardrules.js';

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
const hint = (key, ...more) => [...more, envHint(filled, key)].filter(Boolean).join(' ');

// h: el with attributes; a false or null attribute is left out.
function h(tag, attrs, ...kids) {
  const e = el(tag, attrs?.class || '', ...kids);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (k === 'class' || v == null || v === false) continue;
    e.setAttribute(k, v === true ? '' : v);
  }
  return e;
}

function iconWith(name, cls) {
  const svg = icon(name);
  if (cls) svg.classList.add(cls);
  return svg;
}

function field(id, label, value, attrs = {}, hint = '') {
  const input = h('input', { id, class: 'pb-input', autocomplete: 'off', ...attrs });
  input.value = value;
  return h('div', { class: 'sode__field' }, h('label', { for: id }, label), input, hint ? h('span', { class: 'sode__hint' }, hint) : null);
}

function choice(name, label, opts, hint = '') {
  const group = h('span', { class: 'pb-btn-group', role: 'radiogroup', 'aria-labelledby': `setup-${name}-label` });
  for (const [v, t] of opts) {
    const id = `setup-${name}-${v}`;
    const input = h('input', { type: 'radio', class: 'pb-btn-check', name, id, value: v });
    input.checked = d[name] === v;
    group.append(input, h('label', { class: 'pb-btn pb-btn--outline', for: id }, t));
  }
  return h(
    'div',
    { class: 'sode__field' },
    h('span', { class: 'sode__field-label', id: `setup-${name}-label` }, label),
    group,
    hint ? h('span', { class: 'sode__hint' }, hint) : null,
  );
}

const section = (title, sub, ...inner) =>
  h('div', { class: 'sode-setup__section' }, h('h3', { class: 'sode-setup__h' }, title), sub ? h('p', { class: 'sode-setup__sub' }, sub) : null, ...inner);

const lead = (text) => h('p', { class: 'sode-setup__lead' }, text);
const strong = (text) => h('span', { class: 'pb-text-strong' }, text);

function statusLine() {
  // Run setup again moves no link before the save, so only the live address has a state.
  const line =
    d.again && d.router.trim() !== (S.cfg.router || '')
      ? { ok: false, text: 'Connects on Finish' }
      : routerLine(d.router, S.router, S.srcs.length + S.dsts.length);
  if (!line) return [];
  return [
    h(
      'span',
      { class: 'pb-status' },
      h('span', { class: `pb-dot ${line.ok ? 'pb-dot--ok' : 'pb-dot--warn'}`, 'aria-hidden': 'true' }),
      h('span', null, line.text),
    ),
  ];
}

const presets = () => sheetPresets;
const presetName = () => presets().find((p) => p.key === d.preset)?.name || d.preset;

function swatches() {
  const box = h('span', { class: 'sode-setup__swatches', role: 'radiogroup', 'aria-labelledby': 'setup-brand-label' });
  for (const c of [{ name: '', label: 'None' }, ...S.brandColors]) {
    const input = h('input', { type: 'radio', name: 'brandBorder', value: c.name, 'aria-label': c.label });
    input.checked = d.brandBorder === c.name;
    box.append(h('label', { class: 'sode-setup__swatch', 'data-pb-tip': c.label, 'data-brand': c.name || null }, input, c.name ? null : icon('ban')));
  }
  return h('div', { class: 'sode__field' }, h('span', { class: 'sode__field-label', id: 'setup-brand-label' }, 'Border color'), box);
}

function sheet() {
  const picked = (p) => (d.preset === p.key ? 'is-picked' : '');
  const head = h('tr', null, h('th'));
  for (const p of presets()) {
    const input = h('input', { type: 'radio', class: 'pb-check__input', name: 'preset', value: p.key });
    input.checked = d.preset === p.key;
    head.append(
      h(
        'th',
        { 'data-preset': p.key, class: picked(p) },
        h('label', { class: 'pb-check' }, input, h('span', { class: 'pb-check__label pb-text-strong' }, p.name)),
        h('span', { class: 'sode-setup__who' }, PRESET_WHO[p.key] || ''),
      ),
    );
  }
  const body = h('tbody');
  for (const [key, text] of PRESET_ROWS) {
    const row = h('tr', null, h('th', { scope: 'row' }, text));
    for (const p of presets()) {
      const mark = p.rows[key]
        ? h('span', { class: 'sode-setup__yes' }, iconWith('check', 'pb-icon--ok'), 'Yes')
        : h('span', { class: 'sode-setup__no' }, icon('x'), 'No');
      row.append(h('td', { 'data-preset': p.key, class: picked(p) }, mark));
    }
    body.append(row);
  }
  return h('table', { class: 'sode-setup__sheet' }, h('thead', null, head), body);
}

function doneText() {
  const addr = () => h('span', { class: 'pb-text-mono' }, d.router.trim());
  const linked = routerLine(d.router, S.router, S.srcs.length + S.dsts.length)?.ok;
  return [
    'You are signed in as ',
    strong(d.name.trim()),
    ...(linked ? [' and connected to the router at ', addr(), '.'] : ['. The router at ', addr(), ' has not answered yet, and Sode keeps trying.']),
    ' You chose ',
    strong(presetName()),
    '.',
  ];
}

const nextLink = (text, section, href = '#settings') => h('li', null, h('a', { href, 'data-section': section }, text));

const VIEW = {
  welcome: () => [
    d.again ? 'Run setup again' : 'Set up Sode',
    section('Name and color', '', field('setup-brandText', 'Name', d.brandText, { placeholder: 'Sode', maxlength: '24' }), swatches()),
    // replaceChildren draws a null as text, so Run setup again leaves the section out.
    ...(d.again
      ? []
      : [
          section(
            'Your user',
            'Administrator',
            field('setup-name', 'Name', d.name, { maxlength: '24' }),
            field('setup-pin', 'PIN', d.pin, { type: 'password', inputmode: 'numeric' }, `4 to ${max()} digits.`),
            field('setup-pin2', 'PIN again', d.pin2, { type: 'password', inputmode: 'numeric' }),
          ),
        ]),
  ],
  router: () => [
    'Router',
    section(
      'Connection',
      '',
      field('setup-router', 'Address', d.router, { placeholder: '10.0.0.5:8910' }, hint('router')),
      h('div', { id: 'setup-router-status' }, ...statusLine()),
    ),
    section(
      'Levels and numbers',
      '',
      field('setup-levels', 'Levels', d.levels, { type: 'number', min: '1', max: '256' }, hint('levels')),
      choice('levelNumbering', 'Level numbering', numberingChoices(d.levels), hint('levelNumbering')),
      choice(
        'numberOffset',
        'Source and destination numbers',
        [
          ['1', 'Start at 1'],
          ['0', 'Start at 0'],
        ],
        hint('numberOffset', 'Ultrix: 1'),
      ),
    ),
    section(
      'Takes',
      '',
      choice(
        'fireMode',
        'Multi destination',
        [
          ['sequential', 'One after another'],
          ['native', 'Router salvo'],
        ],
        hint('fireMode'),
      ),
      d.fireMode === 'native'
        ? field('setup-group', 'Router salvo group', d.group, { type: 'number', min: '0', max: '127' }, hint('group', 'Must be unused on the router'))
        : null,
    ),
  ],
  access: () => [
    'Access',
    lead('Without sign-in:'),
    ...(hint('preset') ? [h('p', { class: 'sode__hint', id: 'setup-preset-hint' }, hint('preset'))] : []),
    sheet(),
  ],
  done: () => [
    'Sode is set up',
    h(
      'div',
      { class: 'sode-setup__done' },
      h(
        'div',
        null,
        h('p', { id: 'setup-done-text' }, ...doneText()),
        h('button', { type: 'button', class: 'pb-btn pb-btn--ok pb-btn--lg', id: 'setup-open' }, 'Open Sode'),
      ),
      h(
        'nav',
        { class: 'sode-setup__next', 'aria-label': 'Next steps' },
        h('h3', null, 'Next steps'),
        h(
          'ul',
          null,
          nextLink('Add more users', 'users'),
          nextLink('Create your first panel', null, 'panels/editor'),
          nextLink('Connect a tablet or another app', 'api'),
          nextLink('Download a backup', 'export-import'),
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
    const chip = h(
      'button',
      { type: 'button', class: 'sode-setup__chip', 'data-go': go ? String(i) : null, tabindex: go ? null : '-1', 'aria-current': now ? 'step' : null },
      h('span', { class: 'sode-setup__dot' }, done ? icon('check') : String(i + 1)),
      h('span', { class: 'sode-setup__label' }, name),
    );
    list.push(h('li', { class: now ? 'is-now' : done ? 'is-done' : '' }, chip));
    if (i < all.length - 1) list.push(h('span', { class: `sode-setup__line ${i < Math.max(at, reached) ? 'is-done' : ''}` }));
  });
  $('setup-steps').replaceChildren(...list);
}

const KEYS = {
  'setup-brandText': 'brandText',
  'setup-name': 'name',
  'setup-pin': 'pin',
  'setup-pin2': 'pin2',
  'setup-router': 'router',
  'setup-levels': 'levels',
  'setup-group': 'group',
};

function render(focus) {
  const step = STEPS[at][0];
  $('setup').classList.toggle('sode-setup--wide', step === 'access');
  renderSteps();
  const [title, ...body] = VIEW[step]();
  $('setup-title').textContent = title;
  $('setup-body').replaceChildren(...body);
  $('setup-foot').hidden = step === 'done';
  $('setup-back').hidden = at === 0 && !d.again;
  $('setup-back').textContent = at === 0 ? 'Cancel' : 'Back';
  $('setup-restore').hidden = step !== 'welcome' || !!d.again;
  $('setup-next').hidden = false;
  $('setup-next').textContent = { welcome: 'Set up', access: 'Finish' }[step] || 'Next';
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
  if (k === 'brandText' || k === 'brandBorder') showBrand(d.brandText, d.brandBorder);
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
    render();
    return;
  }
  const next = e.target.closest('.sode-setup__next a[data-section]');
  if (next) {
    e.preventDefault();
    openSode();
    $('settings-open').click();
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
  d = startAnswers(S.cfg, S.setup);
  filled = S.setup;
  sentRouter = d.router.trim();
  at = reached = 0;
  finished = finishing = false;
  render();
}

function waitPage() {
  const phone = layout() === 'phone';
  $('setup-wait-title').textContent = phone ? 'Sode is not set up yet' : 'Setup runs in another browser';
  const kids = [];
  if (taken && !phone) kids.push(h('p', { class: 'pb-text-strong', id: 'setup-taken' }, 'Another browser took control. Your answers are lost.'));
  else if (free && !phone) kids.push(h('p', { class: 'pb-text-strong', id: 'setup-free' }, 'The other browser closed or lost its connection.'));
  if (phone) kids.push(desktopOnly('set it up'));
  else {
    kids.push(h('p', null, "Setup runs in one browser at a time. Take control discards the other browser's answers."));
    kids.push(h('button', { type: 'button', class: 'pb-btn pb-btn--outline', id: 'setup-take' }, 'Take control'));
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
  if (d && sentRouter && S.cfg.router !== sentRouter) {
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

// Run setup again (setupagain.js): the same steps over Settings, with no Your user and no Done. Nothing is sent here;
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
