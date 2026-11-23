// The Live Editor's Smart menu for a Grid panel. It sits beside the picked names on the live grid. It edits them
// through the drawer's own list edits (editorgridlist.js), so the drawer, the run and a save see one change. This file
// also holds the Live Editor's own keys: editor undo and redo, and the rows clipboard (editorrowclip.js). There `u`
// does nothing, since route Undo is the panel page's. The phone layout has no Smart menu, only the drawer.
import { el } from './core.js';
import { chord } from './keys.js';
import { gridCommand } from './gridchords.js';
import { createSmartMenu } from './editormenu.js';
import { createPasteReader } from './editorclip.js';
import { createRouterList } from './editorrouterlist.js';
import { MOVE_KEYS } from './editorgridlist.js';
import { edgeListKey } from './editorgridrules.js';
import { pickAfterRemove } from './edgeclip.js';
import { placeMenu, placeAbove, menuArea } from './editorrules.js';
import { myItemRights } from './editorrights.js';
import { plural } from './gridmodel.js';

const KEY = { srcs: 'sources', dsts: 'destinations' };
const KIND = { sources: 'srcs', destinations: 'dsts' };
const RIGHTS = { srcs: 'sources', dsts: 'dsts' };
const OFF = { left: 0, top: 0, right: 0, bottom: 0 }; // a box for a menu that stays hidden until its name shows
const phone = () => document.documentElement.dataset.layout === 'phone';

// ed is the editor's seams (paneleditor.js); lists the list apis with the rows clipboard ({ srcs, dsts, clip }), or
// null when the panel is read only. kind() is the kind picked last, whose list holds the picks. openArt() shows the
// Artwork panel. drawer() is the open drawer's node, or null, which the menu keeps off.
export function createLiveMenu(ed, lists, { kind, openArt, drawer }) {
  let run = null; // the run's hooks: pick(kind, ns) and cursor() (gridpanelrun.js)
  let grid = null; // the run's .sode-panel__grid
  let reader = null;
  let picks = { srcs: [], dsts: [] };
  let frame = 0;
  const vp = () => ({ w: document.documentElement.clientWidth, h: document.documentElement.clientHeight });
  const menu = createSmartMenu(null, {
    onEscape: () => grid?.querySelector('.sode-gridpanel__scroll')?.focus({ preventScroll: true }),
    area: () => menuArea(vp(), drawer()?.getBoundingClientRect() || null, grid?.getBoundingClientRect() || null),
    place: (box, m, area, gap) => (kind() === 'srcs' ? placeAbove : placeMenu)(box, m, area, gap),
  });
  menu.root.id = 'ed-live-menu';
  menu.root.setAttribute('aria-label', 'Smart menu');

  const list = () => lists[kind()];
  const tell = (text) => ed.status(text);
  const following = () => list().following();
  const picked = () =>
    list()
      .picked()
      .map((r) => r.n);

  // Insert: the router's names the panel does not have, as Add from the router lists them. They go in after the last
  // picked name in list order.
  const inserts = {};
  function insertList(k) {
    const key = KEY[k];
    inserts[k] ??= createRouterList({
      key: `${key}-ins`,
      what: lists[k].what,
      title: 'Insert from the router',
      kind: RIGHTS[k],
      edge: () => ed.fields()[key],
      router: () => ed.routerNames()?.[key] || [],
      rights: () => myItemRights(ed.places()),
      names: () => ed.routerNames() || {},
      readOnly: ed.readOnly,
      fixed: () => '',
      set: (next) => ed.editEdges({ [key]: next }, 'insert'),
      at: () => {
        const members = ed.fields()[key]?.members || [];
        const last = Math.max(-1, ...picks[k].map((n) => members.indexOf(n)));
        return last < 0 ? members.length : last + 1;
      },
      onAdd: (ns) => run?.pick(k, ns.slice(0, 1)),
    });
    return inserts[k];
  }
  const flyouts = { srcs: null, dsts: null };
  function insertPanel(k) {
    if (!flyouts[k]) {
      flyouts[k] = el('div', 'pb-menu pb-menu__sub sode-find sode-live__insert', insertList(k).node);
      flyouts[k].id = `ed-live-insert-${KEY[k]}`;
    }
    insertList(k).paint();
    return flyouts[k];
  }
  const openInsert = () => {
    const panel = insertPanel(kind());
    panel.dataset.open = '1';
    panel.querySelector('input')?.focus();
  };

  function removePicked() {
    const ns = picked();
    if (!ns.length || following()) return;
    const k = kind();
    const next = pickAfterRemove(list().shown(), new Set(ns));
    list().remove(ns);
    run?.pick(k, next === null ? [] : [next]);
    tell(`Removed ${plural(ns.length, list().what)} from the panel.`);
  }

  function items() {
    const rows = list().picked();
    const pinned = rows.length > 0 && rows.every((r) => r.pinned);
    const out = [
      { label: 'Artwork', icon: 'image', run: () => openArt() },
      pinned
        ? { label: 'Unpin', icon: 'pin-off', chord: 'p', run: () => list().pin(picked(), false) }
        : { label: 'Pin', icon: 'pin', chord: 'p', run: () => list().pin(picked(), true) },
    ];
    // Follow the router: the router sets the order and the members, so only Artwork and Pin apply.
    if (following()) return out;
    const move = (label, dir) => ({ label, hint: MOVE_KEYS[dir], run: () => list().move(dir) });
    out.push(
      { label: 'Move', icon: 'arrow-up-down', sub: [move('To the top', 'top'), move('Up', 'up'), move('Down', 'down'), move('To the end', 'end')] },
      { label: 'Insert', icon: 'list-plus', chord: 'i', panel: insertPanel(kind()), run: openInsert },
      { divider: true },
      { label: 'Remove from the panel', icon: 'trash-2', keys: 'Delete', run: removePicked },
    );
    return out;
  }

  const open = () => !!run && !!grid && !!lists && !phone() && !!kind() && picks[kind()].length > 0;

  // The cursor's name when the cursor is on the picked bar, else the last pick. null while that name is out of view.
  function anchor() {
    const k = kind();
    const c = run.cursor();
    const at = k === 'srcs' ? c.src : c.dst;
    const n = c.bar === k && picks[k].includes(at) ? at : picks[k].at(-1);
    const [mine, other] = k === 'srcs' ? ['src', 'dst'] : ['dst', 'src'];
    const view = grid.querySelector('.sode-gridpanel__scroll')?.getBoundingClientRect();
    for (const node of grid.querySelectorAll(`[data-${mine}="${n}"]:not([data-${other}])`)) {
      const r = node.getBoundingClientRect();
      if (!r.width || !r.height) continue;
      if (view && (r.right <= view.left || r.left >= view.right || r.bottom <= view.top || r.top >= view.bottom)) continue;
      return r;
    }
    return null;
  }

  // A rebuild loses focus in the Insert flyout, since its node moves: it goes back to the same field.
  function show() {
    frame = 0;
    if (!open()) return menu.hide();
    const had = menu.root.contains(document.activeElement) ? document.activeElement : null;
    const box = anchor();
    menu.show(box || OFF, items());
    if (!box) menu.move(null);
    if (had && had.closest('.sode-live__insert') && !menu.root.contains(document.activeElement)) had.focus({ preventScroll: true });
  }
  const later = () => (frame ||= requestAnimationFrame(show));
  const follow = () => open() && menu.move(anchor());

  // While a pointer is down on the grid (a Drawn line, a drag along a bar), the menu fades out of its way.
  const away = (on) => menu.root.toggleAttribute('data-away', on);
  const down = (e) => grid?.contains(e.target) && away(true);
  const up = () => {
    away(false);
    later();
  };
  document.addEventListener('pointerdown', down, true);
  document.addEventListener('pointerup', up, true);
  document.addEventListener('pointercancel', up, true);
  window.addEventListener('resize', later);

  return {
    root: menu.root,
    // The run's hooks and grid node, once the run has drawn.
    useRun(r, node) {
      run = r;
      grid = node;
      grid.addEventListener('scroll', follow, { capture: true, passive: true });
      reader = createPasteReader(grid, (text) => {
        const r = lists?.clip.paste(text, null, (key) => {
          const members = ed.fields()[key]?.members || [];
          const ns = picks[KIND[key]];
          return ns.length ? members[Math.max(...ns.map((n) => members.indexOf(n)))] : null;
        });
        if (r?.added.length) run?.pick(KIND[r.key], r.added);
      });
      later();
    },
    // Each new picks object from the run, after the lists took it.
    picks(p) {
      picks = { srcs: p.srcs, dsts: p.dsts };
      later();
    },
    // The editor's fields changed: Pin or Unpin, and Follow the router, can change the items.
    changed: later,
    // The run's keydown (gridpanelrun.js editKey). ctx is { bar, inControl, line }. True when the key is the Live
    // Editor's: the run then leaves it alone.
    key(e, ctx) {
      // The menu's own keys (editormenu.js) and its Insert flyout's.
      if (menu.root.contains(e.target)) return true;
      // Wherever focus is, so the run never asks for a route Undo here.
      if (chord(e) === 'u' && !ctx.inControl) return true;
      if (!lists || ed.readOnly()) return false;
      const a = document.activeElement;
      if (!grid || !(grid.contains(a) || a === document.body || !a)) return false;
      const cmd = gridCommand(e);
      if (cmd === 'undo' || cmd === 'redo') {
        e.preventDefault();
        ed.step(cmd === 'undo');
        return true;
      }
      if (cmd === 'copy' || cmd === 'cut') {
        e.preventDefault();
        if (kind()) lists.clip[cmd](KEY[kind()]);
        else tell(`Nothing ${cmd === 'copy' ? 'copied' : 'cut'}: pick sources or destinations first.`);
        return true;
      }
      // The browser's paste event reads the clipboard, so the key stays the browser's.
      if (cmd === 'paste') {
        reader?.key();
        return true;
      }
      if (!open() || ctx.inControl || ctx.line || (ctx.bar !== 'srcs' && ctx.bar !== 'dsts')) return false;
      if (menu.run(e)) {
        e.preventDefault();
        return true;
      }
      const c = chord(e);
      if (c === 'enter') {
        e.preventDefault();
        menu.focus();
        return true;
      }
      if (c === 'delete' || c === 'backspace') {
        e.preventDefault();
        removePicked();
        return true;
      }
      const mv = edgeListKey(e)?.move;
      if (mv && !following()) {
        e.preventDefault();
        list().move(mv);
        return true;
      }
      return false;
    },
    stop() {
      cancelAnimationFrame(frame);
      frame = 0;
      reader?.stop();
      grid?.removeEventListener('scroll', follow, { capture: true });
      document.removeEventListener('pointerdown', down, true);
      document.removeEventListener('pointerup', up, true);
      document.removeEventListener('pointercancel', up, true);
      window.removeEventListener('resize', later);
      menu.root.remove();
      run = null;
      grid = null;
    },
  };
}
