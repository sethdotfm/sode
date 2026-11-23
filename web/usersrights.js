// The Rights tab of Manage users: one entry at a time (a user, an API client or Guest), each list
// "All" or "Only these", plus Visibility. Keeps its own drafts; users.js reads them on save through
// rightsrules.accessReq. A factory that gets hooks, so nothing imports users.js back.

import { el, S } from './core.js';
import { entryKeys, newEntry, pickValid } from './rightsrules.js';
import { check, field, opt } from './formbits.js';

const LISTS = [
  { kind: 'sources', title: 'Sources', example: () => '1-8, 12' },
  { kind: 'dsts', title: 'Destinations', example: () => '1-8, 12' },
  { kind: 'levels', title: 'Levels', example: () => `1-${S.levels.length || 8}` },
  { kind: 'presets', title: 'Presets', hint: "One name per line (Everyone or the user's Me presets)", area: true },
];
const off = () => S.cfg?.numberOffset ?? 0;
// Rights name presets by id. The Everyone presets, and a user's own Me presets, give the names to type.
const presets = () => S.cfg?.presets || [];

const select = (id, options) => {
  const s = el('select', 'pb-select');
  s.id = id;
  s.append(...options);
  return s;
};
const selectField = (id, text, control, hint) => field(id, text, el('span', 'pb-select-wrap', control), hint);

export function makeRightsTab({ users, clients, touched }) {
  const entries = new Map(); // 'user:<rid>' | 'client:<rid>' | 'guest' to rightsrules' {orig, draft, dirty}
  let pick = '';

  const pickSel = select('rights-pick', []);
  const body = el('div');
  const node = el('div', null, el('div', 'sode__rights-pick', selectField('rights-pick', 'Rights for', pickSel)), body);

  const entryOf = (key) => {
    if (!entries.has(key))
      entries.set(key, newEntry(undefined, off(), key.startsWith('user:') ? { noAutoLock: !!S.cfg?.newUserDefaults?.noAutoLock } : {}, [], S.routerUid));
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
    return 'Guest';
  };

  function draw() {
    const e = entryOf(pick);
    const [kind] = pick.split(':');
    const dirty = (field) => {
      e.dirty.add(field);
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
      if (l.example) text.placeholder = l.example();
      text.disabled = d.all;
      text.setAttribute('aria-label', `${l.title} allowed`);
      mode.addEventListener('change', () => {
        d.all = mode.value === 'all';
        text.disabled = d.all;
        dirty('rights');
        if (!d.all) text.focus();
      });
      text.addEventListener('input', () => {
        d.text = text.value;
        dirty('rights');
      });
      parts.push(
        el(
          'div',
          'sode__rights-list',
          el('div', 'sode__rights-head', Object.assign(el('label', null, l.title), { htmlFor: mode.id }), el('span', 'pb-select-wrap', mode)),
          text,
          ...(l.hint ? [el('span', 'sode__hint', l.hint)] : []),
        ),
      );
    }
    const vis = select('rights-visibility', [opt('readonly', 'Read-only'), opt('hidden', 'Hidden from the API')]);
    vis.value = e.draft.visibility;
    vis.addEventListener('change', () => {
      e.draft.visibility = vis.value;
      dirty('rights');
    });
    parts.push(selectField('rights-visibility', 'Visibility', vis));
    // The Protect right: set and free Sode's Protects on the destinations above. Unset follows a user's Role or the
    // Guest role (GuestActs on the server); an API client has no Role, so unset is off.
    const unsetText = { user: 'From the Role (on for a Trusted user)', guest: 'From the Guest role (on for Trusted user)' };
    const prot = select('rights-protect', [opt('', unsetText[kind] || 'No'), opt('yes', 'Yes'), ...(unsetText[kind] ? [opt('no', 'No')] : [])]);
    prot.value = e.draft.protect;
    prot.addEventListener('change', () => {
      e.draft.protect = prot.value;
      dirty('rights');
    });
    parts.push(selectField('rights-protect', 'Protect', prot, "Set and free Sode's Protects"));
    if (kind === 'user') parts.push(flagBox(e, dirty));
    body.replaceChildren(el('h3', 'sode__profiles-h', titleOf(pick)), ...parts);
  }

  const flagBox = (e, dirty) =>
    el(
      'div',
      'sode__field',
      check('No auto-lock', {
        checked: e.draft.noAutoLock,
        onChange: (v) => {
          e.draft.noAutoLock = v;
          dirty('noAutoLock');
        },
      }).label,
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
    reset: () => entries.clear(),
    setNoAutoLock(rid, v) {
      const e = entryOf(`user:${rid}`);
      e.draft.noAutoLock = v;
      e.dirty.add('noAutoLock');
      if (pick === `user:${rid}`) draw();
    },
    adoptUser(rid, u) {
      entries.set(`user:${rid}`, newEntry(u.rights || {}, off(), { noAutoLock: !!u.noAutoLock }, [...presets(), ...(u.presets || [])], S.routerUid));
    },
    adoptClient(rid, c) {
      entries.set(`client:${rid}`, newEntry(c.rights || {}, off(), {}, presets(), S.routerUid));
    },
    adoptGuest(rights) {
      entries.set('guest', newEntry(rights || {}, off(), {}, presets(), S.routerUid));
    },
    read: () => entries,
  };
}
