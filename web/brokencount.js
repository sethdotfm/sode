// Broken and Name moved counts: per router in Settings > Routers (an Administrator's covers everyone's items) and in
// My user for the user's own items. The numbers are the server's (brokenCount, internal/app/brokencount.go); the
// list under each count opens Swap or the item's editor.
import { $, el, emit, on, onMsg, wsSend } from './core.js';
import { profileName } from './profile.js';
import { statusBadge } from './brokenview.js';
import { editHref } from './panelsrules.js';

const KIND = { preset: 'Preset', filterSet: 'Filter set', panel: 'Panel' };
let role = '';
let seq = 0;
const asked = { all: 0, mine: 0 };

export const countText = (r) => `${r.broken} Broken, ${r.moved} Name moved`;

// id is the swapItem id: preset/<owner>/<id>, filterSet/<owner>/<name> or panel/<id>; owner "everyone" is Everyone.
function editOf(it) {
  const [kind, owner, ...rest] = it.id.split('/');
  if (kind === 'panel') return it.owner && it.owner === profileName() ? { href: editHref(owner) } : null;
  const mine = owner !== 'everyone' && it.owner === profileName();
  const everyone = owner === 'everyone' && (role === 'administrator' || role === 'trusted');
  if (!mine && !everyone) return null;
  const scope = mine ? 'user' : 'global';
  return { ev: kind === 'preset' ? 'editPresetAt' : 'editFilterSetAt', at: { scope, name: kind === 'preset' ? it.name : rest.join('/') } };
}

function row(it) {
  const acts = [];
  if (role === 'administrator' && it.swap) {
    const b = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Swap…');
    b.type = 'button';
    b.addEventListener('click', () => emit('swapAt', it.swap));
    acts.push(b);
  }
  const ed = editOf(it);
  if (ed?.href) {
    const a = el('a', 'pb-btn pb-btn--outline pb-btn--sm', 'Edit');
    a.href = ed.href;
    acts.push(a);
  } else if (ed) {
    const b = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Edit');
    b.type = 'button';
    b.addEventListener('click', () => emit(ed.ev, ed.at));
    acts.push(b);
  }
  const who = it.owner || 'Everyone';
  return el(
    'li',
    'sode__broken-row',
    statusBadge(it.status),
    el('span', null, `${KIND[it.kind] || it.kind}: ${it.name}`),
    el('span', 'sode__hint', who),
    ...acts,
  );
}

// box: the count line's holder. The list opens under it on a press of the count.
function paint(box, r) {
  if (!box) return;
  if (!r || (!r.broken && !r.moved)) {
    box.replaceChildren(el('span', 'sode__hint', 'No Broken or Name moved items'));
    return;
  }
  const list = el('ul', 'sode__broken-list', ...r.items.map(row));
  list.hidden = true;
  const toggle = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', countText(r));
  toggle.type = 'button';
  toggle.setAttribute('aria-expanded', 'false');
  toggle.addEventListener('click', () => {
    list.hidden = !list.hidden;
    toggle.setAttribute('aria-expanded', String(!list.hidden));
  });
  box.replaceChildren(toggle, list);
}

export function askBrokenCounts() {
  if (role === 'administrator') {
    asked.all = ++seq;
    wsSend({ t: 'brokenCount', id: asked.all });
  }
  if (profileName()) {
    asked.mine = ++seq;
    wsSend({ t: 'brokenCount', id: asked.mine, mine: true });
  }
}

onMsg('brokenCountResult', (m) => {
  if (!m.ok) return;
  const r = (m.routers || [])[0];
  if (m.id === asked.all) paint($('router-1-broken'), r);
  if (m.id === asked.mine) {
    const box = $('account-broken');
    if (box) box.hidden = false;
    paint(box, r);
  }
});

on('access', (a) => {
  role = a.role || '';
  const box = $('account-broken');
  if (box && !a.user) box.hidden = true;
  askBrokenCounts();
});
