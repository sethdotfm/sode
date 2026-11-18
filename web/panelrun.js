// Runs one panel (the page /panels/<id>, or the editor's live test). It loads the panel, polls the router over
// /api/v1, keeps the state model and draws it. Every call goes out with the panel's key. That key is the user's
// session key or, for a panel that acts as its API client, the key in the URL fragment. The third kind is a Public
// panel's short-lived key (panelpublic.go). The editor's live test passes its own ui and call, and is not an instance of the panel: no
// keepalive, no device name.
import { emit, on, S } from './core.js';
import { sessionFetch, jsonCall, startSession, signInAgain, onSessionKey, nameSessionKey } from './sessionkey.js';
import { pageUi } from './panelmount.js';
import { startKeepalive, announceKeyless } from './panelkeepalive.js';
import { withKey, guardFetch } from './sessionkeyrules.js';
import { fromPanel } from './panelwatchrules.js';
import { keyFromHash, kioskOf, openFailText, sessionNote, offlineOf } from './panelrules.js';
import { setupFromHash, pollFail, publicFetch, withoutKey } from './panelkeyrules.js';
import { exchangeSetup, storedKey, forgetKey, askPublic, askPublicAtOpen, keepLinkKey, followAssign } from './panelsetup.js';
import { buttonsBody } from './panelbuttons.js';
import { gridBody } from './gridpanelrun.js';
import { isGridPanel } from './gridpaneledges.js';
import { lookOf } from './panellook.js';
import { createArt } from './panelart.js';
import { createWake, enterFullscreen, featureNotes, wantsFullscreen } from './panelfeel.js';
import { watchPanel } from './panellive.js';
import { withEtag } from './etagfetch.js';
import { withRoutes, framesApply, sameRoutes } from './paneltally.js';
import { instanceFrom } from './panellink.js';
import { isPublic } from './panelsrules.js';
import { initChrome } from './panelfull.js';
import { initName } from './panelname.js';

const POLL_MS = 2000;
const NAMES_MS = 30000;
// While the WebSocket is up, its frames carry the tally, and the tally is read only this often, as a check. The 2 s
// poll of the small reads stays: it finds a revoked key or a lost Sode API, which the socket does not show.
const CHECK_MS = 60000;
// Frames that come close together (a Group or a Preset) are drawn as one.
const FRAME_MS = 30;
const ID_BYTES = 8;

const say = (n, text) => {
  n.textContent = text;
  n.hidden = !text;
};

// A random key per press: a retry of the same request replays its answer instead of a second take.
function takeKey() {
  const b = crypto.getRandomValues(new Uint8Array(ID_BYTES));
  return `panel-${Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('')}`;
}

// Returns { stop, show } at once; stop ends every timer, for an embedded run that goes away. show(patch) draws the
// panel with those fields changed (the editor's unsaved lists), and a patch with rev marks that saved copy as seen.
export function runPanel(seg, opts = {}) {
  // Before the panel is read, a show waits in early.
  const run = {
    stop: () => {},
    show: (patch) => (run.early = { ...run.early, ...patch }),
  };
  start(seg, opts, run);
  return run;
}

// page is the Page an embedded run opens on (the editor's focused Page); a page of its own opens on the kept one.
async function start(seg, { ui = pageUi(), call: given, embedded = !!given, page = '' } = {}, run) {
  const timers = [];
  let stopWatch = () => {};
  let offFrames = () => {};
  let ended = false; // the key is gone or refused, or an embedded run was stopped: only a new link or start helps
  let art = null;
  let body = null; // panelbuttons.js or gridpanelrun.js, by the panel's type
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
    offFrames();
    body?.stop();
    art?.revoke();
    unannounce();
  };
  // lock.js drops the page lock's PIN pad for a Public panel, never for the editor's live test.
  const tellLock = (on) => embedded || emit('publicPanel', { on });
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
  let id = seg; // the id after the panel is read: it is possible that the address holds its short name
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
  let linked = !!key;
  // A key that sits in this browser's storage, so an ended one is forgotten there.
  let stored = !key && !embedded;
  if (stored) key = storedKey(seg);
  // The page cannot read the mode without a key, so with none it asks whether the panel is Public before a session.
  let keyIsPublic = false;
  if (!key && !embedded) {
    key = await askPublicAtOpen(seg);
    if (ended) return;
    keyIsPublic = !!key;
    if (keyIsPublic) tellLock(true);
  }
  // Before the first read: a Session-mode page waits there for a sign-in. It is possible that the sign-in never comes.
  if (!key) announce(false);
  // Every call names its panel and instance for the Communication Log; the editor's live test is no instance.
  let instance = '';
  const tag = (init) => fromPanel(init, id, instance);
  const sendKeyed = (path, init, k) => fetch(path, withKey(tag(init), k));
  const keyFetch = (k, pub) =>
    guardFetch(pub ? publicFetch(k, { ask: () => askPublic(seg), send: sendKeyed }) : (path, init) => sendKeyed(path, init, k), {
      final: (s) => s === 401,
    });
  let keyed = key && keyFetch(key, keyIsPublic);
  const call = given || (key ? (path, init = {}) => keyed(path, init) : (path, init = {}) => sessionFetch(path, tag(init)));
  // Only the reads that carry an ETag are kept and sent with If-None-Match; a 304 gives the same body again. The
  // tally, protected and names replies are only read, so a 304 gives the same object too.
  const callJSON = jsonCall(withEtag(call, (p) => p === 'api/v1/tally' || p === 'api/v1/protected' || p === 'api/v1/names'));

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
  for (;;) {
    try {
      panel = await callJSON(`api/v1/panels/${seg}`);
      break;
    } catch (e) {
      if (e.status === undefined) throw e;
      // A 404 forgets too: a short name that moved to another panel must not keep sending the old panel's key.
      if ((e.status === 401 || e.status === 404) && stored && key) forgetKey(seg);
      // A stale key on a panel that is now Public: an old bookmark still opens it.
      const pub = e.status === 401 && key && !keyIsPublic && !embedded ? await askPublicAtOpen(seg) : '';
      if (ended) return;
      if (pub) {
        history.replaceState(null, '', `${location.pathname}${location.search}${withoutKey(location.hash)}`);
        // A Public key lives in memory only, never in this browser's storage.
        [key, keyIsPublic, stored, linked, keyed] = [pub, true, false, false, keyFetch(pub, true)];
        tellLock(true);
        continue;
      }
      if (e.status === 401) announce(endedCode(e.code));
      say(ui.note, e.status === 403 && e.code === 'network_not_allowed' ? e.message : openFailText(e.status, !!key));
      return;
    }
  }
  // Back to editing came before the panel did: nothing below is allowed to start a timer or a watch.
  if (ended) return;
  // Kept only after it worked, so a mistyped or ended key never replaces a stored one.
  if (linked) stored = keepLinkKey(seg, key);
  if (panel.acts === 'api' && !key && !embedded) {
    say(ui.note, 'This panel acts as its own API client. Open its link with the key.');
    return;
  }
  id = panel.id;
  tellLock(isPublic(panel));
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
      chrome.show(kioskOf(location.hash, p), look.titleBar, isGridPanel(p));
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

  let ready = false;
  // Each confirm sends one POST with one Idempotency-Key.
  const post = (path, sent) =>
    callJSON(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Idempotency-Key': takeKey() },
      body: JSON.stringify(sent),
      noWait: true,
    });
  // The adapter a body gets from the run. A panel's type never changes, so a live update keeps its body.
  const ctx = {
    ui,
    art,
    // The editor's live test gives a Grid panel the page's header parts (panelmount.js liveUi), so the grid runs as
    // on its own page. The run stays embedded all the same: no key, no instance, no keepalive.
    embedded: embedded && !(isGridPanel(panel) && ui.name),
    page,
    id,
    call,
    callJSON,
    post,
    poll: () => poll(),
    ready: () => ready,
    instance: () => instance,
    keyed: () => !!key,
    touched() {
      wake?.retry();
      if (wantsFullscreen(look) && !embedded) enterFullscreen(document, look.landscape);
    },
  };
  body = (isGridPanel(panel) ? gridBody : buttonsBody)(ctx, panel);

  // Blur the whole panel and say Offline, so nobody mistakes a stale grid for a working one.
  const showOffline = (o) => {
    ui.main.classList.toggle('sode-panels--offline', !!o);
    ui.offline.hidden = !o;
    if (o) ui.offlineDetail.textContent = o.detail;
  };

  let namesAt = 0;
  let polling = false;
  // A take's answer asks for a poll; one already in flight can carry the tally from before the take.
  let again = false;
  // live: the WebSocket sent a hello since it last closed, so its tally frames hold every change since then. A check
  // that finds the kept tally wrong stops trusting frames until the next hello.
  let live = S.wsState === 'up' && !!S.cfg;
  let trusted = true;
  let rights = panel.rights;
  let shown = null; // the tally the body has: the last read, with every frame since put in.
  let stale = false; // after a hello, frames from before it can have gaps, so the next poll reads the tally.
  let helloCount = 0;
  let late = null; // frames that came while a tally read was in flight, put into its answer
  let rows = []; // frames waiting for their draw
  let rowsTimer = 0;
  let prot = null;
  let link = null;
  let checkedAt = 0;
  async function poll() {
    if (ended) return;
    if (polling) {
      again = true;
      return;
    }
    polling = true;
    again = false;
    const check = Date.now() - checkedAt >= CHECK_MS;
    const readTally = !live || !trusted || !shown || stale || check || !framesApply(rights);
    late = readTally ? [] : null;
    const hellos = helloCount;
    try {
      const wantNames = Date.now() - namesAt > NAMES_MS;
      // A key without the presets scope gets 403: its Preset cells then show as unavailable.
      const [tally, p, nm, pr, l] = await Promise.all([
        readTally ? callJSON('api/v1/tally') : null,
        callJSON('api/v1/protected'),
        wantNames ? callJSON('api/v1/names') : null,
        wantNames ? callJSON('api/v1/presets').catch(() => ({})) : null,
        // A key without the status scope gets 403: the router link is then unknown, not down.
        callJSON('api/v1/status').catch((e) => {
          if (e.status === undefined || e.status >= 500) throw e;
          return null;
        }),
      ]);
      if (nm) namesAt = Date.now();
      [prot, link] = [p, l];
      if (tally) {
        const read = late.length ? withRoutes(tally, late) : tally;
        // Frames still waiting for their draw are part of what the page holds.
        const kept = shown && rows.length ? withRoutes(shown, rows) : shown;
        if (live && kept && !stale && trusted && !sameRoutes(kept, read)) trusted = false;
        shown = read;
        checkedAt = Date.now();
        if (hellos === helloCount) stale = false;
      }
      const off = offlineOf({ failed: false, routerUp: link ? link.routerUp : true });
      showOffline(off);
      body.data({ tally: shown, prot, nm, pr, link, first: !ready && !off });
      ready = !off;
      say(ui.note, '');
    } catch (e) {
      const f = pollFail(e.status, e.code, e.status === undefined ? undefined : e.message || undefined, !!key && !keyIsPublic);
      say(ui.note, f.text);
      if (!f.stop) {
        ready = false;
        // A refusal (a session not signed in yet) is not a lost link.
        if (e.status === undefined || e.status >= 500) showOffline(offlineOf({ failed: true, routerUp: true }));
      }
      if (f.stop) {
        ended = true;
        ready = false;
        tellLock(false);
        if (stored && f.forget) forgetKey(seg);
        // Nothing is allowed to call Sode again: not the polls, the update check or the keepalive.
        timers.forEach(clearInterval);
        stopWatch();
        offFrames();
        body.stop();
        keep?.stop();
        // Only the keyless announcement, which carries no key.
        if (e.status === 401) announce(endedCode(e.code));
      }
    }
    polling = false;
    late = null;
    body.render();
    if (again) poll();
  }

  // A tally read under Hidden leaves routes out, so frames on it after the Rights widen would never bring them back.
  const setRights = (r) => {
    if (framesApply(r) !== framesApply(rights)) stale = true;
    rights = r;
  };
  const putRows = () => {
    rowsTimer = 0;
    const u = rows;
    rows = [];
    if (ended || !shown || !u.length) return;
    shown = withRoutes(shown, u);
    body.data({ tally: shown, prot, nm: null, pr: null, link, first: false });
    body.render();
  };
  const offs = [
    on('tallyRows', (u) => {
      if (ended) return;
      late?.push(...u);
      if (!framesApply(rights)) return poll();
      if (!live || !trusted) return;
      if (!shown) return late || poll();
      rows.push(...u);
      rowsTimer ||= setTimeout(putRows, FRAME_MS);
    }),
    // A hello after a (re)connect: frames from now on are whole only from a full read made after it.
    on('hello', () => {
      live = true;
      trusted = true;
      stale = true;
      helloCount++;
      poll();
    }),
    on('wsclose', () => {
      live = false;
    }),
    on('protectedRows', () => poll()),
    on('names', () => {
      namesAt = 0;
      poll();
    }),
  ];
  let router = S.router;
  offs.push(
    on('conn', () => {
      if (S.router === router) return;
      router = S.router;
      if (live) poll();
    }),
  );
  offFrames = () => {
    offs.forEach((off) => off());
    clearTimeout(rowsTimer);
  };

  let keep = null;
  if (!embedded) {
    // The page runs with a key now and is an instance, so it is no longer listed as keyless.
    unannounce();
    const showName = initName(ui.name, (name) => keep.rename(name));
    const ask = panel.settings?.askInstanceName !== false;
    keep = startKeepalive(
      call,
      id,
      ui.device,
      instanceFrom(location.search),
      (name, editable) => {
        const renamed = name !== instance;
        instance = name;
        if (ask) showName(name, editable);
        body.renamed(renamed);
      },
      (msg) => {
        run.stop();
        followAssign(msg);
      },
      ask,
    );
  }
  // A newer saved copy. The watcher counts down and waits out staged routes first.
  const watch = watchPanel({
    ui,
    id,
    panel,
    // Every read carries the caller's Rights and the confirm settings, which a layout update waiting on staged
    // routes must not hold back.
    get: async (path) => {
      const p = await callJSON(path);
      if (p) setRights(p.rights);
      if (p) body.read?.(p);
      return p;
    },
    state: () => body.pending(),
    apply(p) {
      panel = p;
      setRights(p.rights);
      tellLock(isPublic(p));
      look = applyLook(p);
      body.apply(p);
    },
  });
  stopWatch = watch;
  run.show = (patch) => {
    if (ended) return;
    panel = { ...panel, ...patch };
    body.apply(panel);
    if (patch.rev !== undefined) watch.seen(panel);
  };
  if (run.early) run.show(run.early);
  // Results and errors fade on their own clocks, so the body is redrawn every second.
  timers.push(
    setInterval(poll, POLL_MS),
    setInterval(() => body.render(), 1000),
  );
  poll();
}
