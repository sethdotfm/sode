// The setup wizard's rules (pure): its steps, what each step needs before Next, and the Finish
// message. setupwizard.js draws them; internal/app/setup.go checks the answers again.
import { levelLabelOf } from './levels.js';
import { numberingRange } from './routerdetailrules.js';

export const STEPS = [
  ['welcome', 'Style & User'],
  ['router', 'Router'],
  ['access', 'Access'],
  ['done', 'Done'],
];
export const DONE = STEPS.length - 1;

// The Access step's rows. The server sends each preset's Yes or No by these keys (setupRows).
export const PRESET_ROWS = [
  ['takeNoSignIn', 'Change routes without signing in'],
  ['sharedNoSignIn', 'Change shared presets and filters without signing in'],
  ['panelLinks', 'Public panels open without signing in'],
  ['appsRead', 'Other apps can read names and tally without a key'],
  ['appsTake', 'Other apps can take without a key'],
  ['tieNoSignIn', 'Change routes across routers without signing in'],
];

const NAME_MAX = 24;
const TITLE_MAX = 32; // a Router title, as config.Router.problems checks it
const PIN_MIN = 4;
const GROUP = 127; // the router's last group, which other control systems seldom use

// The wizard's fields from the hello's config, which SODE_* filled. Multi destination and its group use the config's value only when SODE_* set it (setup.env).
export function startAnswers(cfg = {}, setup = {}) {
  const env = setup.env || {};
  return {
    brandText: cfg.brandText || '',
    brandBorder: cfg.brandBorder || '',
    name: '',
    pin: '',
    pin2: '',
    // The same default title as Add router in Settings > Routers: "Router A".
    routerTitle: cfg.routerTitle || `Router ${cfg.routerPrefix || 'A'}`,
    router: cfg.router || '',
    levels: String(cfg.levels || ''),
    levelNumbering: cfg.levelNumbering || '1',
    numberOffset: String(cfg.numberOffset ?? 1),
    fireMode: (env.fireMode && cfg.fireMode) || 'sequential',
    group: String(env.group && cfg.salvoGroup != null ? cfg.salvoGroup : GROUP),
    preset: setup.preset || 'standard',
  };
}

// Each tab keeps its answers in sessionStorage, so a tab that lost setup to Take control gets them back when it takes
// control again. sessionStorage, not localStorage: two tabs of one browser share localStorage, and a shared machine
// would keep a name and a router address. Never the PINs.
export const KEPT_KEY = 'sode-setup-answers';
const KEPT = ['brandText', 'brandBorder', 'name', 'routerTitle', 'router', 'levels', 'levelNumbering', 'numberOffset', 'fireMode', 'group', 'preset'];

export const keptAnswers = (d) => Object.fromEntries(KEPT.map((k) => [k, d[k]]));

// The start answers with each kept answer put back; anything that is not a kept string is left out.
export function refillAnswers(d, kept) {
  const out = { ...d };
  if (!kept || typeof kept !== 'object') return out;
  for (const k of KEPT) if (typeof kept[k] === 'string') out[k] = kept[k];
  return out;
}

// '' when SODE_* did not fill it.
export function envHint(setup, key) {
  const name = setup?.env?.[key];
  return name ? `Filled from ${name}.` : '';
}

export const pinMax = (cfg = {}) => cfg.maxPinLen || 8;

const wholeIn = (s, lo, hi) => /^\d+$/.test(String(s).trim()) && +s >= lo && +s <= hi;

export function stepProblem(d, i, max = 8) {
  switch (STEPS[i]?.[0]) {
    case 'welcome': {
      if (d.again) return ''; // Run setup again skips Add a first user
      const name = d.name.trim();
      if (!name) return 'Enter your name.';
      if ([...name].length > NAME_MAX) return `A name has at most ${NAME_MAX} characters.`;
      return restorePinProblem(d.pin, d.pin2, max);
    }
    case 'router':
      if ([...(d.routerTitle || '').trim()].length > TITLE_MAX) return `A Router title has at most ${TITLE_MAX} characters.`;
      if (!d.router.trim()) return 'Enter the router address.';
      if (!wholeIn(d.levels, 1, 256)) return 'Levels is a number from 1 to 256.';
      if (d.fireMode === 'native' && !wholeIn(d.group, 0, 127)) return 'Router salvo group is a number from 0 to 127.';
      return '';
  }
  return '';
}

// Add a first user and a restore without PINs use it.
export function restorePinProblem(pin, pin2, max = 8) {
  if (!/^\d+$/.test(pin) || pin.length < PIN_MIN || pin.length > max) return `A PIN has ${PIN_MIN} to ${max} digits.`;
  if (pin !== pin2) return 'The PINs differ.';
  return '';
}

// The furthest step at or before want that every earlier step lets through.
export function stopAt(d, want, max) {
  for (let i = 0; i < want; i++) if (stepProblem(d, i, max)) return i;
  return want;
}

// A chip moves to a visited step until Finish; after it every step is locked.
export const chipGoes = (i, at, reached, finished) => !finished && i !== at && i <= reached;

// The three Level numbering buttons show the real range of the router, as its detail view in Settings > Routers does:
// "1–17", "0–16" and "V, 1–16".
export function numberingChoices(n) {
  const top = wholeIn(n, 1, 256) ? +n : NaN;
  return ['1', '0', 'va'].map((mode) => [mode, numberingRange(top, mode)]);
}

// Repeat your PIN stays on the page.
export function finishMessage(d, id) {
  return {
    t: 'setupFinish',
    id,
    setup: {
      brandText: d.brandText.trim(),
      brandBorder: d.brandBorder,
      name: d.name.trim(),
      pin: d.pin,
      title: (d.routerTitle || '').trim(),
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

const count = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`;

// The Router step's status line, or null before an address is typed.
export function routerLine(address, router, srcs, dsts) {
  if (!address.trim()) return null;
  if (router === 'up' && srcs + dsts > 0) return { ok: true, text: `Connected: ${count(srcs, 'source')} and ${count(dsts, 'destination')} discovered.` };
  return { ok: false, text: 'No answer yet. Sode keeps trying.' };
}

// The Level numbering guide's chips: the typed count in the picked numbering, colored as the router's levels are.
// known is the hello's levels; a level past them gets the swatch the server would give it (config.LevelMap).
export function guideLevels(n, numbering, known = []) {
  const top = wholeIn(n, 1, 256) ? +n : 0;
  return Array.from({ length: top }, (_, i) => {
    const ui = i + 1;
    const label = levelLabelOf([], numbering, ui);
    const k = known[i];
    if (k?.color) return { ui, label, color: k.color };
    return { ui, label, swatch: k?.swatch || (ui === 1 ? 'video' : String(((ui - 2) % 16) + 1).padStart(2, '0')) };
  });
}
