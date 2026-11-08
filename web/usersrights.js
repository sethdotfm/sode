// The Rights tab of Manage users: one entry at a time (a user, an API client or Guest), each list
// "All" or "Only these", plus Visibility. Keeps its own drafts; users.js reads them on save through
// rightsrules.accessReq. A factory taking hooks, so nothing imports users.js back.

import { el, S } from './core.js';
import { SCOPES } from './apirules.js';
import { entryKeys, newEntry, pickValid } from './rightsrules.js';
import { check, field, opt } from './formbits.js';

const LISTS = [
  { kind: 'sources', title: 'Sources', hint: 'Numbers as shown, with ranges: 1-8, 12.' },
  { kind: 'dsts', title: 'Destinations', hint: 'Numbers as shown, with ranges: 1-8, 12.' },
  { kind: 'levels', title: 'Levels', hint: 'Level numbers start at 1, whatever the router calls its levels.' },
  { kind: 'presets', title: 'Presets', hint: 'One preset name per line.', area: true },
];
const off = () => S.cfg?.numberOffset ?? 0;

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
      entries.set(key, newEntry(undefined, off(), key === 'guest' ? { scopes: [] } : key.startsWith('user:') ? { noAutoLock: false } : {}));
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
          el('span', 'sode__hint', l.hint),
        ),
      );
    }
    const vis = select('rights-visibility', [opt('readonly', 'Read-only'), opt('hidden', 'Hidden')]);
    vis.value = e.draft.visibility;
    vis.addEventListener('change', () => {
      e.draft.visibility = vis.value;
      dirty('rights');
    });
    parts.push(
      selectField(
        'rights-visibility',
        'Visibility',
        vis,
        'Read-only shows everything and refuses commands outside these lists. Hidden also removes what is outside them from what this actor sees over the API. Browsers always see everything.',
      ),
    );
    if (kind === 'user') parts.push(flagBox(e, dirty));
    if (kind === 'guest') parts.push(scopeBoxes(e, dirty));
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
      el('span', 'sode__hint', 'Auto-lock skips this user. The Lock button still works for them.'),
    );
  const scopeBoxes = (e, dirty) =>
    el(
      'div',
      'sode__field',
      el('span', 'sode__field-label', 'Guest API scopes'),
      el(
        'div',
        'sode__scopes',
        ...SCOPES.map(
          (s) =>
            check(s.text, {
              checked: e.draft.scopes.includes(s.id),
              onChange: (v) => {
                e.draft.scopes = SCOPES.map((x) => x.id).filter((id) => (id === s.id ? v : e.draft.scopes.includes(id)));
                dirty('scopes');
              },
            }).label,
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
    reset: () => entries.clear(),
    adoptUser(rid, u) {
      entries.set(`user:${rid}`, newEntry(u.rights || {}, off(), { noAutoLock: !!u.noAutoLock }));
    },
    adoptClient(rid, c) {
      entries.set(`client:${rid}`, newEntry(c.rights || {}, off()));
    },
    adoptGuest(rights, scopes) {
      entries.set('guest', newEntry(rights || {}, off(), { scopes: scopes || [] }));
    },
    read: () => entries,
  };
}
