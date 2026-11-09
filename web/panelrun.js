// Runs one panel page (/panels/<id>): loads the panel, polls the router over /api/v1, keeps the state
// model and draws it. Every call goes out with the panel's key: the user's session key, or the
// key in the URL fragment for a panel that acts as its API client.
import { $ } from './core.js';
import { sessionFetch, startSession, signInAgain, onSessionKey } from './sessionkey.js';
import { withKey } from './sessionkeyrules.js';
import { keyFromHash, kioskOf, openFailText, envFrom, namesFrom, cellsIn, errorText } from './panelrules.js';
import { createState, pressDestination, pressSource, startTake, finishTake, failTake, applyTally, idleClear, errorOf } from './panelmodel.js';
import { buildGrid, paintGrid } from './panelview.js';

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
  document.querySelector('.sode-panels__head').hidden = kioskOf(location.hash, panel);
  document.title = panel.name;
  $('panel-title').textContent = panel.name;

  const cells = cellsIn(panel);
  const set = panel.settings || {};
  let st = createState({ mode: set.mode, multi: set.multi, levels: Array.isArray(set.levels) && set.levels.length ? set.levels : null });
  let env = { levels: [], tally: new Map(), protectedDsts: new Set() };
  let names = namesFrom({});
  let namesAt = 0;
  let ready = false;

  const render = () => {
    const now = Date.now();
    paintGrid(grid, { st, env, names, now });
    say('panel-status', errorText(errorOf(st, now), names));
  };
  const grid = buildGrid($('panel-grid'), panel, cells, onPress);

  let polling = false;
  async function poll() {
    if (polling) return;
    polling = true;
    try {
      const wantNames = Date.now() - namesAt > NAMES_MS;
      const [tally, prot, nm] = await Promise.all([get('api/v1/tally'), get('api/v1/protected'), wantNames ? get('api/v1/names') : null]);
      if (nm) {
        names = namesFrom(nm);
        namesAt = Date.now();
      }
      env = envFrom(tally, prot);
      st = applyTally(st, env);
      ready = true;
      say('panels-note', '');
    } catch {
      say('panels-note', 'Lost contact with Sode. Retrying.');
    }
    polling = false;
    render();
  }

  async function take() {
    const started = startTake(st, env, Date.now());
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

  function onPress(cell) {
    if (!ready) return;
    const now = Date.now();
    if (cell.kind === 'destination') st = pressDestination(st, cell, env, now);
    else if (cell.kind === 'source') st = pressSource(st, cell, env, now);
    else return void take();
    render();
  }

  setInterval(poll, POLL_MS);
  setInterval(() => {
    st = idleClear(st, Date.now());
    render();
  }, 1000);
  poll();
}
