// The artwork control of the Cell tab. Icon A is `art` (a vendored icon or an upload), Icon B is `artB` (a digit or
// letter glyph beside an icon or a corner badge on an image, panellook.js ART_B_CHOICES). An upload is resized and
// checked before sending (artcompress.js prepareArt); the server checks again by content.
import { el, icon } from './core.js';
import { sessionFetch, jsonCall } from './sessionkey.js';
import { btn } from './formbits.js';
import { artFilter, ART_ICONS } from './sidebarrules.js';
import { artOf, artBOf, ART_B_CHOICES } from './panellook.js';
import { artNode } from './panelview.js';
import { prepareArt } from './artcompress.js';

const send = jsonCall(sessionFetch, { down: 'Sode did not answer, so the image was not uploaded.', failed: (s) => `The image was not uploaded (${s}).` });

async function upload(panelId, file, sent) {
  const body = await send(`api/v1/panels/${panelId}/artwork`, { method: 'POST', headers: { 'Content-Type': sent.type || file.type }, body: sent });
  return body?.id;
}

// A pane of the picker: a grid of choices, and a None button for the column header. pick(value) gets undefined for None.
function pane(name, choices, draw, pick) {
  const grid = el('div', 'sode-side__icons');
  grid.setAttribute('role', 'group');
  grid.setAttribute('aria-label', name);
  const none = btn('pb-btn--ghost pb-btn--sm', 'None', () => pick(undefined));
  for (const c of choices) {
    const b = btn('pb-btn--ghost pb-btn--icon pb-btn--sm', '', () => pick(c));
    b.setAttribute('aria-label', c);
    b.dataset.pbTip = c;
    b.append(draw(c));
    grid.append(b);
  }
  return { grid, none };
}

// opts: {ed, icons, set(value), setB(glyph), setHideText(on)}. update(value, caption, hideText, artB) repaints; the
// preview shows the artwork row dimmed behind the caption. Hide text shows only once the cell has Icon A or B.
export function createArtControl({ ed, icons = false, set, setB, setHideText }) {
  const preview = el('div', 'sode-side__art');
  const note = el('p', 'sode__hint');
  note.setAttribute('role', 'status');
  const file = el('input');
  file.type = 'file';
  file.accept = 'image/jpeg,image/png,image/gif,image/svg+xml';
  file.hidden = true;
  const up = btn('pb-btn--outline pb-btn--sm', 'Upload image', () => file.click());
  // An upload is `art` without the icon: prefix (artOf tells them apart); Icon A and B are the other kind of artwork.
  const clearImage = btn('pb-btn--outline pb-btn--sm', 'Remove Image', () => set(undefined));
  const clearIcon = btn('pb-btn--outline pb-btn--sm', 'Remove Icon', () => {
    if (!artOf({ art: shownValue })?.id) set(undefined);
    setB?.(undefined);
  });
  const picker = el('div', 'sode-side__picker');
  picker.hidden = true;
  const choose = btn('pb-btn--outline pb-btn--sm', 'Choose icon', () => {
    picker.hidden = !picker.hidden;
    choose.setAttribute('aria-expanded', String(!picker.hidden));
  });
  choose.setAttribute('aria-expanded', 'false');

  // Two columns shown at once, each with its own None and its own scroll: Icon A is the sprite, Icon B the glyphs.
  // Choosing keeps the picker open, so "[camera] 2" is two clicks.
  const column = (text, p, extra) =>
    el('div', 'sode-side__col', el('div', 'sode-side__col-top', el('span', 'sode-side__col-h', text), ...(extra ? [extra] : []), p.none), p.grid);
  let shownValue = '';
  const panes = [
    {
      id: 'a',
      ...pane(
        'Icon A',
        ART_ICONS,
        (n) => icon(n),
        (v) => set(v === undefined ? undefined : `icon:${v}`),
      ),
    },
    ...(setB
      ? [
          {
            id: 'b',
            ...pane(
              'Icon B',
              ART_B_CHOICES,
              (g) => el('span', 'sode-side__glyph', g),
              (v) => setB(v),
            ),
          },
        ]
      : []),
  ];
  const find = el('input', 'pb-input sode-side__find');
  find.type = 'search';
  find.placeholder = 'Search icons';
  find.autocomplete = 'off';
  find.setAttribute('aria-label', 'Search icons by name');
  find.addEventListener('input', () => {
    const shown = new Set(artFilter(ART_ICONS, find.value));
    for (const b of panes[0].grid.querySelectorAll('[aria-label]')) b.hidden = !shown.has(b.getAttribute('aria-label'));
  });
  for (const p of panes) p.col = column(p.id === 'a' ? 'Icon A' : 'Icon B', p, p.id === 'a' ? find : null);
  picker.append(...panes.map((p) => p.col));

  let hidden = false;
  const hide = btn('pb-btn--outline pb-btn--sm', 'Hide text', () => setHideText(!hidden));
  hide.hidden = true;
  const bar = el('div', 'sode-side__row', up, ...(icons ? [choose] : []), clearImage, clearIcon, ...(setHideText ? [hide] : []));
  const node = el('div', 'sode__field', el('span', 'sode__field-label', 'Artwork'), preview, bar, ...(icons ? [picker] : []), note, file);

  file.addEventListener('change', async () => {
    const f = file.files[0];
    file.value = '';
    if (!f) return;
    note.textContent = 'Uploading.';
    try {
      const { blob, problem } = await prepareArt(f);
      if (problem) {
        note.textContent = problem;
        return;
      }
      const id = await upload(ed.panel().id, f, blob);
      note.textContent = '';
      set(id);
    } catch (e) {
      note.textContent = e.message;
    }
  });

  let shown = '';
  return {
    node,
    // drawn: the cell shows artwork, its own or its item's default, so Hide text applies.
    update(value, caption = '', hideText = false, artB = '', drawn = false) {
      const ro = ed.readOnly();
      const saved = !!ed.panel();
      const shownB = artBOf({ art: value, artB });
      const any = !!value || !!shownB;
      up.disabled = ro || !saved;
      choose.disabled = ro;
      shownValue = value;
      const upload = !!artOf({ art: value })?.id;
      clearImage.disabled = ro || !upload;
      clearIcon.disabled = ro || !(shownB || (value && !upload));
      hidden = hideText;
      hide.hidden = !any && !drawn;
      hide.disabled = ro;
      hide.textContent = hideText ? 'Show text' : 'Hide text';
      for (const p of panes) for (const b of [p.none, ...p.grid.querySelectorAll('button')]) b.disabled = ro;
      for (const b of panes[0].grid.querySelectorAll('[aria-label]')) b.setAttribute('aria-pressed', String(value === `icon:${b.getAttribute('aria-label')}`));
      for (const b of panes[1]?.grid.querySelectorAll('[aria-label]') || []) b.setAttribute('aria-pressed', String(artB === b.getAttribute('aria-label')));
      if (!saved && !note.textContent) note.textContent = 'Save the panel before uploading an image.';
      else if (saved && note.textContent === 'Save the panel before uploading an image.') note.textContent = '';
      const key = `${value}|${shownB}|${caption}|${hideText}`;
      if (shown === key) return;
      shown = key;
      preview.replaceChildren();
      preview.hidden = !any;
      preview.toggleAttribute('data-hide-text', hideText); // full strength, as the cell (panels.css)
      if (!any) return;
      const row = artNode({ art: value, artB: shownB }, ed.art());
      if (!row) return;
      preview.append(row);
      if (!hideText) preview.append(el('span', 'sode-side__art-text', caption || (artOf({ art: value })?.icon || shownB ? 'Caption' : '')));
    },
  };
}
