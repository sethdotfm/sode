// The editor's Pages bar, left of the grid: a card per Page. Its title row holds the number, name and the On switch
// (asked for by the user, though the change still waits for Save). One row of icons below holds rename, Copy to,
// Move up, Move down and Delete Page. Cards are dragged to reorder, and Add Page ends the list. Rules are editorpagerules.js;
// orderedlist.js draws the rows' order controls; paneleditor.js holds the list and does each act.
import { el, icon } from './core.js';
import { btn } from './formbits.js';
import { confirmBox } from './confirmbox.js';
import { pageLabel, PAGE_MAX, PAGE_NAME_MAX } from './panelpages.js';
import { leadsTo } from './editorpagerules.js';
import { createOrderedList } from './orderedlist.js';

// act: open(id, multi), rename(i, text), enable(i, on), move(i, to), copyTo(i), remove(i), add().
export function createPageBar(host, act) {
  const list = el('ol', 'sode-pages__list');
  const add = btn('pb-btn--outline pb-btn--sm sode-pages__add', '', () => act.add());
  add.id = 'ed-page-add';
  add.append(icon('plus'), el('span', null, 'Add Page'));
  const full = el('p', 'sode__hint', `A panel holds at most ${PAGE_MAX} Pages.`);
  const ask = confirmBox(host);
  host.append(el('h2', 'sode-pages__title', 'Pages'), list, add, full, ask.node);
  let renaming = -1;
  let drawn = '';
  let last = null;
  const rows = createOrderedList({
    rowClass: 'sode-pages__row',
    toolsClass: 'sode-pages__tools',
    move: (from, to) => act.move(from, to),
    remove: (i) => remove(last.pages, i),
    removeTip: 'Delete Page',
  });

  function nameField(p, i) {
    const input = el('input', 'pb-input sode-pages__rename');
    input.maxLength = PAGE_NAME_MAX;
    input.value = p.name || '';
    input.placeholder = `Page ${i + 1}`;
    input.setAttribute('aria-label', `Name of Page ${i + 1}`);
    input.dataset.act = 'rename-field';
    let done = false;
    const finish = (keep) => {
      if (done) return;
      done = true;
      renaming = -1;
      if (keep) act.rename(i, input.value);
      else paint(last, true);
      host.querySelector(`[data-act="rename"][data-i="${i}"]`)?.focus();
    };
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') finish(true);
      else if (e.key === 'Escape') finish(false);
      else return;
      e.preventDefault();
      e.stopPropagation();
    });
    input.addEventListener('blur', () => finish(true));
    return input;
  }

  function row(p, i, s) {
    const n = s.pages.length;
    const isOpen = s.open.includes(p.id);
    const open = el(
      'button',
      'pb-btn pb-btn--ghost pb-btn--sm sode-pages__open',
      el('span', 'sode-pages__num', String(i + 1)),
      el('span', 'sode-pages__name', pageLabel(p, i)),
    );
    open.type = 'button';
    open.dataset.act = 'open';
    open.setAttribute('aria-pressed', String(isOpen));
    if (p.id === s.focus) open.setAttribute('aria-current', 'page');
    open.dataset.pbTip = 'Click to edit this Page. Cmd or Ctrl-click to open up to four side by side.';
    open.addEventListener('click', (e) => act.open(p.id, e.metaKey || e.ctrlKey));
    // On a Mac, Ctrl-click is a right click and never fires click.
    open.addEventListener('contextmenu', (e) => {
      if (!e.ctrlKey) return;
      e.preventDefault();
      act.open(p.id, true);
    });
    const on = el('input', 'pb-switch__input');
    on.type = 'checkbox';
    on.setAttribute('role', 'switch');
    on.checked = p.enabled !== false;
    on.disabled = s.readOnly;
    on.dataset.act = 'enable';
    on.setAttribute('aria-label', `Page ${i + 1} enabled`);
    // A refused change (the last enabled Page) does not always repaint the bar, so the switch is put back here.
    on.addEventListener('change', () => {
      if (act.enable(i, on.checked) === false) on.checked = true;
    });
    const head = el('div', 'sode-pages__head', open, el('label', 'pb-check sode-pages__on', on, el('span', 'pb-check__label', 'On')));
    const li = rows.row({
      i,
      n,
      id: p.id,
      readOnly: s.readOnly,
      body: [head, renaming === i ? nameField(p, i) : null],
      actions: [
        {
          icon: 'text-cursor-input',
          tip: 'Rename',
          act: 'rename',
          run: () => {
            renaming = i;
            paint(last, true);
          },
        },
        { icon: 'copy', tip: 'Copy to', act: 'copy', run: () => act.copyTo(i), off: n < 2 },
      ],
      removeOff: n < 2,
    });
    li.classList.toggle('is-open', isOpen);
    li.classList.toggle('is-focus', p.id === s.focus);
    li.classList.toggle('is-off', p.enabled === false);
    return li;
  }

  async function remove(pages, i) {
    const n = leadsTo(pages, pages[i].id);
    const cells = n ? ` ${n} Go to Page cell${n > 1 ? 's' : ''} lead${n > 1 ? '' : 's'} to it and will turn dashed.` : ' No Go to Page cell leads to it.';
    if (await ask.ask(`Delete Page ${i + 1}, ${pageLabel(pages[i], i)}, and its cells? Undo cannot bring it back.${cells}`, 'Delete')) act.remove(i);
  }

  // s: { pages, open, focus, readOnly }. Rows are redrawn only when what they show changed, and keyboard focus goes
  // back to the same control on the same Page.
  function paint(s, force = false) {
    last = s;
    const key = JSON.stringify([s.pages.map((p) => [p.id, p.name || '', p.enabled !== false, leadsTo(s.pages, p.id)]), s.open, s.focus, s.readOnly, renaming]);
    if (key === drawn && !force) return;
    drawn = key;
    const was = host.contains(document.activeElement) ? document.activeElement : null;
    const back = was && { act: was.dataset.act, id: was.closest('[data-id]')?.dataset.id };
    list.replaceChildren(...s.pages.map((p, i) => row(p, i, s)));
    add.disabled = s.readOnly || s.pages.length >= PAGE_MAX;
    full.hidden = s.pages.length < PAGE_MAX;
    host.querySelector('.sode-pages__rename')?.focus();
    if (!back?.act || host.contains(document.activeElement)) return;
    const li = [...list.children].find((x) => x.dataset.id === back.id);
    const target = li?.querySelector(`[data-act="${back.act}"]`);
    (target && !target.disabled ? target : li?.querySelector('[data-act="open"]'))?.focus();
  }

  return { paint, cancel: () => ask.cancel() };
}
