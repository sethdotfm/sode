// The Rights tab of Manage users: one entry at a time (a user, an API client or Guest), each list
// "All" or "Only these", plus Visibility. Keeps its own drafts; users.js reads them on save through
// rightsrules.accessReq. A factory taking hooks, so nothing imports users.js back.

import { el, S } from './core.js';
import { SCOPES } from './apirules.js';
import { draftOf, entryKeys, pickValid } from './rightsrules.js';

const LISTS = [
  { kind: 'sources', title: 'Sources', hint: 'Numbers as shown, with ranges: 1-8, 12.' },
  { kind: 'dsts', title: 'Destinations', hint: 'Numbers as shown, with ranges: 1-8, 12.' },
  { kind: 'levels', title: 'Levels', hint: 'Level numbers start at 1, whatever the router calls its levels.' },
  { kind: 'presets', title: 'Presets', hint: 'One preset name per line.', area: true },
];
const off = () => S.cfg?.numberOffset ?? 0;

const opt = (value, text) => Object.assign(el('option', null, text), { value });
const select = (id, options) => {
  const s = el('select', 'pb-select');
  s.id = id;
  s.append(...options);
  return s;
};
const field = (id, text, control, hint) =>
  el(
    'div',
    'sode__field',
    Object.assign(el('label', null, text), { htmlFor: id }),
    el('span', 'pb-select-wrap', control),
    ...(hint ? [el('span', 'sode__hint', hint)] : []),
  );

export function makeRightsTab({ users, clients, touched }) {
  const entries = new Map(); // 'user:<rid>' | 'client:<rid>' | 'guest' to {orig, draft, dirty}
  const was = new Map(); // user rid to the saved No auto-lock
  const flags = new Map(); // user rid to No auto-lock, only once changed here
  let guestScopes = null; // null until the directory loaded them
  let pick = '';

  const pickSel = select('rights-pick', []);
  const body = el('div');
  const node = el('div', null, el('div', 'sode__rights-pick', field('rights-pick', 'Rights for', pickSel)), body);

  const entryOf = (key) => {
    if (!entries.has(key)) entries.set(key, { orig: undefined, draft: draftOf({}, off()), dirty: false });
    return entries.get(key);
  };
  const keys = () => entryKeys(users(), clients());
  const titleOf = (key) => {
    const [kind, rid] = key.split(':');
    if (kind === 'user')
      return `User: ${
        users()
          .find((u) => String(u.rid) === rid)
          ?.name.trim() || '(new user)'
      }`;
    if (kind === 'client')
      return `API client: ${
        clients()
          .find((c) => String(c.rid) === rid)
          ?.name.trim() || '(new client)'
      }`;
    return 'Guest (a signed-out browser, and an API call with no key)';
  };

  function draw() {
    const e = entryOf(pick);
    const [kind, rid] = pick.split(':');
    const dirty = () => {
      e.dirty = true;
      touched();
    };
    const parts = [];
    for (const l of LISTS) {
      const d = e.draft[l.kind];
      const mode = select(`rights-${l.kind}-mode`, [opt('all', 'All'), opt('only', 'Only these')]);
      mode.value = d.all ? 'all' : 'only';
      const text = l.area ? el('textarea', 'pb-input') : el('input', 'pb-input');
      if (l.area) text.rows = 3;
      text.value = d.text;
      text.spellcheck = false;
      text.disabled = d.all;
      text.setAttribute('aria-label', `${l.title} allowed`);
      mode.addEventListener('change', () => {
        d.all = mode.value === 'all';
        text.disabled = d.all;
        dirty();
        if (!d.all) text.focus();
      });
      text.addEventListener('input', () => {
        d.text = text.value;
        dirty();
      });
      parts.push(
        el(
          'div',
          'sode__rights-list',
          el('div', 'sode__rights-head', Object.assign(el('label', null, l.title), { htmlFor: mode.id }), el('span', 'pb-select-wrap', mode)),
          text,
          el('span', 'sode__hint', l.hint),
        ),
      );
    }
    const vis = select('rights-visibility', [opt('readonly', 'Read-only'), opt('hidden', 'Hidden')]);
    vis.value = e.draft.visibility;
    vis.addEventListener('change', () => {
      e.draft.visibility = vis.value;
      dirty();
    });
    parts.push(
      field(
        'rights-visibility',
        'Visibility',
        vis,
        'Read-only shows everything and refuses commands outside these lists. Hidden also removes what is outside them from what this actor sees over the API. Browsers always see everything.',
      ),
    );
    if (kind === 'user') parts.push(flagBox(Number(rid), dirty));
    if (kind === 'guest') parts.push(scopeBoxes());
    body.replaceChildren(el('h3', 'sode__profiles-h', titleOf(pick)), ...parts);
  }

  const check = (text, checked, onChange) => {
    const input = el('input', 'pb-check__input');
    input.type = 'checkbox';
    input.checked = checked;
    input.addEventListener('change', () => onChange(input.checked));
    return el('label', 'pb-check', input, el('span', 'pb-check__label', text));
  };
  const flagBox = (rid, dirty) =>
    el(
      'div',
      'sode__field',
      check('No auto-lock', flags.has(rid) ? flags.get(rid) : !!was.get(rid), (v) => {
        flags.set(rid, v);
        dirty();
      }),
      el('span', 'sode__hint', 'Auto-lock skips this user. The Lock button still works for them.'),
    );
  const scopeBoxes = () =>
    el(
      'div',
      'sode__field',
      el('span', 'sode__field-label', 'Guest API scopes'),
      el(
        'div',
        'sode__scopes',
        ...SCOPES.map((s) =>
          check(s.text, !!guestScopes?.includes(s.id), (v) => {
            guestScopes = SCOPES.map((x) => x.id).filter((id) => (id === s.id ? v : guestScopes?.includes(id)));
            entryOf('guest').dirty = true;
            touched();
          }),
        ),
      ),
      el('span', 'sode__hint', 'What an API call with no key may use, when Guest API is on in Settings.'),
    );

  // Names and rows change under the tab: rebuild the list, keep the pick if its row is still there.
  function refresh() {
    const ks = keys();
    pick = pickValid(ks, pick);
    pickSel.replaceChildren(...ks.map((k) => opt(k, titleOf(k))));
    pickSel.value = pick;
    draw();
  }
  pickSel.addEventListener('change', () => {
    pick = pickSel.value;
    draw();
  });

  return {
    node,
    refresh,
    show(key) {
      pick = key;
      refresh();
    },
    reset() {
      entries.clear();
      flags.clear();
      was.clear();
      guestScopes = null;
    },
    adoptUser(rid, u) {
      entries.set(`user:${rid}`, { orig: u.rights || {}, draft: draftOf(u.rights, off()), dirty: false });
      was.set(rid, !!u.noAutoLock);
    },
    adoptClient(rid, c) {
      entries.set(`client:${rid}`, { orig: c.rights || {}, draft: draftOf(c.rights, off()), dirty: false });
    },
    adoptGuest(rights, scopes) {
      entries.set('guest', { orig: rights || {}, draft: draftOf(rights, off()), dirty: false });
      guestScopes = scopes || [];
    },
    read: () => ({ entries, flags, guestScopes }),
  };
}
