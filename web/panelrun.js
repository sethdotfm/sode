// Runs one panel: the page /panels/<id>, or embedded in the Live Editor with its own ui and call (no keepalive, no
// instance). The key is the user's session key, the link's key, or a Public panel's short-lived key (panelpublic.go).
import { emit, on, S, showNote } from './core.js';
import { sessionFetch, jsonCall, startSession, signInAgain, onSessionKey, nameSessionKey } from './sessionkey.js';
import { pageUi } from './panelmount.js';
import { paintOffline, waitForSode } from './panelwait.js';
import { startKeepalive, announceKeyless } from './panelkeepalive.js';
import { withKey, guardFetch } from './sessionkeyrules.js';
import { fromPanel } from './panelwatchrules.js';
import { keyFromHash, kioskOf, openFailText, sessionNote, offlineOf } from './panelrules.js';
import { setupFromHash, pollFail, publicFetch, withoutKey } from './panelkeyrules.js';
import { exchangeSetup, storedKey, forgetKey, askPublic, askPublicAtOpen, keepLinkKey, followAssign, markBack } from './panelsetup.js';
import { backWaitMs } from './panelbackrules.js';
import { showStopped } from './panelstopped.js';
import { buttonsBody } from './panelbuttons.js';
import { gridBody } from './gridpanelrun.js';
import { isGridPanel } from './gridpaneledges.js';
import { panelIn, placesUsed } from './panelitems.js';
import { makePlaces, mergeTally, mergeProtected, mergeNames, wireRoutes, undoIn, presetsIn } from './routerids.js';
import { lookOf } from './panellook.js';
import { pref } from './device.js';
import { createArt } from './panelart.js';
import { createWake, enterFullscreen, featureNotes, wantsFullscreen } from './panelfeel.js';
import { watchPanel } from './panellive.js';
import { withEtag } from './etagfetch.js';
import * as F from './paneltally.js';
import { instanceFrom } from './panellink.js';
import { isPublic } from './panelsrules.js';
import { initChrome } from './panelfull.js';
import { initName } from './panelname.js';

// The small reads are polled while the WebSocket carries the tally. The poll finds a revoked key or a lost Sode API,
// which the socket does not show.
const POLL_MS = 2000;
const NAMES_MS = 30000;
// Frames that come close together (a Group or a Preset) are drawn as one.
const FRAME_MS = 30;
const ID_BYTES = 8;

// A random key per press: a retry of the same request replays its answer instead of a second take.
function takeKey() {
  const b = crypto.getRandomValues(new Uint8Array(ID_BYTES));
  return `panel-${Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('')}`;
}

const endedCode = (code) => code === 'key_revoked' || code === 'key_expired';

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

// The page's key and the calls made with it. d.ended: the key is gone or refused, or an embedded run was stopped;
// only a new link or start helps. While the page has no working key it is a Keyless instance: listed for an Assign,
// never sent anything else.
function makeDoor(seg, ui, embedded, given, run) {
  const d = { ended: false, id: seg, instance: '', key: null, keyIsPublic: false, stored: false, linked: false, keyless: null, backTimer: 0 };
  // The routers in the order of GET /routers, for item numbers (routerids.js). Read one time, before the panel.
  d.places = makePlaces();
  let placesRead = false;
  // lock.js drops the page lock's PIN pad for a Public panel, never for the editor's live test.
  d.tellLock = (on) => embedded || emit('publicPanel', { on });
  d.unannounce = () => {
    d.keyless?.stop();
    d.keyless = null;
  };
  // A public line: it is possible that the panel is Public again. After a random wait the page reloads and asks one
  // time at open, a marked ask. Refused, it is a keyless page again and waits for the next line.
  const comeBack = () => {
    d.backTimer ||= setTimeout(() => {
      markBack(seg);
      location.reload();
    }, backWaitMs());
  };
  const assigned = (msg) => {
    run.stop();
    followAssign(msg);
  };
  d.announce = (keyEnded) => {
    if (embedded || (d.keyless && d.keyless.keyEnded === keyEnded)) return;
    d.unannounce();
    d.keyless = { keyEnded, ...announceKeyless(seg, keyEnded, assigned, comeBack) };
  };
  d.assigned = assigned;
  // Every call names its panel and instance for the Communication Log; the editor's live test is no instance.
  const tag = (init) => fromPanel(init, d.id, d.instance);
  const sendKeyed = (path, init, k) => fetch(path, withKey(tag(init), k));
  const keyFetch = (k, pub) =>
    guardFetch(pub ? publicFetch(k, { ask: () => askPublic(seg), send: sendKeyed }) : (path, init) => sendKeyed(path, init, k), {
      final: (s) => s === 401,
    });
  let keyed = null;

  // Resolves the panel, or null when the page cannot open it (a note says why).
  d.open = async () => {
    let key = embedded ? null : keyFromHash(location.hash);
    const setup = embedded ? '' : setupFromHash(location.hash);
    if (!key && setup) {
      const got = await exchangeSetup(seg, setup);
      if (d.ended) return null;
      if (got.error) {
        d.announce(false);
        showNote(ui.note, got.error);
        return null;
      }
      key = got.key;
    }
    d.linked = !!key;
    // A key that sits in this browser's storage, so an ended one is forgotten there.
    d.stored = !key && !embedded;
    if (d.stored) key = storedKey(seg);
    // The page cannot read the mode without a key, so with none it asks whether the panel is Public before a session.
    if (!key && !embedded) {
      key = await askPublicAtOpen(seg);
      if (d.ended) return null;
      d.keyIsPublic = !!key;
      if (d.keyIsPublic) d.tellLock(true);
    }
    // Before the first read: a Session-mode page waits there for a sign-in. It is possible that the sign-in never comes.
    if (!key) d.announce(false);
    d.key = key;
    keyed = key && keyFetch(key, d.keyIsPublic);
    d.call = given || (key ? (path, init = {}) => keyed(path, init) : (path, init = {}) => sessionFetch(path, tag(init)));
    // Only the reads that carry an ETag are kept and sent with If-None-Match; a 304 gives the same body again. The
    // tally, protected and names replies are only read, so a 304 gives the same object too.
    d.callJSON = jsonCall(withEtag(d.call, (p) => /^api\/v1\/(tally|protected|names)(\?|$)/.test(p)));

    if (!key && !embedded) {
      ui.signin.addEventListener('click', signInAgain);
      onSessionKey((k, step, err) => {
        if (step === 'ended') d.announce(true);
        const n = sessionNote(step, err);
        ui.signin.hidden = !n.retry;
        if (k) showNote(ui.note, d.feature || '');
        else if (n.text) showNote(ui.note, n.text);
      });
      nameSessionKey(`Panel ${seg}`);
      startSession();
    }

    for (;;) {
      try {
        if (!placesRead) {
          const r = await d.callJSON('api/v1/routers').catch(() => null);
          if (r?.routers) [d.places, placesRead] = [makePlaces(r.routers), true];
        }
        const panel = panelIn(await d.callJSON(`api/v1/panels/${seg}`), d.places);
        // Back to editing came before the panel did: nothing after this is allowed to start a timer or a watch.
        if (d.ended) return null;
        // Kept only after it worked, so a mistyped or ended key never replaces a stored one.
        if (d.linked) d.stored = keepLinkKey(seg, d.key);
        if (panel.acts === 'api' && !d.key && !embedded) {
          showNote(ui.note, 'API panel: open its link with the key.');
          return null;
        }
        d.id = panel.id;
        return panel;
      } catch (e) {
        if (e.status === undefined) throw e;
        // A 404 forgets too: a short name that moved to another panel must not keep sending the old panel's key.
        if ((e.status === 401 || e.status === 404) && d.stored && d.key) forgetKey(seg);
        // A stale key on a panel that is now Public: an old bookmark still opens it.
        const pub = e.status === 401 && d.key && !d.keyIsPublic && !embedded ? await askPublicAtOpen(seg) : '';
        if (d.ended) return null;
        if (pub) {
          history.replaceState(null, '', `${location.pathname}${location.search}${withoutKey(location.hash)}`);
          // A Public key lives in memory only, never in this browser's storage.
          Object.assign(d, { key: pub, keyIsPublic: true, stored: false, linked: false });
          keyed = keyFetch(pub, true);
          d.tellLock(true);
          continue;
        }
        if (e.status === 401) d.announce(endedCode(e.code));
        showNote(ui.note, e.status === 403 && e.code === 'network_not_allowed' ? e.message : openFailText(e.status, !!d.key));
        return null;
      }
    }
  };
  return d;
}

// Title, header, font scale and theme come from the stored panel, so a live update runs it again. Auto is the device
// setting as it is now, which the theme key (themekey.js) changes. A stored theme marks the page, and the key then
// does nothing.
function lookApplier(ui, embedded) {
  const root = document.documentElement;
  const chrome = embedded ? null : initChrome(ui);
  const setTheme = (t) => {
    // Embedded, the panel's tokens switch with color-scheme and the editor around it keeps its own theme.
    if (embedded) {
      ui.main.style.colorScheme = t === 'auto' ? '' : t;
      return;
    }
    root.toggleAttribute('data-sode-panel-theme', t !== 'auto');
    const want = t === 'auto' ? pref('theme') : t;
    if (want !== 'system') root.dataset.theme = want;
    else delete root.dataset.theme;
  };
  return (p) => {
    const look = lookOf(p.settings || {});
    if (!embedded) {
      chrome.show(kioskOf(location.hash, p), look.titleBar, isGridPanel(p));
      document.title = p.name;
      root.toggleAttribute('data-sode-landscape', look.landscape);
    }
    ui.title.textContent = p.name;
    ui.grid.style.setProperty('--panel-fs', look.scale);
    setTheme(look.theme);
    return look;
  };
}

// The tally, protected list, names and router link, kept current by the poll and the WebSocket's frames (the feed
// rules are paneltally.js). onFail(e) handles a failed poll; it returns true when the page stops. The frames carry the
// first router only, so the feed keeps the first router's tally. With two or more routers, each poll also reads the
// other routers the panel uses (used(), places). The body gets one answer of each kind on item numbers.
function tallyFeed({ door, body, ui, rights, used, showOffline, onFail }) {
  let feed = F.newFeed(S.wsState === 'up' && !!S.cfg, rights);
  let namesAt = 0;
  let polling = false;
  // A take's answer asks for a poll; one already in flight can carry the tally from before the take.
  let again = false;
  let ready = false;
  let prot = null;
  let link = null;
  let rowsTimer = 0;
  // The other routers' last answers: place -> { tally, prot, names }.
  let more = new Map();
  let firstNames = null;
  const q = (place) => (door.places.many ? `?router=${encodeURIComponent(door.places.uid(place))}` : '');
  const others = () => (door.places.many ? used().filter((p) => p > 0) : []);
  const parts = (k) => others().flatMap((place) => (more.get(place)?.[k] ? [{ place, reply: more.get(place)[k] }] : []));
  // The same answers give the same objects, so an unchanged poll skips the body's work (withEtag gives the same
  // objects on a 304).
  const memo = (fn) => {
    let was = [];
    let out = null;
    return (...args) => {
      const key = args.flatMap((a) => (Array.isArray(a) ? a.map((x) => x.reply) : [a]));
      if (key.length !== was.length || key.some((x, i) => x !== was[i])) [was, out] = [key, fn(...args)];
      return out;
    };
  };
  const tallyOf = memo(mergeTally);
  const protOf = memo(mergeProtected);
  const namesOf = memo((first, list) => mergeNames(first, list, door.places));
  // A router of the panel is up. Without the routers list (a Sode from before it), the first router's link counts.
  const anyUp = (l) => {
    if (!l?.routers?.length || !door.places.many) return l ? l.routerUp : true;
    return used().some((p) => l.routers.find((r) => r.uid === door.places.uid(p))?.routerUp);
  };
  const shownData = (nm) => ({ tally: tallyOf(feed.shown, parts('tally')), prot: protOf(prot, parts('prot')), nm });
  async function poll() {
    if (door.ended) return;
    if (polling) {
      again = true;
      return;
    }
    polling = true;
    again = false;
    const readTally = F.wantsRead(feed, Date.now());
    feed = F.readStarts(feed, readTally);
    const hellos = feed.hellos;
    try {
      const wantNames = Date.now() - namesAt > NAMES_MS;
      const rest = others();
      const [tally, p, nm, pr, l, read] = await Promise.all([
        readTally ? door.callJSON(`api/v1/tally${q(0)}`) : null,
        door.callJSON(`api/v1/protected${q(0)}`),
        wantNames ? door.callJSON(`api/v1/names${q(0)}`) : null,
        // A key without the presets scope gets 403: its Preset cells then show as unavailable.
        wantNames
          ? door
              .callJSON('api/v1/presets')
              .then((r) => presetsIn(r, door.places))
              .catch(() => ({}))
          : null,
        // A key without the status scope gets 403: the router link is then unknown, not down.
        door.callJSON('api/v1/status').catch((e) => {
          if (e.status === undefined || e.status >= 500) throw e;
          return null;
        }),
        Promise.all(
          rest.map((place) =>
            Promise.all([
              door.callJSON(`api/v1/tally${q(place)}`),
              door.callJSON(`api/v1/protected${q(place)}`),
              wantNames || !more.get(place)?.names ? door.callJSON(`api/v1/names${q(place)}`) : null,
            ]),
          ),
        ),
      ]);
      const next = new Map();
      rest.forEach((place, i) => {
        const [t, pt, n] = read[i];
        next.set(place, { tally: t, prot: pt, names: n || more.get(place)?.names });
      });
      const renamed = !!nm || read.some((x) => !!x[2]);
      more = next;
      if (nm) [namesAt, firstNames] = [Date.now(), nm];
      [prot, link] = [p, l];
      if (tally) feed = F.readCame(feed, tally, hellos, Date.now());
      const off = offlineOf({ failed: false, routerUp: anyUp(link) });
      showOffline(off);
      body.data({ ...shownData(renamed && firstNames ? namesOf(firstNames, parts('names')) : null), pr, link, first: !ready && !off });
      ready = !off;
      showNote(ui.note, door.feature);
    } catch (e) {
      ready = false;
      if (!onFail(e) && (e.status === undefined || e.status >= 500)) showOffline(offlineOf({ failed: true, routerUp: true }));
    }
    polling = false;
    feed = F.readEnds(feed);
    body.render();
    if (again) poll();
  }
  const putRows = () => {
    rowsTimer = 0;
    const d = F.drawRows(feed);
    feed = d.feed;
    if (door.ended || !d.changed) return;
    body.data({ ...shownData(null), pr: null, link, first: false });
    body.render();
  };
  let router = S.router;
  const offs = [
    on('tallyRows', (u) => {
      if (door.ended) return;
      const r = F.framesCame(feed, u);
      feed = r.feed;
      if (r.act === 'poll') poll();
      else if (r.act === 'queue') rowsTimer ||= setTimeout(putRows, FRAME_MS);
    }),
    // A hello after a (re)connect: frames from now on are whole only from a full read made after it.
    on('hello', () => {
      feed = F.helloCame(feed);
      poll();
    }),
    on('wsclose', () => (feed = F.socketClosed(feed))),
    on('protectedRows', () => poll()),
    on('names', () => {
      namesAt = 0;
      poll();
    }),
    on('conn', () => {
      if (S.router === router) return;
      router = S.router;
      if (feed.live) poll();
    }),
  ];
  return {
    poll,
    ready: () => ready,
    rights: (r) => (feed = F.rightsCame(feed, r)),
    stop() {
      offs.forEach((off) => off());
      clearTimeout(rowsTimer);
    },
  };
}

// page is the Page an embedded run opens on (the editor's focused Page); a page of its own opens on the kept one.
async function start(seg, { ui = pageUi(), call: given, embedded = !!given, page = '' } = {}, run) {
  const door = makeDoor(seg, ui, embedded, given, run);
  const timers = [];
  let stopWatch = () => {};
  let art = null;
  let body = null; // panelbuttons.js or gridpanelrun.js, by the panel's type
  let feed = null;
  let wake = null;
  let keep = null;
  door.feature = ''; // the wake lock note, which a good poll keeps on screen.
  // Nothing calls Sode after this: no poll, update check or keepalive.
  const halt = () => {
    door.ended = true;
    timers.forEach(clearInterval);
    stopWatch();
    feed?.stop();
    body?.stop();
    keep?.stop();
  };
  run.stop = () => {
    halt();
    wake?.off();
    art?.revoke();
    door.unannounce();
    clearTimeout(door.backTimer);
  };

  if (!embedded) await waitForSode(ui, () => door.ended);
  let panel = await door.open();
  if (!panel) return;
  const { call, id } = door;
  const places = door.places;
  // The bodies work on item numbers (routerids.js). The Undo cell's answer comes in on them. A take goes out as router
  // references, and its results come back on the item numbers it was sent with.
  const callJSON = async (path, init) => {
    const r = await door.callJSON(path, init);
    return (init?.method || 'GET') === 'GET' && /^api\/v1\/panels\/[^/]+\/undo$/.test(path) && r ? { ...r, undo: undoIn(r.undo, places) } : r;
  };
  const postTake = async (path, sent, init) => {
    if (path !== 'api/v1/takes' || !Array.isArray(sent.routes)) return callJSON(path, { ...init, body: JSON.stringify(sent) });
    const r = await callJSON(path, { ...init, body: JSON.stringify({ ...sent, routes: wireRoutes(sent.routes, places) }) });
    if (!Array.isArray(r?.results)) return r;
    return { ...r, results: r.results.map((x, i) => (sent.routes[i] ? { ...x, src: sent.routes[i].src, dst: sent.routes[i].dst } : x)) };
  };
  door.tellLock(isPublic(panel));
  art = createArt(call, id);
  const applyLook = lookApplier(ui, embedded);
  let look = applyLook(panel);
  if (!embedded) {
    if (look.wake) {
      wake = createWake(navigator, document);
      wake.on();
    }
    door.feature = featureNotes(look, navigator, window.isSecureContext).join(' ');
    showNote(ui.note, door.feature);
  }

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
    places: () => places,
    // Each confirm sends one POST with one Idempotency-Key.
    post: (path, sent) =>
      postTake(path, sent, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Idempotency-Key': takeKey() },
        noWait: true,
      }),
    poll: () => feed?.poll(),
    ready: () => !!feed?.ready(),
    instance: () => door.instance,
    keyed: () => !!door.key,
    touched() {
      wake?.retry();
      if (wantsFullscreen(look) && !embedded) enterFullscreen(document, look.landscape);
    },
  };
  body = (isGridPanel(panel) ? gridBody : buttonsBody)(ctx, panel);

  feed = tallyFeed({
    door,
    body,
    ui,
    rights: panel.rights,
    used: () => placesUsed(panel, places),
    showOffline: (o) => paintOffline(ui, o),
    onFail(e) {
      const f = pollFail(e.status, e.code, e.status === undefined ? undefined : e.message || undefined, !!door.key && !door.keyIsPublic, door.keyIsPublic);
      showNote(ui.note, f.text);
      if (!f.stop) return false;
      halt();
      door.tellLock(false);
      if (door.stored && f.forget) forgetKey(seg);
      // Only the keyless announcement, which carries no key.
      if (e.status === 401) door.announce(endedCode(e.code));
      if (e.status === 401 && door.keyIsPublic) showStopped(ui);
      return true;
    },
  });

  if (!embedded) {
    // The page runs with a key now and is an instance, so it is no longer listed as keyless.
    door.unannounce();
    const showName = initName(ui.name, (name) => keep.rename(name));
    const ask = panel.settings?.askInstanceName !== false;
    const named = (name, editable) => {
      const renamed = name !== door.instance;
      door.instance = name;
      if (ask) showName(name, editable);
      body.renamed(renamed);
    };
    keep = startKeepalive(call, id, ui.device, instanceFrom(location.search), named, door.assigned, ask);
  }
  // A newer saved copy. The watcher counts down and waits out staged routes first.
  const watch = watchPanel({
    ui,
    id,
    panel,
    // Every read carries the caller's Rights and the confirm settings, which a layout update waiting on staged
    // routes must not hold back.
    get: async (path) => {
      const p = panelIn(await callJSON(path), places);
      if (p) feed.rights(p.rights);
      if (p) body.read?.(p);
      return p;
    },
    state: () => body.pending(),
    apply(p) {
      panel = p;
      feed.rights(p.rights);
      door.tellLock(isPublic(p));
      look = applyLook(p);
      body.apply(p);
    },
  });
  stopWatch = watch;
  run.show = (patch) => {
    if (door.ended) return;
    panel = panelIn({ ...panel, ...patch }, places);
    body.apply(panel);
    if (patch.rev !== undefined) watch.seen(panel);
  };
  if (run.early) run.show(run.early);
  // Results and errors fade on their own clocks, so the body is redrawn every second.
  timers.push(
    setInterval(feed.poll, POLL_MS),
    setInterval(() => body.render(), 1000),
  );
  feed.poll();
}
