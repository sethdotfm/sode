// Dialog Esc handling; no imports, shared by widgets.js and the lock chip.
// Only the newest open dialog handles Esc; open-ness is read from root's `hidden` at Esc time, so a dialog hidden elsewhere is skipped.
const stack = []; // { root, onEsc, order }
let shows = 0;
let listening = false;

export function topDialog(entries) {
  let top = null;
  for (const e of entries) {
    if (e.root.hidden || !e.root.isConnected) continue;
    if (!top || e.order > top.order) top = e;
  }
  return top;
}

// Close only when mousedown and mouseup both hit the backdrop: a press released inside, or a selection dragged out, fires click on root.
export function backdropClose(downTarget, upTarget, root) {
  return downTarget === root && upTarget === root;
}

// Scrims share one z-index, so moving root last in its parent puts it in front; no-op if already last (keeps focus and scroll).
export function bringToFront(root) {
  const parent = root.parentNode;
  if (!parent || parent.lastElementChild === root) return;
  const had = root.contains(document.activeElement) ? document.activeElement : null;
  parent.appendChild(root);
  if (had && document.activeElement !== had) had.focus({ preventScroll: true });
}

function listen() {
  if (listening) return;
  listening = true;
  // Capture, so it runs before page key handlers. Only keys outside the top dialog are taken;
  // inside, a field's own Esc still comes first.
  document.addEventListener(
    'keydown',
    (e) => {
      if (e.key !== 'Escape') return;
      const top = topDialog(stack);
      if (!top || top.root.contains(e.target)) return;
      e.preventDefault();
      e.stopImmediatePropagation();
      top.onEsc();
    },
    true,
  );
}

// How many dialogs the Esc stack holds, for the tests.
export const heldDialogs = () => stack.length;

// Stopped at root so the layouts' own keys don't act behind it. The stack holds a dialog only while it is shown:
// a modal built per open is removed from the page on close, and a held root would keep its whole tree alive.
export function trapDialog(root, dialog, onEsc) {
  const entry = { root, onEsc, order: 0 };
  stack.push(entry);
  listen();
  let down = null,
    up = null;
  root.addEventListener('mousedown', (e) => {
    down = e.target;
    up = null;
  });
  root.addEventListener('mouseup', (e) => {
    up = e.target;
  });
  root.addEventListener('click', (e) => {
    const d = down,
      u = up;
    down = up = null;
    if (e.target === root && backdropClose(d, u, root)) onEsc();
  });
  root.addEventListener('keydown', (e) => {
    e.stopPropagation();
    if (e.key === 'Escape') {
      e.preventDefault();
      onEsc();
      return;
    }
    if (e.key !== 'Tab') return;
    const all = [...dialog.querySelectorAll('button, input, select')].filter((x) => !x.disabled && x.offsetParent);
    const first = all[0],
      last = all[all.length - 1];
    if (e.shiftKey && (document.activeElement === first || document.activeElement === dialog)) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  });
  return {
    show() {
      root.hidden = false;
      bringToFront(root);
      entry.order = ++shows;
      if (!stack.includes(entry)) stack.push(entry);
    },
    hide() {
      root.hidden = true;
      const i = stack.indexOf(entry);
      if (i >= 0) stack.splice(i, 1);
    },
  };
}
