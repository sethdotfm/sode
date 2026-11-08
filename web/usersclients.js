// The API clients tab of Manage users: a client's name and API scopes, and every key of every user
// and client (made, listed and revoked with the Advanced Settings PIN). Names and scopes save with the
// users' Save; keys act at once. A factory taking hooks, so nothing imports users.js back.

import { el, onMsg, wsSend, sentence } from './core.js';
import { SCOPES } from './apirules.js';
import { expiryFrom, keyNameProblem, keysByOwner, ownerText } from './keyrules.js';
import { keyRow, secretNode } from './keyview.js';

const MAX_CLIENTS = 12; // config.MaxAPIClients

const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};
const opt = (value, text) => Object.assign(el('option', null, text), { value });

export function makeClientsTab({ touched, adopt }) {
  const rows = el('div');
  const keyHost = el('div');
  const shown = el('div');
  const err = el('div', 'pb-note pb-note--error');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  let rid = 0;
  let seq = 0;
  let listSeq = 0;
  let owners = []; // the saved users and clients a key can be made for
  let open = false;

  const say = (t) => {
    err.textContent = sentence(t);
    err.hidden = !t;
  };

  function addRow(c = { name: '', scopes: [] }) {
    const id = ++rid;
    const name = el('input', 'pb-input');
    name.value = c.name;
    name.placeholder = 'Name';
    name.spellcheck = false;
    name.autocomplete = 'off';
    name.setAttribute('aria-label', 'API client name');
    name.dataset.field = 'client-name';
    name.addEventListener('input', touched);
    const scopes = el('div', 'sode__scopes');
    for (const s of SCOPES) {
      const input = el('input', 'pb-check__input');
      input.type = 'checkbox';
      input.checked = (c.scopes || []).includes(s.id);
      input.dataset.scope = s.id;
      input.addEventListener('change', touched);
      scopes.append(el('label', 'pb-check', input, el('span', 'pb-check__label', s.text)));
    }
    const row = el(
      'div',
      'sode__clients-row',
      el('div', 'sode__users-in', name),
      scopes,
      btn('pb-btn--ghost pb-btn--sm', 'Remove', () => {
        row.remove();
        touched();
      }),
    );
    row.dataset.rid = String(id);
    row.dataset.was = c.name ? c.name : '';
    rows.append(row);
    adopt?.(id, c);
    return name;
  }

  const read = () =>
    [...rows.children].map((row) => ({
      rid: Number(row.dataset.rid),
      was: row.dataset.was,
      name: row.querySelector('input[data-field="client-name"]').value,
      scopes: SCOPES.filter((s) => row.querySelector(`input[data-scope="${s.id}"]`).checked).map((s) => s.id),
    }));

  // ---- keys
  function paintKeys(keys) {
    const now = Date.now();
    const groups = keysByOwner(keys);
    keyHost.replaceChildren(
      ...(groups.length
        ? groups.flatMap((g) => [
            el(
              'div',
              'sode__keys-owner',
              el('strong', null, ownerText(g.owner)),
              btn('pb-btn--outline pb-btn--sm', 'Revoke all', () => {
                if (!wsSend({ t: 'keyRevokeAll', owner: g.owner, id: ++listSeq })) say('Not connected to Sode.');
              }),
            ),
            ...g.keys.map((k) =>
              keyRow(k, now, () => {
                if (!wsSend({ t: 'keyRevokeAny', keyId: k.id, id: ++listSeq })) say('Not connected to Sode.');
              }),
            ),
          ])
        : [el('p', 'sode__hint', 'No keys yet.')]),
    );
  }
  const load = () => {
    if (open) wsSend({ t: 'keyListAll', id: ++listSeq });
  };
  onMsg('keyListAllResult', (m) => m.ok && paintKeys(m.keys));
  for (const t of ['keyRevokeAnyResult', 'keyRevokeAllResult'])
    onMsg(t, (m) => {
      if (!m.ok) say(m.err);
      load();
    });

  const who = el('select', 'pb-select');
  who.id = 'clients-key-owner';
  const kname = el('input', 'pb-input');
  kname.id = 'clients-key-name';
  kname.placeholder = 'Panel 21';
  const kexp = el('input', 'pb-input');
  kexp.id = 'clients-key-expires';
  kexp.type = 'date';
  const make = btn('pb-btn--outline pb-btn--sm', 'Make key', () => {
    say('');
    const o = owners[who.selectedIndex];
    if (!o) return say('Save the API client first, then make its key.');
    const bad = keyNameProblem(kname.value);
    if (bad) return say(bad);
    const exp = expiryFrom(kexp.value, Date.now());
    if (exp.error) return say(exp.error);
    const id = ++seq;
    if (!wsSend({ t: 'keyCreateFor', id, owner: o, keyName: kname.value.trim(), expires: exp.ms })) return say('Not connected to Sode.');
    make.disabled = true;
  });
  onMsg('keyCreateForResult', (m) => {
    if (m.id !== seq) return;
    make.disabled = false;
    if (!m.ok) return say(m.err);
    kname.value = kexp.value = '';
    shown.replaceChildren(secretNode(m.key));
    load();
  });

  const labeled = (id, text, control) => el('div', 'sode__field', Object.assign(el('label', null, text), { htmlFor: id }), control);
  const node = el(
    'div',
    null,
    el(
      'p',
      'sode__hint',
      'An API client is an app or panel with its own name in History. It has its own API scopes (here) and its own Rights (Rights tab). Renaming a client ends its keys. A wall panel that must outlive PIN changes should use a client key, since a user key ends when that user changes their PIN.',
    ),
    rows,
    el(
      'div',
      'sode__users-acts',
      btn('pb-btn--outline pb-btn--sm', 'Add API client', () => {
        if (rows.children.length >= MAX_CLIENTS) return say(`At most ${MAX_CLIENTS} API clients.`);
        addRow().focus();
        touched();
      }),
    ),
    el('h3', 'sode__profiles-h', 'Keys'),
    el('p', 'sode__hint', 'Every key of every user and API client. A key is shown once when it is made, and Sode keeps only a hash of it.'),
    keyHost,
    el('h3', 'sode__profiles-h', 'Make a key'),
    el(
      'div',
      'sode__keys-new',
      labeled('clients-key-owner', 'For', el('span', 'pb-select-wrap', who)),
      labeled('clients-key-name', 'Key name', kname),
      labeled('clients-key-expires', 'Expires (optional)', kexp),
      el('div', null, make),
    ),
    shown,
    err,
  );

  return {
    node,
    read,
    addRow,
    clear: () => rows.replaceChildren(),
    count: () => rows.children.length,
    // users: the saved users' names, for the "Make a key for" list.
    setOwners(saved) {
      owners = saved;
      who.replaceChildren(...owners.map((o) => opt(o.name, ownerText(o))));
    },
    // The tab is on screen: list the keys, and forget a key shown before.
    shownNow(on_) {
      open = on_;
      if (!on_) shown.replaceChildren();
      else load();
    },
    forget: () => shown.replaceChildren(),
    say,
  };
}
