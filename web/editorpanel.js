// The sidebar's Panel tab: identity and size, appearance, behavior, device and feedback. Every control writes
// one key of the panel's settings through setSetting (sidebarrules.js), so the stored file only holds choices.
import { el, $ } from './core.js';
import { field, check, opt } from './formbits.js';
import { THEMES } from './panellook.js';
import { featureSupport } from './panelfeel.js';
import { setSetting, fontPercent, MODE_HINT, FONT_MAX, FONT_MIN } from './sidebarrules.js';
import { createSlugField } from './editorslug.js';
import { linkRow } from './panellinkrow.js';
import { createLevelBoxes } from './editorlevels.js';
import { createSizeFields } from './editorsize.js';

const section = (title, ...kids) => el('section', 'sode-side__sec', el('h3', 'sode-side__h', title), ...kids);
const select = (id, options) => {
  const s = el('select', 'pb-select', ...options.map(([v, t]) => opt(v, t)));
  s.id = id;
  return { node: el('span', 'pb-select-wrap', s), select: s };
};

export function createPanelTab(ed) {
  const node = el('div', 'sode-side__tab');
  const set = (key, value) => ed.setFields({ settings: setSetting(ed.fields().settings, key, value) });
  const toggles = [];
  const toggle = (key, text, hint) => {
    const c = check(text, { id: `ed-set-${key}`, onChange: (on) => set(key, on) });
    toggles.push({ key, input: c.input });
    return el('div', 'sode__field', c.label, ...(hint ? [el('span', 'sode__hint', hint)] : []));
  };

  const name = el('input', 'pb-input');
  name.id = 'ed-side-name';
  name.maxLength = 64;
  name.autocomplete = 'off';
  name.addEventListener('input', () => {
    $('ed-name').value = name.value;
    $('ed-name').dispatchEvent(new Event('input'));
  });
  const ident = el('p', 'sode__hint');
  const slug = createSlugField(ed);
  const linkBox = el('div');
  let linkOf = ''; // id and mode the link row was built for, so a refresh does not rebuild it under a hand
  const sizes = createSizeFields(ed);
  const identity = section('Identity', field('ed-side-name', 'Name', name), slug.node, sizes.node, ident, linkBox);

  const size = el('input', 'pb-input sode-ed__num');
  size.id = 'ed-set-font';
  size.type = 'number';
  size.min = String(FONT_MIN);
  size.max = String(FONT_MAX);
  size.inputMode = 'numeric';
  size.placeholder = '100';
  size.title = `${FONT_MIN} to ${FONT_MAX}. Each cell can scale from this.`;
  const sizeNote = el('span', 'sode__hint');
  sizeNote.setAttribute('role', 'status');
  size.addEventListener('input', () => {
    const t = size.value.trim();
    const r = t === '' ? { value: 100 } : fontPercent(t);
    sizeNote.textContent = r.error || '';
    if (!r.error) set('fontSize', r.value);
  });
  const sizeField = el(
    'div',
    'sode__field sode-side__inline',
    Object.assign(el('label', null, 'Font size'), { htmlFor: 'ed-set-font' }),
    size,
    el('span', 'sode__hint', '%'),
  );
  const sizeRow = el('div', 'sode-side__sizerow', sizeField, sizeNote);
  const theme = select('ed-set-theme', [
    ['auto', 'Follow the browser'],
    ['light', 'Light'],
    ['dark', 'Dark'],
  ]);
  theme.select.addEventListener('change', () => set('theme', theme.select.value));
  const appearance = section(
    'Appearance',
    toggle('titleBar', 'Show the title bar'),
    sizeRow,
    el('div', 'sode__field', Object.assign(el('label', null, 'Theme'), { htmlFor: 'ed-set-theme' }), theme.node),
  );

  const mode = select('ed-set-mode', [
    ['take', 'Stage, then Take'],
    ['instant', 'Instant'],
  ]);
  mode.select.addEventListener('change', () => set('mode', mode.select.value));
  const modeHint = el('span', 'sode__hint');
  const guard = select('ed-set-guard', [
    ['none', 'None'],
    ['hold', 'Hold the Take cell'],
    ['double', 'Press the Take cell twice'],
  ]);
  guard.select.addEventListener('change', () => set('guard', guard.select.value));
  let levels = [];
  const levelBoxes = createLevelBoxes({ ed, onPatch: (v) => set('levels', v || []) });
  const behavior = section(
    'Behavior',
    el('div', 'sode__field', el('label', null, 'Mode'), mode.node, modeHint),
    el('div', 'sode__field', el('label', null, 'Take button behavior'), guard.node),
    toggle('multi', 'Stage several Destinations at once', 'Off lets one Destination be staged at a time.'),
    el('div', 'sode__field', el('span', 'sode__field-label', 'Default levels'), levelBoxes.node),
    toggle('resultText', 'Show the result as text on a Destination', 'Off shows only the color and icon.'),
    toggle('kiosk', 'Hide the header (kiosk)', 'For a wall tablet. The link can also add kiosk=1.'),
  );

  const feel = featureSupport(navigator);
  const device = section(
    'Device and feedback',
    toggle('wake', 'Keep the screen awake', ''),
    toggle('haptics', 'Enable haptics', ''),
    toggle('fullscreen', 'Enter fullscreen at once', 'The first press on the panel goes fullscreen, as a browser needs a touch first.'),
  );
  const deviceNote = el('p', 'sode__hint');
  device.append(deviceNote);

  node.append(identity, appearance, behavior, device);

  return {
    node,
    setLevels(list) {
      levels = list;
      levelBoxes.reset();
    },
    update() {
      const p = ed.panel();
      const s = ed.fields().settings;
      const ro = ed.readOnly();
      if (document.activeElement !== name) name.value = $('ed-name').value;
      name.disabled = ro;
      slug.update();
      ident.textContent = p ? `Id ${p.id}. Owner ${p.owner}.` : 'Not saved yet.';
      const want = p ? `${p.id}|${p.acts}|${p.slug || ''}` : '';
      if (want !== linkOf) {
        linkOf = want;
        linkBox.replaceChildren(...(p ? [linkRow(p)] : []));
      }
      for (const { key, input } of toggles) {
        const dflt = key === 'titleBar' || key === 'multi';
        input.checked = dflt ? s[key] !== false : s[key] === true;
        // A switch the editor's own browser cannot honour still shows what the tablet will do; only turning it on is blocked.
        input.disabled = ro || (key === 'wake' && !feel.wake && !input.checked) || (key === 'haptics' && !feel.haptics && !input.checked);
      }
      const unsupported = [!feel.wake && 'keeping the screen awake', !feel.haptics && 'vibration'].filter(Boolean);
      deviceNote.textContent = unsupported.length
        ? `This browser has no support for ${unsupported.join(' or ')}. It is the panel's own browser that counts.`
        : '';
      if (document.activeElement !== size) size.value = s.fontSize ? String(s.fontSize) : '';
      size.disabled = ro;
      mode.select.value = s.mode === 'instant' ? 'instant' : 'take';
      mode.select.disabled = ro;
      modeHint.textContent = MODE_HINT[mode.select.value];
      guard.select.value = s.guard === 'hold' || s.guard === 'double' ? s.guard : 'none';
      guard.select.disabled = ro;
      levelBoxes.draw(levels, s.levels);
      sizes.update();
      theme.select.value = THEMES.includes(s.theme) ? s.theme : 'auto';
      theme.select.disabled = ro;
      // The grid is the preview: its tokens switch with color-scheme, so the page around it keeps the editor's own theme.
      $('ed-grid').style.colorScheme = theme.select.value === 'auto' ? '' : theme.select.value;
    },
  };
}
