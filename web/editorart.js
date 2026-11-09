// The artwork control of the Cell tab. Icon A is `art` (a vendored icon or an upload), Icon B is `artB` (a digit or
// letter glyph drawn beside it, panellook.js ART_B_CHOICES). Upload type and size are checked before sending
// (sidebarrules.js); the server checks again by content.
import { el, icon } from './core.js';
import { sessionFetch, jsonCall } from './sessionkey.js';
import { btn } from './formbits.js';
import { artProblem, ART_ICONS } from './sidebarrules.js';
import { artOf, ART_B_CHOICES } from './panellook.js';
import { artNode } from './panelview.js';
import { compressImage } from './artcompress.js';

const send = jsonCall(sessionFetch, { down: 'Sode did not answer, so the image was not uploaded.', failed: (s) => `The image was not uploaded (${s}).` });

async function upload(panelId, file) {
  const sent = await compressImage(file);
  const body = await send(`api/v1/panels/${panelId}/artwork`, { method: 'POST', headers: { 'Content-Type': sent.type || file.type }, body: sent });
  return body?.id;
}

// A pane of the picker: a grid of choices with None first. pick(value) gets undefined for None.
function pane(name, choices, draw, pick) {
  const grid = el('div', 'sode-side__icons');
  grid.setAttribute('role', 'group');
  grid.setAttribute('aria-label', name);
  const none = btn('pb-btn--ghost pb-btn--sm', 'None', () => pick(undefined));
  grid.append(none);
  for (const c of choices) {
    const b = btn('pb-btn--ghost pb-btn--icon pb-btn--sm', '', () => pick(c));
    b.setAttribute('aria-label', c);
    b.dataset.pbTip = c;
    b.append(draw(c));
    grid.append(b);
  }
  return grid;
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
  const clear = btn('pb-btn--outline pb-btn--sm', 'Remove', () => {
    set(undefined);
    setB?.(undefined);
  });
  const picker = el('div', 'sode-side__picker');
  picker.hidden = true;
  const choose = btn('pb-btn--outline pb-btn--sm', 'Choose icon', () => {
    picker.hidden = !picker.hidden;
    choose.setAttribute('aria-expanded', String(!picker.hidden));
  });
  choose.setAttribute('aria-expanded', 'false');

  // Two panes: Icon A is the sprite, Icon B the glyphs. Choosing keeps the picker open, so "[camera] 2" is two clicks.
  const panes = [
    {
      id: 'a',
      text: 'Icon A',
      node: pane(
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
            text: 'Icon B',
            node: pane(
              'Icon B',
              ART_B_CHOICES,
              (g) => el('span', 'sode-side__glyph', g),
              (v) => setB(v),
            ),
          },
        ]
      : []),
  ];
  const tabs = el('div', 'pb-btn-group sode-side__panes');
  tabs.setAttribute('role', 'tablist');
  let shownPane = 'a';
  const showPane = (id) => {
    shownPane = id;
    for (const p of panes) {
      p.node.hidden = p.id !== id;
      p.tab.setAttribute('aria-selected', String(p.id === id));
      p.tab.classList.toggle('pb-btn--outline', p.id !== id);
    }
  };
  for (const p of panes) {
    p.tab = btn('pb-btn--sm', p.text, () => showPane(p.id));
    p.tab.setAttribute('role', 'tab');
    p.tab.id = `ed-art-pane-${p.id}`;
    tabs.append(p.tab);
  }
  picker.append(tabs, ...panes.map((p) => p.node));
  showPane(shownPane);

  let hidden = false;
  const hide = btn('pb-btn--outline pb-btn--sm', 'Hide text', () => setHideText(!hidden));
  hide.hidden = true;
  const bar = el('div', 'sode-side__row', up, ...(icons ? [choose] : []), clear, ...(setHideText ? [hide] : []));
  const node = el('div', 'sode__field', el('span', 'sode__field-label', 'Artwork'), preview, bar, ...(icons ? [picker] : []), note, file);

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
    update(value, caption = '', hideText = false, artB = '') {
      const ro = ed.readOnly();
      const saved = !!ed.panel();
      const any = !!value || !!artB;
      up.disabled = ro || !saved;
      choose.disabled = ro;
      clear.disabled = ro || !any;
      hidden = hideText;
      hide.hidden = !any;
      hide.disabled = ro;
      hide.textContent = hideText ? 'Show text' : 'Hide text';
      for (const p of panes) for (const b of p.node.querySelectorAll('button')) b.disabled = ro;
      for (const b of panes[0].node.querySelectorAll('[aria-label]')) b.setAttribute('aria-pressed', String(value === `icon:${b.getAttribute('aria-label')}`));
      for (const b of panes[1]?.node.querySelectorAll('[aria-label]') || []) b.setAttribute('aria-pressed', String(artB === b.getAttribute('aria-label')));
      if (!saved && !note.textContent) note.textContent = 'Save the panel before uploading an image.';
      else if (saved && note.textContent === 'Save the panel before uploading an image.') note.textContent = '';
      const key = `${value}|${artB}|${caption}|${hideText}`;
      if (shown === key) return;
      shown = key;
      preview.replaceChildren();
      preview.hidden = !any;
      if (!any) return;
      const row = artNode({ art: value, artB }, ed.art());
      if (!row) return;
      preview.append(row);
      if (!hideText) preview.append(el('span', 'sode-side__art-text', caption || (artOf({ art: value })?.icon || artB ? 'Caption' : '')));
    },
  };
}
