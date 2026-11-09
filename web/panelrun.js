// Runs one panel page (/panels/<id>): loads the panel, polls the router over /api/v1, keeps the state
// model and draws it. Every call goes out with the panel's key: the user's session key, or the
// key in the URL fragment for a panel that acts as its API client.
import { $ } from './core.js';
import { sessionFetch, jsonCall, startSession, signInAgain, onSessionKey, nameSessionKey } from './sessionkey.js';
import { startKeepalive } from './panelkeepalive.js';
import { withKey } from './sessionkeyrules.js';
import { keyFromHash, kioskOf, openFailText, envFrom, namesFrom, cellsIn, errorText } from './panelrules.js';
import { setupFromHash, pollFail } from './panelkeyrules.js';
import { exchangeSetup, storedKey, forgetKey } from './panelsetup.js';
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
import { createWake, buzz, enterFullscreen } from './panelfeel.js';
import { watchPanel } from './panellive.js';
import { instanceFrom } from './panellink.js';

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

export async function runPanel(seg) {
  let id = seg; // the id once the panel is read: the address may hold its short name
  let key = keyFromHash(location.hash);
  const setup = setupFromHash(location.hash);
  if (!key && setup) {
    const got = await exchangeSetup(seg, setup);
    if (got.error) return say('panels-note', got.error);
    key = got.key;
  }
  const stored = !key;
  if (stored) key = storedKey(seg);
  const call = key ? (path, init = {}) => fetch(path, withKey(init, key)) : sessionFetch;
  const callJSON = jsonCall(call);

  if (!key) {
    $('panels-signin').addEventListener('click', signInAgain);
    onSessionKey((k, step) => {
      $('panels-signin').hidden = step !== 'sign-in';
      if (k) say('panels-note', '');
      else if (step === 'sign-in') say('panels-note', 'Sign in with your PIN to open this panel.');
      else if (step === 'no-users') say('panels-note', 'This panel needs a user, and Sode has none.');
    });
    nameSessionKey(`Panel ${seg}`);
    startSession();
  }

  let panel;
  try {
    panel = await callJSON(`api/v1/panels/${seg}`);
  } catch (e) {
    if (e.status === undefined) throw e;
    // A 404 forgets too: a short name that moved to another panel must not keep sending the old panel's key.
    if ((e.status === 401 || e.status === 404) && stored && key) forgetKey(seg);
    say('panels-note', e.status === 403 && e.code === 'network_not_allowed' ? e.message : openFailText(e.status, !!key));
    return;
  }
  if (panel.acts === 'api' && !key) {
    say('panels-note', 'This panel acts as its own API client. Open its link with the key.');
    return;
  }
  id = panel.id;
  const art = createArt(call, id);
  // Title, header, font scale and backdrop come from the stored panel, so a live update runs it again.
  const applyLook = (p) => {
    const look = lookOf(p.settings || {});
    document.querySelector('.sode-panels__head').hidden = kioskOf(location.hash, p) || !look.titleBar;
    document.title = p.name;
    $('panel-title').textContent = p.name;
    $('panel-grid').style.setProperty('--panel-fs', look.scale);
    const main = document.querySelector('.sode-panels');
    main.classList.toggle('sode-panels--backdrop', !!look.backdrop);
    main.style.backgroundImage = '';
    if (look.backdrop) {
      art.url(look.backdrop).then((url) => {
        if (url) main.style.backgroundImage = `url("${url}")`;
      });
    }
    return look;
  };
  let look = applyLook(panel);
  if (look.wake) createWake(navigator, document).on();

  let cells = cellsIn(panel);
  const stateOf = (p) => {
    const set = p.settings || {};
    return createState({
      mode: set.mode,
      multi: set.multi,
      guard: set.guard,
      resultText: set.resultText,
      levels: Array.isArray(set.levels) && set.levels.length ? set.levels : null,
    });
  };
  let st = stateOf(panel);
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
  let grid = buildGrid($('panel-grid'), panel, cells, onPress, { down: holdDown, cancel: holdCancel }, art);

  let polling = false;
  let ended = false; // the key is gone or refused: only a new link helps
  async function poll() {
    if (polling || ended) return;
    polling = true;
    try {
      const wantNames = Date.now() - namesAt > NAMES_MS;
      // A key without the presets scope gets 403: its Preset cells then show as unavailable.
      const [tally, prot, nm, pr] = await Promise.all([
        callJSON('api/v1/tally'),
        callJSON('api/v1/protected'),
        wantNames ? callJSON('api/v1/names') : null,
        wantNames ? callJSON('api/v1/presets').catch(() => ({})) : null,
      ]);
      if (nm) {
        names = namesFrom(nm);
        presets = pr;
        namesAt = Date.now();
      }
      env = envFrom(tally, prot, presets, panel.rights);
      st = applyTally(st, env);
      ready = true;
      say('panels-note', '');
    } catch (e) {
      const f = pollFail(e.status, e.code, e.status === undefined ? undefined : e.message || undefined, !!key);
      say('panels-note', f.text);
      if (f.stop) {
        ended = true;
        ready = false;
        if (stored) forgetKey(seg);
      }
    }
    polling = false;
    render();
  }

  async function send(started) {
    if (!started) return;
    st = started.state;
    render();
    try {
      const body = await callJSON('api/v1/takes', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Idempotency-Key': takeKey() },
        body: JSON.stringify({ routes: started.routes }),
      });
      st = finishTake(st, body || {}, Date.now());
    } catch (e) {
      st =
        e.status === undefined
          ? failTake(st, 'network', 'Sode did not answer, so the take may not have been sent', Date.now())
          : failTake(st, e.code || String(e.status), e.message || 'The take was refused', Date.now());
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
    if (look.fullscreen) enterFullscreen(document);
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

  startKeepalive(call, id, $('panel-device'), instanceFrom(location.search));
  // A newer saved copy: rebuild the grid and the state from it. The watcher waits for a quiet moment first.
  watchPanel({
    id,
    rev: panel.rev,
    get: callJSON,
    state: () => ({ viewed: !!st.view || st.staged.length > 0, busy: st.busy }),
    apply(p) {
      clearTimeout(holdTimer);
      panel = p;
      cells = cellsIn(p);
      st = stateOf(p);
      look = applyLook(p);
      grid = buildGrid($('panel-grid'), p, cells, onPress, { down: holdDown, cancel: holdCancel }, art);
      render();
    },
  });
  setInterval(poll, POLL_MS);
  setInterval(() => {
    st = idleClear(st, Date.now());
    render();
  }, 1000);
  poll();
}
