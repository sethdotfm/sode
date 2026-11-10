// Runs one panel (the page /panels/<id>, or the editor's live test): loads the panel, polls the router over
// /api/v1, keeps the state model and draws it. Every call goes out with the panel's key: the user's session
// key, or the key in the URL fragment for a panel that acts as its API client. The editor's live test passes
// its own ui and call, and is not an instance of the panel: no keepalive, no device name.
import { sessionFetch, jsonCall, startSession, signInAgain, onSessionKey, nameSessionKey } from './sessionkey.js';
import { pageUi } from './panelmount.js';
import { startKeepalive, announceKeyless } from './panelkeepalive.js';
import { withKey, guardFetch } from './sessionkeyrules.js';
import { fromPanel } from './panelwatchrules.js';
import { keyFromHash, kioskOf, openFailText, sessionNote, envFrom, namesFrom, drawnCells, errorText, offlineOf } from './panelrules.js';
import { setupFromHash, pollFail } from './panelkeyrules.js';
import { exchangeSetup, storedKey, forgetKey, keepLinkKey, followAssign } from './panelsetup.js';
import {
  createState,
  pressDestination,
  pressSource,
  pressPreset,
  clear,
  startTake,
  startInstant,
  finishTake,
  failTake,
  applyTally,
  carryView,
  showPage,
  pressPage,
  errorOf,
  guardDown,
  guardHeld,
  guardCancel,
  GUARD_HOLD_MS,
} from './panelmodel.js';
import { buildGrid, paintGrid } from './panelview.js';
import { lookOf, withDefault, defaultsBy } from './panellook.js';
import { createArt } from './panelart.js';
import { createWake, enterFullscreen, featureNotes, wantsFullscreen } from './panelfeel.js';
import { watchPanel } from './panellive.js';
import { instanceFrom } from './panellink.js';
import { initChrome } from './panelfull.js';
import { initName } from './panelname.js';
import { pagesOf } from './panelpages.js';
import { NAV_KINDS, landingPage, pageIndex, wrapOf } from './panelnav.js';
import { savedPage, keepPage } from './pagekeep.js';

const POLL_MS = 2000;
const NAMES_MS = 30000;
const ID_BYTES = 8;

const say = (n, text) => {
  n.textContent = text;
  n.hidden = !text;
};

// A random key per press: a retry of the same request replays its answer instead of taking twice.
function takeKey() {
  const b = crypto.getRandomValues(new Uint8Array(ID_BYTES));
  return `panel-${Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('')}`;
}

// Returns { stop } at once; stop ends every timer, for an embedded run that goes away.
export function runPanel(seg, opts = {}) {
  const run = { stop: () => {} };
  start(seg, opts, run);
  return run;
}

async function start(seg, { ui = pageUi(), call: given, embedded = !!given } = {}, run) {
  const timers = [];
  let stopWatch = () => {};
  let ended = false; // the key is gone or refused, or an embedded run was stopped: only a new link or start helps
  let holdTimer = 0;
  let art = null;
  let wake = null;
  // While the page has no working key it is a Keyless instance: listed for an Assign, never sent anything else.
  let keyless = null;
  const unannounce = () => {
    keyless?.stop();
    keyless = null;
  };
  run.stop = () => {
    ended = true;
    timers.forEach(clearInterval);
    stopWatch();
    clearTimeout(holdTimer);
    art?.revoke();
    unannounce();
  };
  const announce = (keyEnded) => {
    if (embedded || (keyless && keyless.keyEnded === keyEnded)) return;
    unannounce();
    keyless = {
      keyEnded,
      ...announceKeyless(seg, keyEnded, (msg) => {
        run.stop();
        followAssign(msg);
      }),
    };
  };
  const endedCode = (code) => code === 'key_revoked' || code === 'key_expired';
  let id = seg; // the id once the panel is read: the address may hold its short name
  let key = embedded ? null : keyFromHash(location.hash);
  const setup = embedded ? '' : setupFromHash(location.hash);
  if (!key && setup) {
    const got = await exchangeSetup(seg, setup);
    if (ended) return;
    if (got.error) {
      announce(false);
      return say(ui.note, got.error);
    }
    key = got.key;
  }
  const linked = !!key;
  // A key that sits in this browser's storage, so an ended one is forgotten there.
  let stored = !key && !embedded;
  if (stored) key = storedKey(seg);
  // Before the first read: a Session-mode page waits there for a sign-in that may never come.
  if (!key) announce(false);
  // Every call names its panel and instance for the Communication Log; the editor's live test is no instance.
  let instance = '';
  const tag = (init) => fromPanel(init, id, instance);
  const keyed = key && guardFetch((path, init) => fetch(path, withKey(tag(init), key)), { final: (s) => s === 401 });
  const call = given || (key ? (path, init = {}) => keyed(path, init) : (path, init = {}) => sessionFetch(path, tag(init)));
  const callJSON = jsonCall(call);

  if (!key && !embedded) {
    ui.signin.addEventListener('click', signInAgain);
    onSessionKey((k, step, err) => {
      if (step === 'ended') announce(true);
      const n = sessionNote(step, err);
      ui.signin.hidden = !n.retry;
      if (k) say(ui.note, '');
      else if (n.text) say(ui.note, n.text);
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
    if (e.status === 401) announce(endedCode(e.code));
    say(ui.note, e.status === 403 && e.code === 'network_not_allowed' ? e.message : openFailText(e.status, !!key));
    return;
  }
  // Back to editing came before the panel did: nothing below may start a timer or a watch.
  if (ended) return;
  // Kept only once it worked, so a mistyped or ended key never replaces a stored one.
  if (linked) stored = keepLinkKey(seg, key);
  if (panel.acts === 'api' && !key && !embedded) {
    say(ui.note, 'This panel acts as its own API client. Open its link with the key.');
    return;
  }
  id = panel.id;
  art = createArt(call, id);
  // Title, header, font scale and theme come from the stored panel, so a live update runs it again.
  // Auto restores whatever the page started with (the browser's own choice or the device setting).
  const root = document.documentElement;
  const startTheme = root.dataset.theme;
  const setTheme = (t) => {
    // Embedded, the panel's tokens switch with color-scheme and the editor around it keeps its own theme.
    if (embedded) {
      ui.main.style.colorScheme = t === 'auto' ? '' : t;
      return;
    }
    const want = t === 'auto' ? startTheme : t;
    if (want) root.dataset.theme = want;
    else delete root.dataset.theme;
  };
  const chrome = embedded ? null : initChrome(ui);
  const applyLook = (p) => {
    const look = lookOf(p.settings || {});
    if (!embedded) {
      chrome.show(kioskOf(location.hash, p), look.titleBar);
      document.title = p.name;
      document.documentElement.toggleAttribute('data-sode-landscape', look.landscape);
    }
    ui.title.textContent = p.name;
    ui.grid.style.setProperty('--panel-fs', look.scale);
    setTheme(look.theme);
    return look;
  };
  let look = applyLook(panel);
  if (!embedded) {
    if (look.wake) {
      wake = createWake(navigator, document);
      wake.on();
    }
    const notes = featureNotes(look, navigator, window.isSecureContext);
    if (notes.length) say(ui.note, notes.join(' '));
  }

  // The current Page's cells; a live update or a Page change makes them again.
  const cellsOf = (p, page) => drawnCells(p, pageIndex(pagesOf(p), page)).map((c) => withDefault(c, defaultsBy(p.artDefaults)));
  // Every Page's cells, so a live update keeps a Destination viewed on another Page.
  const everyCell = (p) => pagesOf(p).flatMap((_, i) => drawnCells(p, i));
  const stateOf = (p) => {
    const set = p.settings || {};
    return createState({
      mode: set.mode,
      multi: set.multi,
      guard: set.guard,
      resultText: set.resultText,
      levels: Array.isArray(set.levels) && set.levels.length ? set.levels : null,
      fixedDests: set.fixedDests,
    });
  };
  // The Test it live run is no instance, so it neither reads nor writes the browser's saved Page.
  let st = showPage(stateOf(panel), landingPage(pagesOf(panel), embedded ? '' : savedPage(id)));
  let cells = cellsOf(panel, st.page);
  const notePage = () => {
    if (!embedded) keepPage(id, st.page);
  };
  notePage();
  let env = { levels: [], tally: new Map(), protectedDsts: new Set() };
  let names = namesFrom({});
  let presets = {};
  let namesAt = 0;
  let ready = false;

  const render = () => {
    const now = Date.now();
    paintGrid(grid, { st, env, names, now, panelName: panel.name, instance, pages: pagesOf(panel), wrap: wrapOf(panel) });
    say(ui.status, errorText(errorOf(st, now), names));
  };
  let grid = buildGrid(ui.grid, panel, cells, onPress, { down: holdDown, cancel: holdCancel }, art);

  // Blur the whole panel and say Offline, so nobody takes a stale grid for a working one.
  const showOffline = (o) => {
    ui.main.classList.toggle('sode-panels--offline', !!o);
    ui.offline.hidden = !o;
    if (o) ui.offlineDetail.textContent = o.detail;
  };

  let polling = false;
  async function poll() {
    if (polling || ended) return;
    polling = true;
    try {
      const wantNames = Date.now() - namesAt > NAMES_MS;
      // A key without the presets scope gets 403: its Preset cells then show as unavailable.
      const [tally, prot, nm, pr, link] = await Promise.all([
        callJSON('api/v1/tally'),
        callJSON('api/v1/protected'),
        wantNames ? callJSON('api/v1/names') : null,
        wantNames ? callJSON('api/v1/presets').catch(() => ({})) : null,
        // A key without the status scope gets 403: the router link is then unknown, not down.
        callJSON('api/v1/status').catch((e) => {
          if (e.status === undefined || e.status >= 500) throw e;
          return null;
        }),
      ]);
      if (nm) {
        names = namesFrom(nm);
        presets = pr;
        namesAt = Date.now();
      }
      env = envFrom(tally, prot, presets, panel.rights);
      st = applyTally(st, env);
      const off = offlineOf({ failed: false, routerUp: link ? link.routerUp : true });
      showOffline(off);
      ready = !off;
      say(ui.note, '');
    } catch (e) {
      const f = pollFail(e.status, e.code, e.status === undefined ? undefined : e.message || undefined, !!key);
      say(ui.note, f.text);
      if (!f.stop) {
        ready = false;
        // A refusal (a session not signed in yet) is not a lost link.
        if (e.status === undefined || e.status >= 500) showOffline(offlineOf({ failed: true, routerUp: true }));
      }
      if (f.stop) {
        ended = true;
        ready = false;
        if (stored) forgetKey(seg);
        // Nothing may call Sode again: not the polls, the update check or the keepalive.
        timers.forEach(clearInterval);
        stopWatch();
        keep?.stop();
        // Only the keyless announcement, which carries no key.
        if (e.status === 401) announce(endedCode(e.code));
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

  function holdCancel() {
    clearTimeout(holdTimer);
    st = guardCancel(st);
    render();
  }
  function holdDown() {
    if (!ready || st.guard !== 'hold' || !startTake(st, env)) return;
    st = guardDown(st, Date.now()).state;
    clearTimeout(holdTimer);
    holdTimer = setTimeout(() => {
      const held = guardHeld(st, Date.now());
      st = held.state;
      if (held.go) send(startTake(st, env));
      render();
    }, GUARD_HOLD_MS);
    render();
  }

  function onPress(cell) {
    // A Page change sends nothing, so it works while the router or Sode is away.
    const nav = NAV_KINDS.includes(cell.kind);
    if (!ready && !nav) return;
    wake?.retry();
    if (wantsFullscreen(look) && !embedded) enterFullscreen(document, look.landscape);
    if (nav) return showNewPage(pressPage(st, cell, pagesOf(panel), wrapOf(panel)));
    const now = Date.now();
    if (cell.kind === 'take') {
      if (st.guard === 'hold' || !startTake(st, env)) return;
      const down = guardDown(st, now);
      st = down.state;
      if (down.go) send(startTake(st, env));
      return render();
    }
    if (cell.kind === 'destination') st = pressDestination(st, cell);
    else if (cell.kind === 'source') st = pressSource(st, cell, env, now);
    else if (cell.kind === 'preset') st = pressPreset(st, cell, env, now);
    else if (cell.kind === 'clear') st = clear(st);
    else return;
    // Instant mode: a Source or Preset press is the Confirm.
    if (cell.kind === 'source' || cell.kind === 'preset') {
      const started = startInstant(st, env);
      if (started) return void send(started);
    }
    render();
  }

  // A Page change keeps the plan and rebuilds the grid from the new Page's cells.
  function showNewPage(next) {
    if (next === st) return;
    clearTimeout(holdTimer);
    st = next;
    notePage();
    cells = cellsOf(panel, st.page);
    grid = buildGrid(ui.grid, panel, cells, onPress, { down: holdDown, cancel: holdCancel }, art);
    render();
  }

  let keep = null;
  if (!embedded) {
    // The page runs with a key now and is an instance, so it is no longer listed as keyless.
    unannounce();
    const showName = initName(ui.name, (name) => keep.rename(name));
    keep = startKeepalive(
      call,
      id,
      ui.device,
      instanceFrom(location.search),
      (name, editable) => {
        instance = name;
        showName(name, editable);
        render();
      },
      (msg) => {
        run.stop();
        followAssign(msg);
      },
    );
  }
  // A newer saved copy: rebuild the grid and the state from it, keeping the viewed destination. The watcher counts down and waits out staged routes first.
  stopWatch = watchPanel({
    ui,
    id,
    panel,
    get: callJSON,
    state: () => ({ staged: st.staged.length > 0, busy: st.busy }),
    apply(p) {
      clearTimeout(holdTimer);
      panel = p;
      // The current Page stays unless the edit disabled or deleted it, then the first enabled Page shows.
      st = showPage(carryView(stateOf(p), st, everyCell(p)), landingPage(pagesOf(p), st.page));
      notePage();
      cells = cellsOf(p, st.page);
      look = applyLook(p);
      grid = buildGrid(ui.grid, p, cells, onPress, { down: holdDown, cancel: holdCancel }, art);
      render();
    },
  });
  // Results and errors fade on their own clocks (RESULT_MS, ERROR_MS), so the grid is redrawn every second.
  timers.push(setInterval(poll, POLL_MS), setInterval(render, 1000));
  poll();
}
