// An open panel page is an instance of the panel (docs/panels-design.md, Monitoring). The device name is asked
// once per browser, then a keepalive goes out every 15 seconds so Sode knows the instance is alive. A held stream
// tells Sode at once when the page closes; the keepalive is the fallback when a proxy or browser will not hold it.
import { readStr, writeStr } from './store.js';
import { KEEPALIVE_MS, deviceFrom, keepaliveBody, streamPath, retryMs, renameTo } from './panelwatchrules.js';

export const DEVICE_KEY = 'sode-panel-device';

// fetch with the Authorization header can stream where EventSource cannot; the server writes a line every 10 s.
// Ends when gate.stop aborts it (a rename opens a new hold under the new name).
async function hold(call, id, device, gate) {
  for (let tries = 0; !gate.ctl.signal.aborted; tries++) {
    try {
      const r = await call(streamPath(id, device), { signal: gate.ctl.signal });
      if (r.ok && r.body) {
        tries = 0;
        const rd = r.body.getReader();
        while (!(await rd.read()).done);
      }
    } catch {}
    await new Promise((ok) => setTimeout(ok, retryMs(tries)));
  }
}

// call(path, init) is the page's keyed fetch; form holds the name input and its Save button. preset is the link's
// ?instance= name: it stands in for the stored name and the question, and is never stored, so the link stays the authority.
// onName(device, editable) hears the name once it is known and after a rename. Returns { rename(name) }; a preset
// name cannot be renamed. The old name's instance goes quiet and is lost after the usual grace, as any closed page is.
export function startKeepalive(call, id, form, preset = '', onName = () => {}) {
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
    hold(call, id, device, gate);
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
