// The sidebar's Panel tab: identity and size, appearance, behavior, device and feedback. Every control writes
// one key of the panel's settings through setSetting (sidebarrules.js), so the stored file only holds choices.
import { el, $, S } from './core.js';
import { field, check, opt, segmented } from './formbits.js';
import { THEMES } from './panellook.js';
import { featureSupport, fullscreenNote, LANDSCAPE_HINT } from './panelfeel.js';
import { setSetting, fontPercent, FONT_MAX, FONT_MIN } from './sidebarrules.js';
import { createSlugField } from './editorslug.js';
import { linkRow } from './panellinkrow.js';
import { createLevelBoxes } from './editorlevels.js';
import { createSizeFields } from './editorsize.js';
import { createFixedField } from './editorfixed.js';
import { fixedDestsOf } from './panelrules.js';
import { isGridPanel } from './gridpaneledges.js';
import { followRouter, stopFollowing } from './editorgridrules.js';

const GRID_MODE_HINT =
  'Instant sends a press on one crosspoint at once and asks before more routes. Stage + Take: Take sends with no confirm box. Stage + Take + Confirm: Take opens the confirm box.';

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
  const ident = el('p', 'sode__hint sode-side__ident');
  const slug = createSlugField(ed);
  const linkBox = el('div');
  let linkOf = ''; // id and mode the link row was built for, so a refresh does not rebuild it under a hand
  const sizes = createSizeFields(ed);
  // A new Grid panel starts with no sources and no destinations, as a Buttons panel starts with no buttons. Edits of
  // the other type's parts go, so a save never sends a Buttons panel's edges.
  const type = segmented(
    'ed-set-type',
    'Panel type',
    [
      ['buttons', 'Buttons'],
      ['grid', 'Grid'],
    ],
    (v) =>
      ed.setFields(
        v === 'grid' ? { type: v, sources: { members: [] }, destinations: { members: [] } } : { type: v, sources: undefined, destinations: undefined },
      ),
  );
  const typeNote = el('span', 'sode__hint');
  const typeField = el('div', 'sode__field', el('span', 'sode__field-label', 'Panel type'), type.node, typeNote);
  const identity = section('Identity', field('ed-side-name', 'Name', name), typeField, slug.node, sizes.node, linkBox);
  identity.firstChild.replaceWith(el('div', 'sode-side__head', el('h3', 'sode-side__h', 'Identity'), ident));

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
  const titleBar = toggle('titleBar', 'Show the title bar');
  const sizeRow = el('div', 'sode-side__sizerow', titleBar, sizeField, sizeNote);
  const theme = select('ed-set-theme', [
    ['auto', 'Follow the browser'],
    ['light', 'Light'],
    ['dark', 'Dark'],
  ]);
  theme.select.addEventListener('change', () => set('theme', theme.select.value));
  const appearance = section(
    'Appearance',
    sizeRow,
    el('div', 'sode__field', Object.assign(el('label', null, 'Theme'), { htmlFor: 'ed-set-theme' }), theme.node),
  );

  // One stored mode for both types. A Grid panel adds takeOnly, its Take mode with no confirm box. A Buttons panel
  // has no confirm box, so it shows only take and instant.
  const modeOptions = {
    buttons: [
      ['take', 'Stage, then Take'],
      ['instant', 'Instant'],
    ],
    grid: [
      ['instant', 'Instant'],
      ['takeOnly', 'Stage + Take'],
      ['take', 'Stage + Take + Confirm'],
    ],
  };
  const modeHint = el('span', 'sode__hint');
  modeHint.id = 'ed-mode-note';
  let modeType = '';
  let mode = null;
  const modeField = el('div', 'sode__field', el('span', 'sode__field-label', 'Mode'));
  const drawMode = (kind) => {
    if (kind === modeType) return;
    modeType = kind;
    const next = segmented('ed-set-mode', 'Mode', modeOptions[kind], (v) => set('mode', v));
    if (mode) mode.node.replaceWith(next.node);
    else modeField.append(next.node, modeHint);
    mode = next;
    modeHint.textContent = kind === 'grid' ? GRID_MODE_HINT : '';
  };
  drawMode('buttons');
  const guard = segmented(
    'ed-set-guard',
    'Take button behavior',
    [
      ['none', 'None'],
      ['hold', 'Hold'],
      ['double', 'Twice'],
    ],
    (v) => set('guard', v),
  );
  const levelBoxes = createLevelBoxes({ ed, onPatch: (v) => set('levels', v || []) });
  const levelField = el('div', 'sode__field', el('span', 'sode__field-label', 'Default levels'), levelBoxes.node);
  const fixed = createFixedField(ed);
  const multi = toggle('multi', 'Stage several Destinations at the same time', 'Off lets one Destination be staged at a time.');
  const multiNote = el('span', 'sode__hint');
  multiNote.id = 'ed-multi-note';
  multi.append(multiNote);
  const guardField = el('div', 'sode__field', el('span', 'sode__field-label', 'Take button behavior'), guard.node);
  const resultText = toggle('resultText', 'Show the result as text on a Destination');
  const wrapPages = toggle('wrapPages', 'Wrap between Pages', 'Next Page on the last Page goes to the first, and Previous Page on the first goes to the last.');
  // Settings that only a Buttons panel uses; a Grid panel keeps them stored but does not show them.
  const buttonsOnly = [sizes.node, guardField, fixed.node, multi, resultText, wrapPages];
  const behavior = section('Behavior', modeField, guardField, fixed.node, multi, levelField, resultText, toggle('kiosk', 'Kiosk mode'), wrapPages);

  // A Grid panel's edges: each one is the router's list, or holds its own. One section for the router, so a
  // choice per router has a place if Sode gets more than one.
  const followSwitches = ['sources', 'destinations'].map((key) => {
    const what = key === 'sources' ? 'source' : 'destination';
    const input = el('input', 'pb-switch__input');
    input.type = 'checkbox';
    input.id = `ed-${key}-all`;
    input.setAttribute('role', 'switch');
    input.addEventListener('change', () => {
      const edge = ed.fields()[key];
      const next = input.checked ? followRouter(edge) : stopFollowing(edge, ed.routerNames()?.[key] || []);
      if (next !== edge) ed.setFields({ [key]: next });
    });
    const label = el('label', 'pb-check', input, el('span', 'pb-check__label', `${what[0].toUpperCase()}${what.slice(1)}s follow the router`));
    const hint = `On: every ${what} the router has, in router order, and the ones it adds later. Pin and artwork still apply. Off: the list keeps what it shows now.`;
    return { key, input, node: el('div', 'sode__field', label, el('span', 'sode__hint', hint)) };
  });
  const router = section('Router', ...followSwitches.map((f) => f.node));

  const feel = featureSupport(navigator);
  const fullscreenField = toggle('fullscreen', 'Automatically enter fullscreen');
  const fullscreenHint = el('span', 'sode__hint');
  fullscreenHint.id = 'ed-fullscreen-note';
  fullscreenField.append(fullscreenHint);
  const device = section(
    'Device and feedback',
    toggle('wake', 'Keep the screen awake', ''),
    fullscreenField,
    toggle('landscape', 'Enforce landscape orientation', LANDSCAPE_HINT),
    toggle('askInstanceName', 'Ask users to set an instance name', 'Off, the panel never asks and shows no name in its header.'),
  );
  const deviceNote = el('p', 'sode__hint');
  device.append(deviceNote);

  node.append(identity, router, appearance, behavior, device);

  return {
    node,
    resetLevels: () => levelBoxes.reset(),
    update() {
      const p = ed.panel();
      const s = ed.fields().settings;
      const ro = ed.readOnly();
      if (document.activeElement !== name) name.value = $('ed-name').value;
      name.disabled = ro;
      slug.update();
      const grid = isGridPanel(ed.fields());
      type.set(grid ? 'grid' : 'buttons');
      type.disable(ro || !!p);
      typeNote.textContent = p ? 'The type stays as the panel was made.' : 'You cannot change the type after the first save.';
      for (const n of buttonsOnly) n.hidden = grid;
      router.hidden = !grid;
      for (const f of followSwitches) {
        f.input.checked = grid && !Array.isArray(ed.fields()[f.key]?.members);
        f.input.disabled = ro;
      }
      ident.replaceChildren(...(p ? [`Id ${p.id}`, el('br'), `Owned by user "${p.owner}"`] : ['Not saved yet.']));
      const want = p ? `${p.id}|${p.acts}|${p.slug || ''}` : '';
      if (want !== linkOf) {
        linkOf = want;
        linkBox.replaceChildren(...(p ? [linkRow(p, { withHost: false })] : []));
      }
      for (const { key, input } of toggles) {
        const dflt = key === 'titleBar' || key === 'multi' || key === 'wrapPages' || key === 'askInstanceName';
        input.checked = dflt ? s[key] !== false : s[key] === true;
        // A switch the editor's own browser cannot honour still shows what the tablet will do; only turning it on is blocked.
        input.disabled = ro || (key === 'wake' && !feel.wake && !input.checked);
      }
      // The fixed set is always staged together, so the stored choice stays but has no effect.
      const fixedOn = fixedDestsOf({ settings: s }).length > 0;
      toggles.find((x) => x.key === 'multi').input.disabled = ro || fixedOn;
      multiNote.textContent = fixedOn ? 'No effect while Fixed destination is on: every press stages all of them together.' : '';
      fixed.update();
      fullscreenHint.textContent = fullscreenNote(s);
      deviceNote.textContent = feel.wake ? '' : "This browser cannot keep the screen awake. It is the panel's own browser that counts.";
      if (document.activeElement !== size) size.value = String(s.fontSize || 100); // shown, so the arrows step from the default
      size.disabled = ro;
      drawMode(grid ? 'grid' : 'buttons');
      mode.set(s.mode === 'instant' ? 'instant' : grid && s.mode === 'takeOnly' ? 'takeOnly' : 'take');
      mode.disable(ro);
      guard.set(s.guard === 'hold' || s.guard === 'double' ? s.guard : 'none');
      guard.disable(ro);
      levelField.hidden = !S.levels.length;
      levelBoxes.draw(s.levels);
      sizes.update();
      theme.select.value = THEMES.includes(s.theme) ? s.theme : 'auto';
      theme.select.disabled = ro;
      // The grid is the preview: its tokens switch with color-scheme, so the page around it keeps the editor's own theme.
      $('ed-grid').style.colorScheme = theme.select.value === 'auto' ? '' : theme.select.value;
    },
  };
}
