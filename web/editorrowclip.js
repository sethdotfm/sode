// Cut, copy and paste of a Grid panel's list rows, in the editor's lists and the Live Editor's grid. The block rules are
// edgeclip.js; the clipboard I/O is editorclip.js. A cut and a paste are each one editor undo step (ed.editEdges).
import { writeClip } from './editorclip.js';
import { copyRows, rowsText, readRows, pasteRows, pasteRowsNote } from './edgeclip.js';
import { removeItems } from './editorgridrules.js';
import { myItemRights } from './editorrights.js';
import { plural } from './gridmodel.js';

const RIGHTS = { sources: 'sources', destinations: 'dsts' };

// lists: { sources, destinations }, the list apis (editorgridlist.js).
export function createRowClip(ed, lists) {
  const tell = (text) => ed.status(text);
  const noun = (key) => lists[key].what;

  // Answers true when the rows reached the clipboard.
  async function copy(key) {
    const ns = lists[key].picked().map((r) => r.n);
    const block = copyRows(ed.fields()[key], key, ns, ed.panel()?.id);
    if (!block) {
      tell(`Nothing copied: no ${noun(key)}s selected.`);
      return false;
    }
    const ok = await writeClip(rowsText(block));
    tell(ok ? `Copied ${plural(ns.length, noun(key))}.` : 'Nothing copied: the browser did not let Sode use the clipboard.');
    return ok;
  }

  // Copy, then one edit removes the rows. A failed copy, or a list that changed during the write, removes nothing.
  async function cut(key) {
    if (ed.readOnly()) return;
    const list = lists[key];
    if (list.following()) return tell(`Nothing cut: Follow the router is on for ${noun(key)}s, so the router sets the list. Copy still works.`);
    const before = ed.fields()[key];
    const ns = list.picked().map((r) => r.n);
    if (!(await copy(key))) return;
    if (ed.fields()[key] !== before) return tell('Copied. Nothing removed: the list changed.');
    ed.editEdges({ [key]: removeItems(before, new Set(ns)) }, 'cut');
    list.clearPicks();
    tell(`Cut ${plural(ns.length, noun(key))}.`);
  }

  // key is the list with focus, where the rows go after its cursor row. A null key (the Live Editor's grid) pastes into
  // the list of the block's own axis. The rows go after the router number that after(that list's key) gives, or at the
  // end for null. Answers { key, added }.
  function paste(text, key, after = () => null) {
    const none = { key, added: [] };
    if (ed.readOnly()) return none;
    if (text === null) {
      tell('Nothing pasted: the browser did not let Sode read the clipboard.');
      return none;
    }
    const block = readRows(text);
    if (!block) {
      tell('Nothing pasted: no Sode sources or destinations on the clipboard.');
      return none;
    }
    const into = key ?? block.axis;
    const list = lists[into];
    const edge = ed.fields()[into];
    const r = pasteRows(edge, ed.routerNames()?.[into] || [], block, {
      axis: into,
      rights: myItemRights(ed.places()),
      kind: RIGHTS[into],
      panel: ed.panel()?.id || '',
      after: key ? list.after() : after(into),
    });
    if (r.edge !== edge) ed.editEdges({ [into]: r.edge }, 'paste');
    if (r.added.length) list.selectNew(r.added);
    tell(pasteRowsNote(r, noun(into)));
    return { key: into, added: r.added };
  }

  return { copy, cut, paste };
}
