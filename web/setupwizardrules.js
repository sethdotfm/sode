// The setup wizard's rules (pure): its steps, what each step needs before Next, and the Finish
// message. setupwizard.js draws them; internal/app/setup.go checks the answers again.

export const STEPS = [
  ['welcome', 'Welcome'],
  ['router', 'Router'],
  ['access', 'Access'],
  ['done', 'Done'],
];
export const DONE = STEPS.length - 1;

// The Access step's rows. The server sends each preset's Yes or No by these keys (setupRows).
export const PRESET_ROWS = [
  ['takeNoSignIn', 'Take without signing in'],
  ['sharedNoSignIn', 'Change shared presets and filters without signing in'],
  ['panelLinks', 'Panels open from their link'],
  ['appsRead', 'Other apps read names and tally without a key'],
  ['appsTake', 'Other apps take without a key'],
  ['pinWaits', 'Wrong PINs make you wait'],
];

export const PRESET_WHO = {
  open: 'One person, or a crew that trusts each other.',
  standard: 'A shared crew. Shared settings need a Trusted user.',
  secured: 'A fixed studio. Everyone signs in.',
};

const NAME_MAX = 24;
const PIN_MIN = 4;
const GROUP = 127; // the router's last group, which other control systems seldom use

// startAnswers: the wizard's fields from the hello's config, which SODE_* filled. Multi destination
// and its group take the config's value only when SODE_* set it (setup.env), else the wizard's own.
export function startAnswers(cfg = {}, setup = {}) {
  const env = setup.env || {};
  return {
    brandText: cfg.brandText || '',
    brandBorder: cfg.brandBorder || '',
    name: '',
    pin: '',
    pin2: '',
    router: cfg.router || '',
    levels: String(cfg.levels || ''),
    levelNumbering: cfg.levelNumbering || '1',
    numberOffset: String(cfg.numberOffset ?? 1),
    fireMode: (env.fireMode && cfg.fireMode) || 'sequential',
    group: String(env.group && cfg.salvoGroup != null ? cfg.salvoGroup : GROUP),
    preset: setup.preset || 'standard',
  };
}

// envHint: the line under a field that SODE_* filled on first start, or ''.
export function envHint(setup, key) {
  const name = setup?.env?.[key];
  return name ? `Filled from ${name}.` : '';
}

export const pinMax = (cfg = {}) => cfg.maxPinLen || 8;

const wholeIn = (s, lo, hi) => /^\d+$/.test(String(s).trim()) && +s >= lo && +s <= hi;

// stepProblem: why step i cannot go on, or '' when it can.
export function stepProblem(d, i, max = 8) {
  switch (STEPS[i]?.[0]) {
    case 'welcome': {
      if (d.again) return ''; // Run setup again skips Your user
      const name = d.name.trim();
      if (!name) return 'Enter your name.';
      if ([...name].length > NAME_MAX) return `A name has at most ${NAME_MAX} characters.`;
      return restorePinProblem(d.pin, d.pin2, max);
    }
    case 'router':
      if (!d.router.trim()) return 'Enter the router address.';
      if (!wholeIn(d.levels, 1, 256)) return 'Levels is a number from 1 to 256.';
      if (d.fireMode === 'native' && !wholeIn(d.group, 0, 127)) return 'Router salvo group is a number from 0 to 127.';
      return '';
  }
  return '';
}

// restorePinProblem: why a new PIN and PIN again cannot be used, or ''. Your user and a restore without PINs use it.
export function restorePinProblem(pin, pin2, max = 8) {
  if (!/^\d+$/.test(pin) || pin.length < PIN_MIN || pin.length > max) return `A PIN has ${PIN_MIN} to ${max} digits.`;
  if (pin !== pin2) return 'The two PINs are not the same.';
  return '';
}

// stopAt: the furthest step at or before want that every earlier step lets through.
export function stopAt(d, want, max) {
  for (let i = 0; i < want; i++) if (stepProblem(d, i, max)) return i;
  return want;
}

// chipGoes: a chip moves to a visited step until Finish; after it every step is locked.
export const chipGoes = (i, at, reached, finished) => !finished && i !== at && i <= reached;

// numberingChoices: the Level numbering buttons for n levels.
export function numberingChoices(n) {
  const top = wholeIn(n, 1, 256) ? +n : 0;
  const to = (a, b) => (top ? `${a} to ${b}` : a);
  return [
    ['1', to(1, top)],
    ['0', to(0, top - 1)],
    ['va', top ? `V + 1 to ${top - 1}` : 'V + 1'],
  ];
}

// finishMessage: the setupFinish message. PIN again stays on the page.
export function finishMessage(d, id) {
  return {
    t: 'setupFinish',
    id,
    setup: {
      brandText: d.brandText.trim(),
      brandBorder: d.brandBorder,
      name: d.name.trim(),
      pin: d.pin,
      router: d.router.trim(),
      levels: +d.levels,
      levelNumbering: d.levelNumbering,
      numberOffset: +d.numberOffset,
      fireMode: d.fireMode,
      group: d.fireMode === 'native' ? +d.group : GROUP,
      preset: d.preset,
    },
  };
}

// routerLine: the Router step's status line, or null before an address is typed.
export function routerLine(address, router, names) {
  if (!address.trim()) return null;
  if (router === 'up' && names > 0) return { ok: true, text: 'Connected. Names read.' };
  return { ok: false, text: 'No answer yet. You can go on, and Sode keeps trying.' };
}
