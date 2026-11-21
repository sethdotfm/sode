// The phone's History rows: swipe left to show Undo, right to show Redo (Retry for failed takes).
import { $, el, S, on, dstItem, srcItem, labelNode, toggle } from './core.js';
import { levelBar } from './levelview.js';
import { fmtTime } from './device.js';
import { entryNote, historyPager, statusNode, groupTitle, groupLevels, stackBefore } from './history.js';
import { layout } from './layout.js';
import { emptyNote } from './searchbox.js';
import { rowActions, disclosureAria, followersHead } from './historyrows.js';
import { protectRow } from './protects.js';
import { picking, isPicked, pickBox, bindPick, pickBar } from './historypick.js';
import { byLabel, disclosure, undoButton, againButton, arrivals, stackSteps, clickToToggle, pullToLoad } from './widgets.js';

const REVEAL = 96;
const OPEN = 'sode-m__recent-row--open';
const OPEN_R = 'sode-m__recent-row--open-r';

export function closeSwipes(except) {
  for (const r of document.querySelectorAll(`.${OPEN}, .${OPEN_R}`)) {
    if (r === except) continue;
    r.classList.remove(OPEN, OPEN_R);
    r.querySelector('.sode-m__recent-face').style.transform = '';
  }
}

// sheetFor(plan) opens the Confirm sheet; pickSrc(n) puts a source in the composer.
export function mobileHistory({ sheetFor, pickSrc }) {
  const arrived = arrivals();

  const openGroups = new Set();
  const openFollowers = new Set();
  const openChanges = new Set();

  const undoBtn = (plan, label, text = 'Undo') => undoButton(plan, label, sheetFor, { text, cls: 'sode-m__undo' });
  const redoBtn = (plan, label) => againButton(plan, label, sheetFor, { placeholder: false, cls: 'sode-m__redo' });

  function historyRow(e) {
    const src = srcItem(e.src, e.srcName),
      dst = dstItem(e.dst, e.dstName);
    const acts = rowActions({ e });
    const undo = undoBtn(acts.undo.plan, acts.undo.label, acts.undo.text);
    const status = statusNode({ e });
    const retry = redoBtn(acts.again.plan, acts.again.label);
    const open = !!e.chain && openChanges.has(e.id);
    const changesId = `m-changes-${e.id}`;
    const disc =
      e.chain &&
      disclosure(open, 'Changes', changesId, () => {
        toggle(openChanges, e.id);
        renderHistory();
      });
    let changes = null;
    if (open) {
      changes = el('div', 'sode-m__changes', ...stackSteps(e, { restore: sheetFor, pickSrc }));
      changes.id = changesId;
    }
    const face = el(
      'div',
      'sode-m__recent-face',
      el(
        'div',
        'sode-m__recent-top',
        el('span', 'sode__hist-time', fmtTime(e.time)),
        el('span', 'sode-m__recent-status', status, ...(picking() ? [pickBox(`e${e.id}`)] : []), disc),
      ),
      el(
        'div',
        'sode-m__recent-route',
        ...(e.chain ? [el('span', 'sode__hist-note', stackBefore(e).text), ' to '] : []),
        e.protect ? el('span', 'pb-badge pb-badge--warn', protectRow(e).text) : el('strong', null, labelNode(src)),
        ' → ',
        el('strong', null, labelNode(dst)),
        entryNote(e),
        byLabel(e),
      ),
      changes,
      e.follows && !open ? el('div', 'sode__hint', `External follow: ${[...new Set(e.follows.map((f) => dstItem(f.dst, f.dstName).label))].join(', ')}`) : null,
      levelBar(e.levels.filter((u) => S.levelBy.has(u))),
    );
    if (disc) clickToToggle(face, disc, '.sode-m__changes');
    const row = el(
      'li',
      'sode-m__recent-row' + (arrived.entry(e) ? ' sode--enter' : '') + (isPicked(`e${e.id}`) ? ' sode-m__recent-row--picked' : ''),
      undo,
      ...(retry ? [retry] : []),
      face,
    );
    row.dataset.key = `e${e.id}`;
    swipeable(row, face, !!retry);
    return row;
  }

  // A Group: swipe to undo all. Open it to act on one take.
  function groupRows(f) {
    const open = openGroups.has(f.group);
    const kidsId = `m-folder-${f.group}`;
    const acts = rowActions({ f });
    const undo = undoBtn(acts.undo.plan, acts.undo.label, acts.undo.text);
    const retry = redoBtn(acts.again.plan, acts.again.label);
    const disc = disclosure(open, `${f.shown.length}`, kidsId, () => {
      toggle(openGroups, f.group);
      renderHistory();
    });
    disc.setAttribute('aria-label', disclosureAria(open, f.shown.length, 'takes'));

    const face = el(
      'div',
      'sode-m__recent-face',
      el(
        'div',
        'sode-m__recent-top',
        el('span', 'sode__hist-time', fmtTime(f.entries[0].time)),
        el('span', 'sode-m__recent-status', statusNode({ f }), ...(picking() ? [pickBox(`f${f.group}`)] : []), disc),
      ),
      el('div', 'sode-m__recent-route', ...groupTitle(f), byLabel(f)),
      levelBar(groupLevels(f)),
    );
    clickToToggle(face, disc);
    const row = el(
      'li',
      'sode-m__recent-row' + (arrived.group(f, open) ? ' sode--enter' : '') + (isPicked(`f${f.group}`) ? ' sode-m__recent-row--picked' : ''),
      undo,
      ...(retry ? [retry] : []),
      face,
    );
    row.dataset.key = `f${f.group}`;
    swipeable(row, face, !!retry);
    if (!open) return [row];
    const kids = el('li', 'sode-m__recent-kids', el('ul', 'sode-m__recent-list', ...f.shown.map(historyRow), ...followerRows(f)));
    kids.id = kidsId;
    return [row, kids];
  }

  function followerRows(f) {
    if (!f.followers?.length) return [];
    const open = openFollowers.has(f.group);
    const { count, aria, kidsId } = followersHead(f, open, 'm');
    const disc = disclosure(open, count, kidsId, () => {
      toggle(openFollowers, f.group);
      renderHistory();
    });
    disc.setAttribute('aria-label', aria);
    const face = el(
      'div',
      'sode-m__recent-face',
      el('div', 'sode-m__recent-top', el('strong', null, 'External follow entries'), el('span', 'sode-m__recent-status', disc)),
    );
    clickToToggle(face, disc);
    const head = el('li', 'sode-m__recent-row', face);
    if (!open) return [head];
    const kids = el('li', 'sode-m__recent-kids', el('ul', 'sode-m__recent-list', ...f.followers.map(historyRow)));
    kids.id = kidsId;
    return [head, kids];
  }

  function swipeable(row, face, canRedo = false) {
    let x0 = null,
      y0 = 0,
      dx = 0,
      base = 0,
      horiz = null;
    const set = (x) => {
      face.style.transform = `translateX(${x}px)`;
    };
    const rest = () => (row.classList.contains(OPEN) ? -REVEAL : row.classList.contains(OPEN_R) ? REVEAL : 0);
    face.addEventListener('pointerdown', (e) => {
      if (picking()) return;
      x0 = e.clientX;
      y0 = e.clientY;
      dx = 0;
      horiz = null;
      delete face.dataset.noTap; // a drag does not always end in a click
      base = rest();
    });
    face.addEventListener('pointermove', (e) => {
      if (x0 == null) return;
      const mx = e.clientX - x0,
        my = e.clientY - y0;
      if (horiz == null && Math.abs(mx) + Math.abs(my) > 8) {
        horiz = Math.abs(mx) > Math.abs(my);
        if (horiz) face.setPointerCapture(e.pointerId);
      }
      if (!horiz) return;
      dx = mx;
      face.classList.add('sode-m__recent-face--drag');
      set(Math.min(canRedo ? REVEAL * 1.3 : 0, Math.max(-REVEAL * 1.3, base + dx)));
    });
    const end = () => {
      if (x0 == null) return;
      face.classList.remove('sode-m__recent-face--drag');
      const at = base + dx;
      const side = !horiz ? 0 : at < -REVEAL / 2 ? -1 : canRedo && at > REVEAL / 2 ? 1 : 0;
      if (horiz || base) face.dataset.noTap = '1';
      if (!horiz && base) {
        closeSwipes();
        x0 = null;
        return;
      }
      if (horiz) {
        closeSwipes(row);
        row.classList.toggle(OPEN, side < 0);
        row.classList.toggle(OPEN_R, side > 0);
      }
      set(rest());
      x0 = null;
    };
    face.addEventListener('pointerup', end);
    face.addEventListener('pointercancel', end);
  }

  const pager = historyPager(5, () => layout() === 'phone');
  pullToLoad($('m-thread'), $('m-recent-more'), pager, () => layout() === 'phone' && !$('m-recent').hidden);

  bindPick($('m-recent-list'));
  // The pick bar sits under the header and pushes the thread down; on other layouts it floats over the page
  // (history.css).
  function placeBar() {
    if (layout() !== 'phone') return void (pickBar.parentNode !== document.body && document.body.append(pickBar));
    if (pickBar.previousElementSibling !== $('m-result')) $('m-result').after(pickBar);
  }
  on('pick', () => {
    placeBar();
    renderHistory();
  });

  function renderHistory() {
    const rows = pager.list();
    $('m-recent-list').replaceChildren(...rows.flatMap((r) => (r.f ? groupRows(r.f) : [historyRow(r.e)])));
    $('m-recent-more').replaceChildren(...[emptyNote(rows, pager.fetching()), pager.button()].filter(Boolean));
  }

  return { pager, placeBar, renderHistory };
}
