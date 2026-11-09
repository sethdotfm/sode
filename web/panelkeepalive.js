// An open panel page is an instance of the panel (docs/panels-design.md, Monitoring). The device name is asked
// once per browser, then a keepalive goes out every 15 seconds so Sode knows the screen is alive.
import { readStr, writeStr } from './store.js';
import { KEEPALIVE_MS, deviceFrom, keepaliveBody } from './panelwatchrules.js';

export const DEVICE_KEY = 'sode-panel-device';

// call(path, init) is the page's keyed fetch; form holds the name input and its Save button. preset is the link's
// ?instance= name: it stands in for the stored name and the question, and is never stored, so the link stays the authority.
export function startKeepalive(call, id, form, preset = '') {
  let device = deviceFrom(preset) || deviceFrom(readStr(DEVICE_KEY));
  let timer = 0;
  const beat = () =>
    call(`api/v1/panels/${id}/instances`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: keepaliveBody(device) }).catch(() => {});
  const run = () => {
    clearInterval(timer);
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
