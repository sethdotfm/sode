// The artwork control shared by the Cell tab (a vendored icon or an upload) and the Panel tab (a backdrop, upload
// only). Stored values are an artwork id, or 'icon:<name>' for a cell. Type and size are checked before sending
// (sidebarrules.js); the server checks again by content.
import { el, icon } from './core.js';
import { sessionFetch, jsonCall } from './sessionkey.js';
import { btn } from './formbits.js';
import { artProblem, ART_ICONS } from './sidebarrules.js';
import { artOf } from './panellook.js';
import { artNode } from './panelview.js';
import { compressImage } from './artcompress.js';

const send = jsonCall(sessionFetch, { down: 'Sode did not answer, so the image was not uploaded.', failed: (s) => `The image was not uploaded (${s}).` });

async function upload(panelId, file) {
  const sent = await compressImage(file);
  const body = await send(`api/v1/panels/${panelId}/artwork`, { method: 'POST', headers: { 'Content-Type': sent.type || file.type }, body: sent });
  return body?.id;
}

// opts: {ed, icons, set(value), setHideText(on)}. update(value, caption, hideText) repaints; the preview shows the artwork dimmed behind the caption.
// Hide text shows only for a cell that has artwork.
export function createArtControl({ ed, icons = false, set, setHideText }) {
  const preview = el('div', 'sode-side__art');
  const note = el('p', 'sode__hint');
  note.setAttribute('role', 'status');
  const file = el('input');
  file.type = 'file';
  file.accept = 'image/jpeg,image/png,image/gif,image/svg+xml';
  file.hidden = true;
  const up = btn('pb-btn--outline pb-btn--sm', 'Upload image', () => file.click());
  const clear = btn('pb-btn--outline pb-btn--sm', 'Remove', () => set(undefined));
  const choose = btn('pb-btn--outline pb-btn--sm', 'Choose icon', () => {
    grid.hidden = !grid.hidden;
    choose.setAttribute('aria-expanded', String(!grid.hidden));
  });
  choose.setAttribute('aria-expanded', 'false');
  const grid = el('div', 'sode-side__icons');
  grid.hidden = true;
  for (const name of ART_ICONS) {
    const b = btn('pb-btn--ghost pb-btn--icon pb-btn--sm', '', () => {
      set(`icon:${name}`);
      grid.hidden = true;
      choose.setAttribute('aria-expanded', 'false');
    });
    b.setAttribute('aria-label', name);
    b.dataset.pbTip = name;
    b.append(icon(name));
    grid.append(b);
  }
  let hidden = false;
  const hide = btn('pb-btn--outline pb-btn--sm', 'Hide text', () => setHideText(!hidden));
  hide.hidden = true;
  const bar = el('div', 'sode-side__row', up, ...(icons ? [choose] : []), clear, ...(setHideText ? [hide] : []));
  const node = el(
    'div',
    'sode__field',
    el('span', 'sode__field-label', icons ? 'Artwork' : 'Backdrop image'),
    preview,
    bar,
    ...(icons ? [grid] : []),
    note,
    file,
  );

  file.addEventListener('change', async () => {
    const f = file.files[0];
    file.value = '';
    if (!f) return;
    note.textContent = artProblem(f);
    if (note.textContent) return;
    note.textContent = 'Uploading.';
    try {
      const id = await upload(ed.panel().id, f);
      note.textContent = '';
      set(id);
    } catch (e) {
      note.textContent = e.message;
    }
  });

  let shown = '';
  return {
    node,
    update(value, caption = '', hideText = false) {
      const ro = ed.readOnly();
      const saved = !!ed.panel();
      up.disabled = ro || !saved;
      choose.disabled = ro;
      clear.disabled = ro || !value;
      hidden = hideText;
      hide.hidden = !value;
      hide.disabled = ro;
      hide.textContent = hideText ? 'Show text' : 'Hide text';
      if (!saved && !note.textContent) note.textContent = 'Save the panel before uploading an image.';
      else if (saved && note.textContent === 'Save the panel before uploading an image.') note.textContent = '';
      if (shown === `${value}|${caption}|${hideText}`) return;
      shown = `${value}|${caption}|${hideText}`;
      preview.replaceChildren();
      preview.hidden = !value;
      if (!value) return;
      // The backdrop is a plain artwork id, a cell's art may also be an icon name.
      const node = artNode({ art: value }, ed.art());
      if (!node) return;
      preview.append(node);
      if (!hideText) preview.append(el('span', 'sode-side__art-text', caption || (artOf({ art: value })?.icon ? 'Caption' : '')));
    },
  };
}
