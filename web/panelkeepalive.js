// An open panel page is an instance of the panel (docs/panels-design.md, Monitoring). The device name is asked
// once per browser, then a keepalive goes out every 15 seconds so Sode knows the instance is alive. A held stream
// tells Sode at once when the page closes; the keepalive is the fallback when a proxy or browser will not hold it.
import { readStr, writeStr } from './store.js';
import { KEEPALIVE_MS, deviceFrom, keepaliveBody, streamPath, retryMs, renameTo } from './panelwatchrules.js';
import { splitLines, assignOf, keylessPath } from './panelassignrules.js';
import { instanceFrom } from './panellink.js';

export const DEVICE_KEY = 'sode-panel-device';

// fetch with the Authorization header can stream where EventSource cannot; the server writes a line every 10 s,
// and an Assign as its last. Ends when gate.stop aborts it (a rename opens a new hold under the new name).
// open(signal) answers the fetch.
async function hold(open, gate, onAssign) {
  for (let tries = 0; !gate.ctl.signal.aborted; tries++) {
    try {
      const r = await open(gate.ctl.signal);
      if (r.ok && r.body) {
        tries = 0;
        const rd = r.body.pipeThrough(new TextDecoderStream()).getReader();
        let rest = '';
        for (let got = await rd.read(); !got.done; got = await rd.read()) {
          const split = splitLines(rest, got.value);
          rest = split.rest;
          for (const line of split.lines) {
            const msg = assignOf(line);
            if (msg) return onAssign(msg);
          }
        }
      }
    } catch {}
    await new Promise((ok) => setTimeout(ok, retryMs(tries)));
  }
}

// A page with no working key (GLOSSARY.md, Keyless instance) holds the keyless announcement under the name it already
// has, and never asks for one. No key rides it; ended says the page's key was ended. Returns { stop() }.
export function announceKeyless(seg, ended, onAssign) {
  const gate = { ctl: new AbortController() };
  const device = deviceFrom(instanceFrom(location.search)) || deviceFrom(readStr(DEVICE_KEY));
  hold((signal) => fetch(keylessPath(seg, device, ended), { signal, cache: 'no-store' }), gate, (msg) => {
    gate.ctl.abort();
    onAssign(msg);
  });
  return { stop: () => gate.ctl.abort() };
}

// call(path, init) is the page's keyed fetch; form holds the name input and its Save button. preset is the link's
// ?instance= name: it stands in for the stored name and the question, and is never stored, so the link stays the authority.
// onName(device, editable) hears the name once it is known and after a rename. onAssign(msg) hears an Assign, after
// which nothing is held or sent. Returns { rename(name), stop() }; a preset name cannot be renamed. The old name's
// instance goes quiet and is lost after the usual grace, as any closed page is.
export function startKeepalive(call, id, form, preset = '', onName = () => {}, onAssign = () => {}) {
  let device = deviceFrom(preset) || deviceFrom(readStr(DEVICE_KEY));
  let timer = 0;
  let gate = null;
  const beat = () =>
    call(`api/v1/panels/${id}/instances`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: keepaliveBody(device) }).catch(() => {});
  let stopped = false;
  const run = () => {
    if (stopped) return;
    clearInterval(timer);
    gate?.ctl.abort();
    gate = { ctl: new AbortController() };
    hold((signal) => call(streamPath(id, device), { signal }), gate, (msg) => {
      stop();
      onAssign(msg);
    });
    beat();
    timer = setInterval(beat, KEEPALIVE_MS);
    onName(device, !preset);
  };
  const rename = (input) => {
    const next = preset ? '' : renameTo(device, input);
    if (!next) return false;
    device = next;
    writeStr(DEVICE_KEY, device);
    run();
    return true;
  };
  // The page's key ended: no more beats or holds.
  const stop = () => {
    clearInterval(timer);
    gate?.ctl.abort();
    stopped = true;
  };
  const api = { rename, stop };
  if (device) {
    run();
    return api;
  }
  const input = form.querySelector('input');
  const save = () => {
    device = deviceFrom(input.value);
    if (!device) return input.focus();
    writeStr(DEVICE_KEY, device);
    form.hidden = true;
    run();
  };
  form.hidden = false;
  form.querySelector('button').addEventListener('click', save);
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') save();
  });
  return api;
}
