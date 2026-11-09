// An open panel page is an instance of the panel (docs/panels-design.md, Monitoring). The device name is asked
// once per browser, then a keepalive goes out every 15 seconds so Sode knows the screen is alive. A held stream
// tells Sode at once when the page closes; the keepalive is the fallback when a proxy or browser will not hold it.
import { readStr, writeStr } from './store.js';
import { KEEPALIVE_MS, deviceFrom, keepaliveBody, streamPath, retryMs } from './panelwatchrules.js';

export const DEVICE_KEY = 'sode-panel-device';

// fetch with the Authorization header can stream where EventSource cannot; the server writes a line every 10 s.
async function hold(call, id, device) {
  for (let tries = 0; ; tries++) {
    try {
      const r = await call(streamPath(id, device));
      if (r.ok && r.body) {
        tries = 0;
        const rd = r.body.getReader();
        while (!(await rd.read()).done);
      }
    } catch {}
    await new Promise((ok) => setTimeout(ok, retryMs(tries)));
  }
}

// call(path, init) is the page's keyed fetch; form holds the name input and its Save button.
export function startKeepalive(call, id, form) {
  let device = deviceFrom(readStr(DEVICE_KEY));
  let timer = 0;
  let held = null;
  const beat = () =>
    call(`api/v1/panels/${id}/instances`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: keepaliveBody(device) }).catch(() => {});
  const run = () => {
    clearInterval(timer);
    if (!held) held = hold(call, id, device);
    beat();
    timer = setInterval(beat, KEEPALIVE_MS);
  };
  if (device) return run();
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
}
