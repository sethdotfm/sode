// Names and scopes save with Settings' one Save; keys act at once. A factory with hooks, so nothing imports users.js back.

import { el, on, onMsg, wsSend, sentence, LINK_DOWN } from './core.js';
import { SCOPES } from './apirules.js';
import { expiryFrom, keyNameProblem, keysByOwner, ownerText } from './keyrules.js';
import { keyRow, secretNode } from './keyview.js';
import { btn, labeled, opt } from './formbits.js';
import { askRecheck } from './recheck.js';

const MAX_CLIENTS = 12; // config.MaxAPIClients

// users: the Users tab's rows, {id, name, was}.
export function makeClientsTab({ users, touched, adopt }) {
  const rows = el('div');
  const keyHost = el('div', 'sode__key-list');
  const shown = el('div');
  const err = el('div', 'pb-note pb-note--error');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  let rid = 0;
  let seq = 0;
  let listSeq = 0;
  let owners = []; // the saved users and clients the For list shows
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
    // A user's own client keeps its name, or the save would make a new client beside it.
    if (c.ownerName) {
      name.readOnly = true;
      name.dataset.pbTip = `Made by ${c.ownerName} from My user`;
    }
    const scopes = el('div', 'sode__scopes');
    for (const s of SCOPES) {
      const input = el('input', 'pb-check__input');
      input.type = 'checkbox';
      input.checked = (c.scopes || []).includes(s.id);
      input.dataset.scope = s.id;
      input.addEventListener('change', touched);
      scopes.append(el('label', 'pb-check', input, el('span', 'pb-check__label', s.text)));
    }
    // The save keeps a user's own client that is left out, so it gets no Remove.
    const remove = c.ownerName
      ? []
      : [
          btn('pb-btn--ghost pb-btn--sm', 'Remove', () => {
            row.remove();
            touched();
          }),
        ];
    const row = el('div', 'sode__clients-row', el('div', 'sode__users-in', name), scopes, ...remove);
    // Panel clients and users' own clients are outside MAX_CLIENTS.
    if (c.ownerName || c.panel) row.dataset.kept = '';
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
              btn('pb-btn--outline pb-btn--sm', 'Revoke all', () => revokeAll(g.owner)),
            ),
            ...g.keys.map((k) =>
              keyRow(k, now, () => {
                if (!wsSend({ t: 'keyRevokeAny', keyId: k.id, id: ++listSeq })) say(LINK_DOWN);
              }),
            ),
          ])
        : [el('p', 'sode__hint', 'No keys yet.')]),
    );
  }
  let revoking = null; // { id, owner } of the Revoke all in flight
  function revokeAll(owner) {
    revoking = { id: ++listSeq, owner };
    if (!wsSend({ t: 'keyRevokeAll', owner, id: revoking.id })) say(LINK_DOWN);
  }
  const load = () => {
    if (open) wsSend({ t: 'keyListAll', id: ++listSeq });
  };
  onMsg('keyListAllResult', (m) => m.ok && paintKeys(m.keys));
  onMsg('keyRevokeAllResult', async (m) => {
    const was = revoking?.id === m.id ? revoking : null;
    revoking = null;
    if (!m.ok && m.recheck && was && (await askRecheck('Revoking all keys'))) return revokeAll(was.owner);
    if (!m.ok) say(m.err);
    load();
  });
  onMsg('keyRevokeAnyResult', (m) => {
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
  const make = btn('sode-btn--primary', 'Make key', () => {
    say('');
    syncOwners();
    const o = owners[who.selectedIndex];
    if (!o) return say('Save the API client first, then make its key.');
    const bad = keyNameProblem(kname.value);
    if (bad) return say(bad);
    const exp = expiryFrom(kexp.value, Date.now());
    if (exp.error) return say(exp.error);
    const id = ++seq;
    if (!wsSend({ t: 'keyCreateFor', id, owner: o, keyName: kname.value.trim(), expires: exp.ms })) return say(LINK_DOWN);
    make.disabled = true;
  });
  on('wsclose', () => {
    if (!open || !make.disabled) return;
    make.disabled = false;
    say(LINK_DOWN);
  });
  onMsg('keyCreateForResult', (m) => {
    if (m.id !== seq || !open) return;
    make.disabled = false;
    if (!m.ok) return say(m.err);
    kname.value = kexp.value = '';
    shown.replaceChildren(secretNode(m.key));
    load();
  });

  // Only saved rows can own a key, and rows change under the tab, so the list is rebuilt from them on every read.
  function syncOwners() {
    owners = [
      ...users()
        .filter((u) => u.id)
        .map((u) => ({ kind: 'user', user: u.id, name: u.was })),
      ...read()
        .filter((c) => c.was)
        .map((c) => ({ kind: 'client', name: c.was })),
    ];
    const keep = who.value;
    const val = (o) => `${o.kind}:${o.user || o.name}`;
    who.replaceChildren(...owners.map((o) => opt(val(o), ownerText(o))));
    if (owners.some((o) => val(o) === keep)) who.value = keep;
  }
  who.addEventListener('focus', syncOwners);
  who.addEventListener('pointerdown', syncOwners);
  // Enter in the key form makes the key; the dialog's Enter saves the Users panel only.
  for (const i of [who, kname, kexp])
    i.addEventListener('keydown', (e) => {
      if (e.key !== 'Enter') return;
      e.preventDefault();
      make.click();
    });
  const node = el(
    'div',
    null,
    el(
      'p',
      'sode__hint',
      'An API client is an app or panel with its own name in History. It has its own API scopes (here) and its own Rights (Rights tab). If you rename a client, its keys end. A user key ends when that user changes their PIN. If a wall panel must work after PIN changes, give it a client key.',
    ),
    rows,
    el(
      'div',
      'sode__users-acts',
      btn('pb-btn--outline', 'Add API client', () => {
        if (rows.querySelectorAll(':scope > :not([data-kept])').length >= MAX_CLIENTS) return say(`At most ${MAX_CLIENTS} API clients.`);
        addRow().focus();
        touched();
      }),
    ),
    el('h3', 'sode__profiles-h', 'All keys'),
    keyHost,
    el('h3', 'sode__profiles-h', 'Make a key'),
    el(
      'div',
      'sode__keys-new',
      labeled('clients-key-owner', 'For', el('span', 'pb-select-wrap', who)),
      labeled('clients-key-name', 'Key name', kname),
      labeled('clients-key-expires', 'Expires', kexp),
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
    // The tab is on screen: list the keys, and forget a key shown before.
    shownNow(on_) {
      open = on_;
      if (!on_) {
        shown.replaceChildren();
        seq++; // a key made meanwhile has nowhere to show
        make.disabled = false;
      } else {
        syncOwners();
        load();
      }
    },
    forget: () => shown.replaceChildren(),
    // A new client saved: its row is now the saved client of that name.
    saved(name) {
      const row = [...rows.children].find((r) => !r.dataset.was && r.querySelector('input[data-field="client-name"]').value.trim() === name);
      if (row) row.dataset.was = name;
    },
  };
}
