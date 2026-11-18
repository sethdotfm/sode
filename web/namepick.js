// The sign-in name box: fuzzy find over the hello's users and a list of every user. lock.js owns the
// keys (its capture handler runs first), so it passes the box's keydowns to key().
import { el } from './core.js';
import { nameMatches, pickName } from './namepickrules.js';

// users() is read live, so a hello that changes the list shows the next time the list paints.
export function namePicker({ id, users, onInput, onPick }) {
  const input = el('input', 'pb-input sode-lock__name');
  input.id = id;
  input.type = 'text';
  input.autocomplete = 'username';
  input.autocapitalize = 'none';
  input.spellcheck = false;
  input.maxLength = 24;
  input.setAttribute('role', 'combobox');
  input.setAttribute('aria-autocomplete', 'list');
  input.setAttribute('aria-expanded', 'false');
  const list = el('ul', 'pb-menu sode-namepick__list');
  list.id = `${id}-list`;
  list.setAttribute('role', 'listbox');
  list.setAttribute('aria-label', 'Users');
  list.hidden = true;
  input.setAttribute('aria-controls', list.id);
  const root = el('div', 'sode-namepick', input, list);

  let matches = [];
  let hi = 0;

  function paint() {
    list.hidden = matches.length === 0;
    input.setAttribute('aria-expanded', String(!list.hidden));
    list.replaceChildren(
      ...matches.map((u, i) => {
        const li = el('li', `pb-menu__item${i === hi ? ' active' : ''}`, u.name);
        li.id = `${list.id}-${i}`;
        li.dataset.i = String(i);
        li.setAttribute('role', 'option');
        li.setAttribute('aria-selected', String(i === hi));
        return li;
      }),
    );
    if (list.hidden) input.removeAttribute('aria-activedescendant');
    else {
      input.setAttribute('aria-activedescendant', `${list.id}-${hi}`);
      list.children[hi]?.scrollIntoView?.({ block: 'nearest' });
    }
  }

  function open() {
    matches = nameMatches(users(), input.value);
    hi = 0;
    paint();
  }

  function close() {
    matches = [];
    paint();
  }

  function pick(name) {
    if (!name) return false;
    input.value = name;
    close();
    onInput();
    onPick(name);
    return true;
  }

  input.addEventListener('input', () => {
    open();
    onInput();
  });
  input.addEventListener('focus', open);
  input.addEventListener('blur', close);
  // pointerdown keeps the focus in the box, so blur does not close the list before the click.
  list.addEventListener('pointerdown', (e) => e.preventDefault());
  list.addEventListener('mousedown', (e) => e.preventDefault());
  list.addEventListener('click', (e) => {
    const li = e.target.closest?.('[role="option"]');
    if (li) pick(matches[+li.dataset.i]?.name);
  });

  // True when the key was the box's: arrows move in the list, Enter picks, Escape closes an open list.
  function key(e) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      if (list.hidden) open();
      else if (matches.length) hi = (hi + (e.key === 'ArrowDown' ? 1 : matches.length - 1)) % matches.length;
      paint();
      return true;
    }
    if (e.key === 'Enter') return pick(matches[hi]?.name || pickName(users(), input.value)) || true;
    if (e.key === 'Escape' && !list.hidden) {
      close();
      return true;
    }
    return false;
  }

  return {
    root,
    input,
    key,
    close,
    value: () => input.value,
    set(v) {
      input.value = v;
      if (document.activeElement === input) open();
      else close();
    },
    // A new hello: an open list shows the new names.
    refresh() {
      if (!list.hidden) open();
    },
  };
}
