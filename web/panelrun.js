// Runs one panel page (/panels/<id>): loads the panel, polls the router over /api/v1, keeps the state
// model and draws it. Every call goes out with the panel's key: the user's session key, or the
// key in the URL fragment for a panel that acts as its API client.
import { $ } from './core.js';
import { sessionFetch, startSession, signInAgain, onSessionKey } from './sessionkey.js';
import { withKey } from './sessionkeyrules.js';
import { keyFromHash, kioskOf, openFailText, envFrom, namesFrom, cellsIn, errorText } from './panelrules.js';
import {
  createState,
  pressDestination,
  pressSource,
  pressPreset,
  pressClear,
  startTake,
  startInstant,
  finishTake,
  failTake,
  applyTally,
  idleClear,
  errorOf,
  guardDown,
  guardHeld,
  guardCancel,
  GUARD_HOLD_MS,
} from './panelmodel.js';
import { buildGrid, paintGrid } from './panelview.js';
import { lookOf } from './panellook.js';
import { createArt } from './panelart.js';
import { createWake, buzz } from './panelfeel.js';

const POLL_MS = 2000;
const NAMES_MS = 30000;
const ID_BYTES = 8;

const say = (id, text) => {
  const n = $(id);
  n.textContent = text;
  n.hidden = !text;
};

// A random key per press: a retry of the same request replays its answer instead of taking twice.
function takeKey() {
  const b = crypto.getRandomValues(new Uint8Array(ID_BYTES));
  return `panel-${Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('')}`;
}

export async function runPanel(id) {
  const key = keyFromHash(location.hash);
  const call = key ? (path, init = {}) => fetch(path, withKey(init, key)) : sessionFetch;
  const get = async (path) => {
    const res = await call(path);
    if (!res.ok) throw new Error(`${path} ${res.status}`);
    return res.json();
  };

  if (!key) {
    $('panels-signin').addEventListener('click', signInAgain);
    onSessionKey((k, step) => {
      $('panels-signin').hidden = step !== 'sign-in';
      if (k) say('panels-note', '');
      else if (step === 'sign-in') say('panels-note', 'Sign in with your PIN to open this panel.');
      else if (step === 'no-users') say('panels-note', 'This panel needs a user, and Sode has none.');
    });
    startSession();
  }

  const res = await call(`api/v1/panels/${id}`);
  if (!res.ok) {
    say('panels-note', openFailText(res.status, !!key));
    return;
  }
  const panel = await res.json();
  if (panel.acts === 'api' && !key) {
    say('panels-note', 'This panel acts as its own API client. Open its link with the key.');
    return;
  }
  const set = panel.settings || {};
  const look = lookOf(set);
  const art = createArt(call, id);
  document.querySelector('.sode-panels__head').hidden = kioskOf(location.hash, panel) || !look.titleBar;
  document.title = panel.name;
  $('panel-title').textContent = panel.name;
  $('panel-grid').style.setProperty('--panel-fs', look.scale);
  if (look.backdrop) {
    art.url(look.backdrop).then((url) => {
      if (!url) return;
      const main = document.querySelector('.sode-panels');
      main.classList.add('sode-panels--backdrop');
      main.style.backgroundImage = `url("${url}")`;
    });
  }
  if (look.wake) createWake(navigator, document).on();

  const cells = cellsIn(panel);
  let st = createState({ mode: set.mode, multi: set.multi, guard: set.guard, levels: Array.isArray(set.levels) && set.levels.length ? set.levels : null });
  let env = { levels: [], tally: new Map(), protectedDsts: new Set() };
  let names = namesFrom({});
  let presets = {};
  let namesAt = 0;
  let ready = false;

  const render = () => {
    const now = Date.now();
    paintGrid(grid, { st, env, names, now });
    say('panel-status', errorText(errorOf(st, now), names));
  };
  const grid = buildGrid($('panel-grid'), panel, cells, onPress, { down: holdDown, cancel: holdCancel }, art);

  let polling = false;
  async function poll() {
    if (polling) return;
    polling = true;
    try {
      const wantNames = Date.now() - namesAt > NAMES_MS;
      // A key without the presets scope gets 403: its Preset cells then show as unavailable.
      const [tally, prot, nm, pr] = await Promise.all([get('api/v1/tally'), get('api/v1/protected'), wantNames ? get('api/v1/names') : null, wantNames ? get('api/v1/presets').catch(() => ({})) : null]);
      if (nm) {
        names = namesFrom(nm);
        presets = pr;
        namesAt = Date.now();
      }
      env = envFrom(tally, prot, presets);
      st = applyTally(st, env);
      ready = true;
      say('panels-note', '');
    } catch {
      say('panels-note', 'Lost contact with Sode. Retrying.');
    }
    polling = false;
    render();
  }

  async function send(started) {
    if (!started) return;
    st = started.state;
    render();
    try {
      const res = await call('api/v1/takes', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Idempotency-Key': takeKey() },
        body: JSON.stringify({ routes: started.routes }),
      });
      const body = await res.json().catch(() => ({}));
      st = res.ok ? finishTake(st, body, Date.now()) : failTake(st, body.error?.code || String(res.status), body.error?.message || 'The take was refused', Date.now());
    } catch {
      st = failTake(st, 'network', 'Sode did not answer, so the take may not have been sent', Date.now());
    }
    render();
    poll();
  }

  let holdTimer = 0;
  function holdCancel() {
    clearTimeout(holdTimer);
    st = guardCancel(st);
    render();
  }
  function holdDown() {
    if (!ready || st.guard !== 'hold' || !startTake(st, env, Date.now())) return;
    st = guardDown(st, Date.now()).state;
    clearTimeout(holdTimer);
    holdTimer = setTimeout(() => {
      const held = guardHeld(st, Date.now());
      st = held.state;
      if (held.go) send(startTake(st, env, Date.now()));
      render();
    }, GUARD_HOLD_MS);
    render();
  }

  function onPress(cell) {
    if (!ready) return;
    if (look.haptics) buzz(navigator);
    const now = Date.now();
    if (cell.kind === 'take') {
      if (st.guard === 'hold' || !startTake(st, env, now)) return;
      const down = guardDown(st, now);
      st = down.state;
      if (down.go) send(startTake(st, env, now));
      return render();
    }
    if (cell.kind === 'destination') st = pressDestination(st, cell, env, now);
    else if (cell.kind === 'source') st = pressSource(st, cell, env, now);
    else if (cell.kind === 'preset') st = pressPreset(st, cell, env, now);
    else if (cell.kind === 'clear') st = pressClear(st, now);
    else return;
    // Instant mode: a Source or Preset press is the Confirm.
    if (cell.kind === 'source' || cell.kind === 'preset') {
      const started = startInstant(st, env, now);
      if (started) return void send(started);
    }
    render();
  }

  setInterval(poll, POLL_MS);
  setInterval(() => {
    st = idleClear(st, Date.now());
    render();
  }, 1000);
  poll();
}
