// Free Sode's Protects, by router (Settings > Routers > Protects). The dialog lists each router with the count of
// Sode's Protects there and a tick box. A router that is down or has Protect off shows why and cannot be ticked. The
// button says how many Protects on how many routers, behind the churning letter and the lockout. The server asks
// the PIN again (Critical). Rules in protectlistrules.js.

import { el, S } from './core.js';
import { btn, check } from './formbits.js';
import { question } from './modal.js';
import { churnKey, lockoutMs } from './lockrules.js';
import { freeLines, freeButton } from './protectlistrules.js';

// rows and routers as protectlistrules.js has them. only: the router UID to tick alone ('' ticks each one that can).
// Resolves with the routers ticked (protectlistrules.js routers), or null on Cancel.
export function askFreeSode(rows, routers, only, linked, back) {
  const lines = freeLines(rows, routers, only, linked);
  const ticked = new Set(lines.filter((l) => l.ticked).map((l) => l.uid));
  const key = churnKey();
  const keyEl = el('kbd', 'sode__key sode__key--locked', key.toUpperCase());
  let ready = false;
  const go = btn('sode-btn--primary', '', () => ready && !go.disabled && q.close([...ticked]));
  go.id = 'protects-free-go';
  const cancel = btn('pb-btn--outline', 'Cancel', () => q.close(null));
  const many = routers.length > 1;
  const list = el(
    'div',
    'sode__pfree-list',
    ...lines.map((l) => {
      const name = many ? `${l.router.prefix} ${l.router.title}` : l.router.title || 'Router';
      const c = check(name, {
        checked: l.ticked,
        onChange: (on) => {
          if (on) ticked.add(l.uid);
          else ticked.delete(l.uid);
          sync();
        },
      });
      c.input.disabled = !l.can;
      c.input.dataset.router = l.uid;
      const count = el('span', 'sode__pfree-count', l.count === 1 ? '1 Protect' : `${l.count} Protects`);
      return el('div', 'sode__pfree-row', c.label, count, l.why ? el('span', 'sode__hint', l.why) : null);
    }),
  );
  function sync() {
    const b = freeButton(lines, ticked);
    go.textContent = `${b.text} (${key.toUpperCase()})`;
    go.disabled = b.disabled || !ready;
  }
  const q = question({
    titleId: 'protects-free-title',
    title: "Free Sode's Protects?",
    rootId: 'protects-free',
    back,
    body: [
      el('p', null, 'Sode frees each Protect that it set, for every user, API client and panel. The Protects of other devices and Router locks stay.'),
      list,
      el('div', 'sode__prompt', 'Press', keyEl, 'to confirm'),
      el('p', 'sode__hint', 'Sode asks your PIN again after this step.'),
    ],
    foot: [cancel, go],
  });
  const lock = setTimeout(() => {
    ready = true;
    keyEl.classList.remove('sode__key--locked');
    sync();
  }, lockoutMs(S.cfg));
  sync();
  q.dialog.addEventListener('keydown', (e) => {
    if (e.ctrlKey || e.metaKey || e.altKey || e.key.toLowerCase() !== key) return;
    e.preventDefault();
    go.click();
  });
  q.dialog.focus();
  return q.answer.then((v) => {
    clearTimeout(lock);
    return v;
  });
}
