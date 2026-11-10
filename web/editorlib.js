// The Choose icon picker's Uploaded section: the Icon library's icons and its Upload. Any editor picks; adding needs a
// Trusted user or the Advanced Settings PIN, asked for after the first refusal as Default artwork does. Tiles draw
// through the panel's createArt, so their blob URLs go when the panel does.
import { el } from './core.js';
import { btn } from './formbits.js';
import { sessionFetch, jsonCall } from './sessionkey.js';
import { pinNeeded } from './templaterules.js';
import { prepareArt } from './artcompress.js';
import { artImage } from './panelview.js';
import { libNameOf, libNameProblem, pickerSections, LIB_TYPES } from './iconlibrules.js';

const send = jsonCall(sessionFetch, { down: 'Sode did not answer, so the icon was not uploaded.', failed: (s) => `The icon was not uploaded (${s}).` });
const read = jsonCall(sessionFetch, { down: 'Sode did not answer, so the Icon library was not read.', failed: (s) => `The Icon library was not read (${s}).` });

// The body is JSON so the PIN can ride in it; readAsDataURL, since spreading 2 MB into btoa throws.
const base64 = (blob) =>
  new Promise((done, fail) => {
    const r = new FileReader();
    r.onload = () => done(String(r.result).slice(String(r.result).indexOf(',') + 1));
    r.onerror = () => fail(new Error('The file could not be read.'));
    r.readAsDataURL(blob);
  });

// set(value) picks for the cell. update(value) repaints; filter(text) applies the picker's search.
export function createLibSection({ ed, set }) {
  let icons = [];
  let text = '';
  let value = '';
  let drawnWith; // the createArt the tiles were drawn with
  const grid = el('div', 'sode-side__icons sode-side__lib');
  grid.id = 'ed-lib-icons';
  grid.setAttribute('role', 'group');
  grid.setAttribute('aria-label', 'Uploaded icons');
  const empty = el('p', 'sode__hint', 'No uploaded icons yet.');
  const note = el('p', 'sode__hint');
  note.setAttribute('role', 'status');
  const name = el('input', 'pb-input sode-side__find');
  name.id = 'ed-lib-name';
  name.autocomplete = 'off';
  name.placeholder = 'Icon name';
  name.setAttribute('aria-label', 'Name for the uploaded icon');
  const file = el('input');
  file.type = 'file';
  file.accept = LIB_TYPES.join(',');
  file.hidden = true;
  const up = btn('pb-btn--outline pb-btn--sm', 'Upload', () => file.click());
  up.id = 'ed-lib-upload';
  const pin = el('input', 'pb-input');
  pin.type = 'password';
  pin.autocomplete = 'off';
  pin.id = 'ed-lib-pin';
  pin.setAttribute('aria-label', 'Advanced Settings PIN');
  const pinRow = el('label', 'sode-side__pin', 'Adding to the Icon library needs a Trusted user or the Advanced Settings PIN', pin);
  pinRow.hidden = true;
  const node = el(
    'div',
    'sode-side__lib-sec',
    el('span', 'sode-side__col-h', 'Uploaded'),
    grid,
    empty,
    el('div', 'sode-side__row', name, up),
    pinRow,
    note,
    file,
  );

  function paint() {
    const art = ed.art();
    drawnWith = art;
    grid.replaceChildren();
    const tiles = pickerSections(icons, [], text)[0].tiles;
    for (const t of tiles) {
      const b = btn('pb-btn--ghost pb-btn--icon pb-btn--sm', '', () => set(t.value));
      b.dataset.lib = t.id;
      b.dataset.pbTip = t.name;
      b.setAttribute('aria-label', t.name);
      b.setAttribute('aria-pressed', String(value === t.value));
      b.disabled = ed.readOnly();
      b.append(art ? artImage('sode-side__lib-img', art, t.value, true) : el('span', 'sode-side__glyph', [...t.name][0]));
      grid.append(b);
    }
    grid.hidden = !tiles.length;
    empty.hidden = !!tiles.length;
    empty.textContent = icons.length ? 'No uploaded icon matches.' : 'No uploaded icons yet.';
  }

  async function load() {
    try {
      icons = (await read('api/v1/icon-library'))?.icons || [];
    } catch (e) {
      note.textContent = e.message;
    }
    paint();
  }

  file.addEventListener('change', async () => {
    const f = file.files[0];
    file.value = '';
    if (!f) return;
    if (!LIB_TYPES.includes(f.type)) {
      note.textContent = 'A library icon is a PNG or SVG image.';
      return;
    }
    const n = name.value.trim() || libNameOf(f.name);
    const problem = libNameProblem(n);
    if (problem) {
      note.textContent = problem;
      return;
    }
    note.textContent = 'Uploading.';
    try {
      const { blob, problem: tooBig } = await prepareArt(f);
      if (tooBig) {
        note.textContent = tooBig;
        return;
      }
      const body = { name: n, data: await base64(blob) };
      if (pin.value) body.pin = pin.value;
      const ic = await send('api/v1/icon-library', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
      pin.value = '';
      pinRow.hidden = true;
      name.value = '';
      note.textContent = ic.name === n ? `Added ${ic.name}.` : `Added as ${ic.name}, since ${n} is taken.`;
      await load();
      set(`lib:${ic.id}`);
    } catch (e) {
      if (pinNeeded(e.status, e.code)) {
        pinRow.hidden = false;
        pin.focus();
      }
      note.textContent = e.message;
    }
  });

  return {
    node,
    load,
    filter(t) {
      text = t;
      paint();
    },
    update(v) {
      const ro = ed.readOnly();
      up.disabled = ro;
      name.disabled = ro;
      if (v === value && drawnWith === ed.art()) {
        for (const b of grid.children) b.disabled = ro;
        return;
      }
      value = v;
      paint();
    },
  };
}
