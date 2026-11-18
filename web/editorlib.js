// The Choose icon picker's Uploaded icons section. It has the Icon library's icons, and an Icon library button that
// opens the dialog with Upload, Rename, Delete and Rescan. The sidebar is for choosing; the library is changed in the dialog.
// Any editor picks; adding needs a Trusted user or the Advanced Settings PIN, asked for after the first refusal as
// Default artwork does. Tiles draw through the panel's createArt, so their blob URLs go when the panel does. Rename,
// Delete and Rescan (the drop-in directory, docs/reference.md) show only to a Trusted user. The server also accepts
// the PIN, but only from the API.
import { el, on, S } from './core.js';
import { btn } from './formbits.js';
import { trapDialog } from './dialogkeys.js';
import { sessionFetch, jsonCall } from './sessionkey.js';
import { pinNeeded } from './templaterules.js';
import { prepareArt } from './artcompress.js';
import { artImage } from './panelview.js';
import { churnKey, lockoutMs } from './lockrules.js';
import { libNameOf, libNameProblem, libDeleteText, libRescanText, pickerSections, LIB_TYPES } from './iconlibrules.js';

const send = jsonCall(sessionFetch, { down: 'Sode did not answer, so the icon was not uploaded.', failed: (s) => `The icon was not uploaded (${s}).` });
const read = jsonCall(sessionFetch, { down: 'Sode did not answer, so the Icon library was not read.', failed: (s) => `The Icon library was not read (${s}).` });
const change = jsonCall(sessionFetch, {
  down: 'Sode did not answer, so the Icon library was not changed.',
  failed: (s) => `The Icon library was not changed (${s}).`,
});

let trusted = false;
const repaints = new Set();
on('access', (a) => {
  trusted = !!a.trusted && !!a.user;
  for (const fn of repaints) fn();
});

// The body is JSON so the PIN can ride in it; readAsDataURL, since spreading 2 MB into btoa throws.
const base64 = (blob) =>
  new Promise((done, fail) => {
    const r = new FileReader();
    r.onload = () => done(String(r.result).slice(String(r.result).indexOf(',') + 1));
    r.onerror = () => fail(new Error('The file could not be read.'));
    r.readAsDataURL(blob);
  });

// set(value) picks for the cell; gone() says a delete happened. update(value) repaints; filter(text) applies the
// picker's search.
export function createLibSection({ ed, set, gone = () => {} }) {
  let icons = [];
  let text = '';
  let value = '';
  let drawnWith; // the createArt the tiles were drawn with
  const grid = el('div', 'sode-side__icons sode-side__lib');
  grid.id = 'ed-lib-icons';
  grid.setAttribute('role', 'group');
  grid.setAttribute('aria-label', 'Uploaded icons');
  // The dialog's own tiles choose the icon to rename or delete.
  const chooseGrid = el('div', 'sode-side__icons sode-side__lib');
  chooseGrid.id = 'ed-lib-manage-icons';
  chooseGrid.setAttribute('role', 'group');
  chooseGrid.setAttribute('aria-label', 'Icons to rename or delete');
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

  let chosen = ''; // the icon id Rename and Delete act on
  let confirming = null; // {key, ready, lock, go}
  const rescan = btn('pb-btn--ghost pb-btn--sm', 'Rescan', () => doRescan());
  rescan.id = 'ed-lib-rescan';
  rescan.dataset.pbTip = 'Add the files in icons/drop-in beside the config';
  const rename = el('input', 'pb-input sode-side__find');
  rename.id = 'ed-lib-rename-name';
  rename.autocomplete = 'off';
  rename.setAttribute('aria-label', 'New name for the icon');
  const renameBtn = btn('pb-btn--outline pb-btn--sm', 'Rename', () => doRename());
  renameBtn.id = 'ed-lib-rename';
  const del = btn('pb-btn--danger pb-btn--sm', 'Delete', () => askDelete());
  del.id = 'ed-lib-delete';
  const manageRow = el('div', 'sode-side__row', rename, renameBtn, del);
  const manageHint = el('p', 'sode__hint', 'Choose an icon to rename or delete.');
  const manageSec = el('div', 'sode-side__lib-sec', el('span', 'sode-side__col-h', 'Rename or delete'), chooseGrid, manageHint, manageRow);
  const confirmBox = el('div', 'sode-side__lib-confirm');
  confirmBox.id = 'ed-lib-confirm';
  confirmBox.tabIndex = -1;
  confirmBox.hidden = true;
  // Stopped here so the letter, Enter or Esc never reach the grid's keys or the picker.
  confirmBox.addEventListener('keydown', (e) => {
    if (!confirming || e.ctrlKey || e.metaKey || e.altKey) return;
    if (e.key === 'Escape') cancelAsk();
    else if (e.key.toLowerCase() === confirming.key || (S.cfg?.confirmEnter && e.key === 'Enter')) confirming.go();
    else return;
    e.preventDefault();
    e.stopPropagation();
  });

  // At the picker's right edge, so its tip opens leftward.
  const open = btn('pb-btn--ghost pb-btn--sm pb-tip--end', 'Icon library', () => openDialog());
  open.id = 'ed-lib-open';
  open.setAttribute('aria-haspopup', 'dialog');
  open.dataset.pbTip = 'Upload, rename or delete the icons every panel can use';
  const node = el('div', 'sode-side__lib-sec', el('div', 'sode-side__lib-head', el('span', 'sode-side__picker-h', 'Uploaded icons'), open), grid, empty);

  const root = el('div', 'pb-modal-backdrop sode__scrim');
  root.id = 'ed-lib-dialog';
  root.hidden = true;
  const dialog = el('div', 'pb-modal');
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', 'ed-lib-title');
  dialog.tabIndex = -1;
  const close = btn('pb-btn--outline', 'Close', () => closeDialog());
  close.id = 'ed-lib-close';
  dialog.append(
    el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Icon library'), { id: 'ed-lib-title' }), rescan),
    el(
      'div',
      'pb-modal__body sode-side__lib-sec',
      el('p', 'sode__hint', 'Every panel can use these icons. A new upload also goes on the selection.'),
      el('div', 'sode-side__row', name, up),
      pinRow,
      manageSec,
      confirmBox,
      note,
      file,
    ),
    el('div', 'pb-modal__footer', close),
  );
  root.append(dialog);
  document.body.append(root);
  const dlg = trapDialog(root, dialog, () => closeDialog());
  let back = null; // focus goes back here on close
  function openDialog() {
    back = document.activeElement;
    chosen = '';
    note.textContent = '';
    dlg.show();
    paint();
    name.focus();
  }
  function closeDialog() {
    if (root.hidden) return;
    cancelAsk();
    dlg.hide();
    if (back?.isConnected) back.focus({ preventScroll: true });
  }
  on('locking', closeDialog);

  function paint() {
    const art = ed.art();
    drawnWith = art;
    if (!icons.some((ic) => ic.id === chosen)) chosen = '';
    const tiles = pickerSections(icons, [], text)[0].tiles;
    const tile = (t, choose) => {
      const b = btn('pb-btn--ghost pb-btn--icon pb-btn--sm', '', () => {
        if (!choose) return set(t.value);
        chosen = t.id;
        rename.value = t.name;
        cancelAsk();
        paint();
      });
      b.dataset.lib = t.id;
      b.dataset.pbTip = t.name;
      b.setAttribute('aria-label', t.name);
      b.setAttribute('aria-pressed', String(choose ? chosen === t.id : value === t.value));
      b.disabled = !choose && ed.readOnly();
      b.append(art ? artImage('sode-side__lib-img', art, t.value, true) : el('span', 'sode-side__glyph', [...t.name][0]));
      return b;
    };
    grid.replaceChildren(...tiles.map((t) => tile(t, false)));
    grid.hidden = !tiles.length;
    empty.hidden = !!tiles.length;
    empty.textContent = icons.length ? 'No uploaded icon matches.' : 'No uploaded icons yet.';
    // The dialog lists every icon: the picker's search does not apply there.
    chooseGrid.replaceChildren(...(trusted && !root.hidden ? pickerSections(icons, [], '')[0].tiles.map((t) => tile(t, true)) : []));
    manageSec.hidden = !trusted || !icons.length;
    rescan.hidden = !trusted;
    manageHint.hidden = !!chosen;
    manageRow.hidden = !chosen;
  }
  repaints.add(paint);

  function cancelAsk() {
    if (confirming) clearTimeout(confirming.lock);
    confirming = null;
    confirmBox.hidden = true;
    confirmBox.replaceChildren();
  }

  async function doRescan() {
    note.textContent = 'Rescanning.';
    rescan.disabled = true;
    try {
      note.textContent = libRescanText(await change('api/v1/icon-library/rescan', { method: 'POST' }));
      await load();
    } catch (e) {
      note.textContent = e.message;
    } finally {
      rescan.disabled = false;
    }
  }

  async function doRename() {
    const ic = icons.find((x) => x.id === chosen);
    const n = rename.value.trim();
    const problem = libNameProblem(n);
    if (!ic || problem) return void (note.textContent = problem);
    try {
      const got = await change(`api/v1/icon-library/${ic.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: n }),
      });
      note.textContent = got.name === n ? `Renamed to ${got.name}.` : `Renamed to ${got.name}, since ${n} is already in use.`;
      await load();
    } catch (e) {
      note.textContent = e.message;
    }
  }

  // The count is read as the Confirm opens, so it is never one from before another editor's save.
  async function askDelete() {
    const ic = icons.find((x) => x.id === chosen);
    if (!ic) return;
    cancelAsk();
    const uses = await change(`api/v1/icon-library/${ic.id}/uses`).catch(() => null);
    if (chosen !== ic.id || root.hidden) return;
    const key = churnKey();
    const keyEl = el('kbd', 'sode__key sode__key--locked', key.toUpperCase());
    const ok = btn('pb-btn--danger pb-btn--sm', `Delete (${key.toUpperCase()})`, () => c.go());
    ok.id = 'ed-lib-delete-ok';
    ok.disabled = true;
    const cancel = btn('pb-btn--outline pb-btn--sm', 'Cancel', () => cancelAsk());
    const c = { key, ready: false, lock: 0 };
    c.lock = setTimeout(() => {
      c.ready = true;
      keyEl.classList.remove('sode__key--locked');
      ok.disabled = false;
    }, lockoutMs(S.cfg));
    c.go = async () => {
      if (!c.ready || confirming !== c) return;
      cancelAsk();
      try {
        await change(`api/v1/icon-library/${ic.id}`, { method: 'DELETE' });
        note.textContent = `Deleted ${ic.name}.`;
        chosen = '';
        ed.art()?.forget(`lib:${ic.id}`);
        gone();
        ed.refresh();
        await load();
      } catch (e) {
        note.textContent = e.message;
      }
    };
    confirming = c;
    confirmBox.replaceChildren(el('p', null, libDeleteText(ic.name, uses)), el('div', 'sode__prompt', 'Press', keyEl, 'to confirm', ok, cancel));
    confirmBox.hidden = false;
    confirmBox.focus();
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
      note.textContent = ic.name === n ? `Added ${ic.name}.` : `Added as ${ic.name}, since ${n} is already in use.`;
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
