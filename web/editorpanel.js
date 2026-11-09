// The sidebar's Panel tab: identity, appearance, behaviour, device and feedback, updates. Every control writes
// one key of the panel's settings through setSetting (sidebarrules.js), so the stored file only holds choices.
import { el, $ } from './core.js';
import { field, check, opt } from './formbits.js';
import { featureSupport } from './panelfeel.js';
import { setSetting, fontPercent, levelsPatch, MODE_HINT, FONT_MAX, FONT_MIN } from './sidebarrules.js';
import { createArtControl } from './editorart.js';
import { linkRow } from './panellinkrow.js';

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
  const linkBox = el('div');
  let linkOf = ''; // id and mode the link row was built for, so a refresh does not rebuild it under a hand
  const identity = section('Identity', field('ed-side-name', 'Name', name), ident, linkBox);

  const size = el('input', 'pb-input sode-ed__num');
  size.id = 'ed-set-font';
  size.type = 'number';
  size.min = String(FONT_MIN);
  size.max = String(FONT_MAX);
  size.inputMode = 'numeric';
  const sizeNote = el('span', 'sode__hint');
  sizeNote.setAttribute('role', 'status');
  size.addEventListener('input', () => {
    const t = size.value.trim();
    const r = t === '' ? { value: 100 } : fontPercent(t);
    sizeNote.textContent = r.error || '';
    if (!r.error) set('fontSize', r.value);
  });
  const sizeField = field('ed-set-font', 'Font size, percent', size, `${FONT_MIN} to ${FONT_MAX}. Each cell can scale from this.`);
  sizeField.append(sizeNote);
  const backdrop = createArtControl({ ed, set: (v) => set('backdrop', v || '') });
  const appearance = section('Appearance', toggle('titleBar', 'Show the title bar'), sizeField, backdrop.node);

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
  const levelBox = el('div', 'sode-side__checks');
  let levels = [];
  const behaviour = section(
    'Behaviour',
    el('div', 'sode__field', el('label', null, 'Mode'), mode.node, modeHint),
    toggle('multi', 'Stage several Destinations at once', 'Off lets one Destination be staged at a time.'),
    el('div', 'sode__field', el('span', 'sode__field-label', 'Levels'), levelBox, el('span', 'sode__hint', 'None ticked means every level.')),
    el('div', 'sode__field', el('label', null, 'Guard'), guard.node, el('span', 'sode__hint', 'A guard on the Take cell stops an accidental press.')),
    toggle('resultText', 'Show the result as text on a Destination', 'Off shows only the colour and icon.'),
    toggle('kiosk', 'Hide the header (kiosk)', 'For a wall tablet. The link can also add kiosk=1.'),
  );

  const feel = featureSupport(navigator);
  const device = section('Device and feedback', toggle('wake', 'Keep the screen awake', ''), toggle('haptics', 'Vibrate on a press', ''));
  const deviceNote = el('p', 'sode__hint');
  device.append(deviceNote);

  const updates = section('Updates', el('p', 'sode__hint', 'An open panel shows a saved change within a few seconds. It waits while someone is mid-route.'));
  node.append(identity, appearance, behaviour, device, updates);

  function drawLevels(chosen) {
    const all = levels.map((l) => l.level);
    levelBox.replaceChildren(
      ...levels.map((l) => {
        const c = check(l.label || `Level ${l.level}`, {
          checked: chosen.includes(l.level),
          onChange: (on) => set('levels', levelsPatch(all, on ? [...chosen, l.level] : chosen.filter((x) => x !== l.level)) || []),
        });
        c.input.disabled = ed.readOnly();
        return c.label;
      }),
    );
    levelBox.dataset.shown = chosen.join(',');
  }

  return {
    node,
    setLevels(list) {
      levels = list;
      levelBox.dataset.shown = '?';
    },
    update() {
      const p = ed.panel();
      const s = ed.fields().settings;
      const ro = ed.readOnly();
      if (document.activeElement !== name) name.value = $('ed-name').value;
      name.disabled = ro;
      ident.textContent = p ? `Id ${p.id}. Owner ${p.owner}.` : 'Not saved yet.';
      const want = p ? `${p.id}|${p.acts}` : '';
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
      const chosen = Array.isArray(s.levels) ? s.levels : [];
      if (levelBox.dataset.shown !== chosen.join(',')) drawLevels(chosen);
      backdrop.update(s.backdrop || '');
    },
  };
}
