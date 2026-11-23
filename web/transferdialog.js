// The two Export & Import dialogs: Settings > Server (exportdialog.js) and My user (ownexport.js). The Import tab is
// importtab.js; pure rules are in exportrules.js.
import { el } from './core.js';
import { btn } from './formbits.js';
import { modal } from './modal.js';
import { makeTabs } from './tabs.js';
import { layout } from './layout.js';
import { desktopOnly } from './desktoponly.js';
import { sizeText } from './exportrules.js';
import { post, call, downloadZip } from './exportcalls.js';
import { importTab } from './importtab.js';
import { keepFocus } from './transfertree.js';

const SIZE_WAIT_MS = 300;
const TITLE = 'Export & Import';

// id prefixes the element ids; api is the export route, with /items and /size under it. bodyOf(list, pick) is the
// export body, or null when nothing is picked. draw(x) gives the tree rows. backupFile adds Back up everything.
export function transferDialog({ id, cls, own = false, api, newPick, bodyOf, draw, top = [], busyNote, backupFile = null }) {
  const cats = el('div', 'sode__xi-cats');
  const exportPanel = el('div', null, ...top, cats);
  const phone = el('div');
  const note = el('span', { class: 'sode__hint', id: `${id}-note`, role: 'status', 'aria-live': 'polite' });
  let back = null;
  let sizeTimer = 0;
  let sizeSeq = 0;
  const x = { list: null, pick: null, open: false, expanded: new Set(), render, changed, sizeLater };

  const setNote = (t, error = false) => {
    note.textContent = t;
    note.classList.toggle('sode__error-text', error);
  };
  const body = () => bodyOf(x.list, x.pick);
  const exportBtn = btn('sode-btn--primary', 'Export', () => download(() => downloadZip(body(), api)), `${id}-export`);
  const backupBtn = backupFile && btn('pb-btn--outline', 'Back up everything', () => download(backupFile), `${id}-backup`);
  const exportActs = el('span', 'sode__xi-acts', ...(backupBtn ? [backupBtn] : []), exportBtn);
  const importer = importTab({
    own,
    setNote,
    // The review fills the whole body, so the tabs go while it shows.
    setReview: (title) => {
      dlg.title.textContent = title || TITLE;
      tabs.node.hidden = !!title;
    },
    backup: () => backupFile && download(backupFile),
  });
  const tabs = makeTabs(
    [
      { id: `${id}-export-tab`, text: 'Export', panel: exportPanel },
      { id: `${id}-import-tab`, text: 'Import', panel: importer.panel },
    ],
    (tab) => {
      exportActs.hidden = tab !== `${id}-export-tab`;
      importer.acts.hidden = tab !== `${id}-import-tab`;
    },
  );
  const dlg = modal({
    titleId: `${id}-title`,
    title: TITLE,
    cls: `pb-modal--md ${cls}`,
    body: [tabs.node, exportPanel, importer.panel, phone],
    foot: [btn('pb-btn--outline', 'Close', close, `${id}-close`), note, exportActs, importer.acts],
    onEsc: close,
  });

  function render() {
    if (!x.list) return;
    keepFocus(cats, () => cats.replaceChildren(...draw(x)));
    exportBtn.disabled = !body();
  }

  function changed() {
    render();
    sizeLater();
  }

  // The zip size, built by the server as the export would be.
  function sizeLater() {
    clearTimeout(sizeTimer);
    const seq = ++sizeSeq;
    exportBtn.textContent = 'Export';
    const b = body();
    if (!b) return;
    sizeTimer = setTimeout(async () => {
      try {
        const { size } = await (await post(`${api}/size`, b)).json();
        if (seq === sizeSeq && x.open) exportBtn.textContent = `Export ${sizeText(size)}`;
      } catch (e) {
        if (seq === sizeSeq && x.open) setNote(e.message, true);
      }
    }, SIZE_WAIT_MS);
  }

  async function load() {
    setNote('Reading…');
    try {
      const got = await (await call(`${api}/items`)).json();
      if (!x.open) return;
      x.list = got;
      x.pick = newPick(got);
      setNote('');
      changed();
    } catch (e) {
      if (x.open) setNote(e.message, true);
    }
  }

  // make resolves { name, size } of the saved zip (exportcalls.js).
  async function download(make) {
    setNote(busyNote);
    for (const b of exportActs.children) b.disabled = true;
    try {
      const got = await make();
      setNote(`Saved ${got.name} (${sizeText(got.size)}).`);
    } catch (e) {
      setNote(e.message, true);
    } finally {
      if (backupBtn) backupBtn.disabled = false;
      exportBtn.disabled = !x.list || !body();
    }
  }

  function close() {
    x.open = false;
    clearTimeout(sizeTimer);
    x.list = x.pick = null;
    cats.replaceChildren();
    importer.reset();
    exportBtn.textContent = 'Export';
    dlg.hide();
    if (back?.isConnected) back.focus();
  }

  x.openDialog = () => {
    back = document.activeElement;
    x.open = true;
    const desk = layout() !== 'phone';
    tabs.node.hidden = !desk;
    exportActs.hidden = !desk;
    importer.acts.hidden = true;
    phone.replaceChildren(desk ? '' : desktopOnly('export or import'));
    setNote('');
    if (desk) {
      tabs.select(`${id}-export-tab`);
      exportBtn.disabled = true; // until the list comes
      load();
    } else exportPanel.hidden = importer.panel.hidden = true;
    dlg.show();
    dlg.dialog.focus();
  };
  return x;
}
