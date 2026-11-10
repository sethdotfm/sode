// The sidebar's Access tab: who the panel acts as, the Rights limits of its own API client, and sharing.
// The server decides who may set up a client (ADR 0006): a Trusted user, within their own Rights.
// Sharing is the Share modal's job (panelshare.js), opened from the toolbar's Share button.
import { $, el, on } from './core.js';
import { sessionFetch, jsonCall } from './sessionkey.js';
import { check, segmented } from './formbits.js';
import { openShare } from './panelshare.js';
import { shareLabel, hasClient, actsSwitch } from './panelsrules.js';
import { confirmBox } from './confirmbox.js';
import { guardActs } from './actsguard.js';
import { draftOf, rightsOf } from './rightsrules.js';
import { rightsRows } from './sidebarrules.js';

const LISTS = [
  ['sources', 'Sources', 'Router numbers, such as 1-8, 12'],
  ['dsts', 'Destinations', 'Router numbers, such as 1-8, 12'],
  ['levels', 'Levels', 'Level numbers, such as 1, 2'],
  ['presets', 'Presets', 'One name per line'],
];

const SAVED_NOTE = {
  api: 'Saved. The panel now has its own API client. Make its links under Share, Link with key.',
  both: 'Saved. The panel has its own API client for links with a key, and the plain link opens it as the signed-in user.',
  session: 'Saved. The panel acts as the signed-in user, and its API client and every key are gone.',
};

const api = jsonCall(sessionFetch, { failed: (s) => `Sode answered ${s}` });

let me = { user: '', trusted: false };
on('access', (a) => (me = { user: a.user || '', trusted: !!a.trusted && !!a.user }));

// offset is the router's numberOffset and levelCount how many levels exist, for typing Rights lists.
export function createAccessTab(ed, ctx) {
  const node = el('div', 'sode-side__tab');
  const acts = segmented(
    'ed-acts',
    'Authentication mode',
    [
      ['session', 'User'],
      ['both', 'Both'],
      ['api', 'API'],
    ],
    async (want) => {
      // A switch that makes or ends the client waits for the Confirm (actsguard.js). A newer choice cancels an open one,
      // and Cancel puts the old mode back.
      guard.cancel();
      const was = ed.fields().acts;
      const sw = actsSwitch(was, want);
      if (!sw) {
        asking = null;
        acts.set(was || 'session');
        return;
      }
      asking = want;
      askedFor = ed.panel()?.id;
      actsNote.textContent = '';
      const go = await guardActs(guard, was, want);
      if (asking !== want) return;
      asking = null;
      if (!go) {
        acts.set(ed.fields().acts);
        return;
      }
      // Acts as makes or removes the panel's API client, so it is saved at once and never left as an unsaved edit. The
      // save carries the panel's other unsaved changes with it. A refusal puts the choice back and says why.
      acts.disable(true);
      actsNote.textContent = 'Saving.';
      ed.setFields({ acts: want });
      const refused = await ed.save();
      if (refused) ed.setFields({ acts: undefined });
      // A switch with no Confirm still says what it kept.
      actsNote.textContent = refused || (sw.confirm ? SAVED_NOTE[want] : `Saved. ${sw.text}`);
    },
  );
  let asking = null; // the mode an open Confirm is for; update() shows it until the answer
  let askedFor; // the panel it was asked on: opening another panel cancels it
  const guard = confirmBox(node);
  const actsHint = el('p', 'sode__hint');
  const actsNote = el('p', 'sode__hint');
  actsNote.setAttribute('role', 'status');
  const actsField = el('div', 'sode__field', el('span', 'sode__field-label', 'Authentication mode'), acts.node, actsHint, guard.node, actsNote);

  const clientBox = el('div', 'sode-side__sec');
  const current = el('p', 'sode-side__rights');
  const table = el('table', 'pb-table pb-table--compact sode-side__table');
  const limit = check("Limit this panel's access", { id: 'ed-limit' });
  const note = el('p', 'sode__hint');
  note.setAttribute('role', 'status');
  const lists = LISTS.map(([kind, name, hint]) => {
    const all = check(`Only some ${name}`, { id: `ed-rl-${kind}` });
    const text = el(kind === 'presets' ? 'textarea' : 'input', 'pb-input');
    text.setAttribute('aria-label', `${name} allowed`);
    text.placeholder = hint;
    if (kind === 'presets') text.rows = 3;
    return { kind, only: all.input, text, node: el('div', 'sode__field', all.label, text) };
  });
  let draft = draftOf(null, 0); // the Rights lists as typed; read back into clientRights on each change
  const hidden = check('Hide what the client may not use', { id: 'ed-rl-hidden' });
  const limits = el('div', 'sode-side__form', ...lists.map((l) => l.node), hidden.label);
  limits.hidden = true;
  clientBox.append(current, table, limit.label, limits, note);

  const shareLine = el('p', 'sode__hint');
  const shareList = el('ul', 'sode-side__people');
  const shareBtn = $('ed-share'); // in the toolbar
  shareBtn.addEventListener('click', () => {
    openShare(ed.panel(), { api, onSaved: (p) => ed.replaceLoaded(p) });
  });
  const share = el('section', 'sode-side__sec', el('h3', 'sode-side__h', 'Sharing'), shareLine, shareList);
  node.append(el('section', 'sode-side__sec', actsField), clientBox, share);

  // Typing a list sends the Rights as written. A bad list keeps the last good Rights and says why.
  function sendRights() {
    for (const l of lists) draft[l.kind] = { all: !l.only.checked, text: l.text.value };
    draft.visibility = hidden.input.checked ? 'hidden' : 'readonly';
    const r = rightsOf(draft, ctx.offset(), ctx.levelCount() || 99);
    note.textContent = r.error || '';
    if (!r.error) ed.setFields({ clientRights: r.rights });
  }
  for (const l of lists) {
    l.only.addEventListener('change', sendRights);
    l.text.addEventListener('input', sendRights);
  }
  hidden.input.addEventListener('change', sendRights);
  limit.input.addEventListener('change', () => {
    if (limit.input.checked) {
      draft = draftOf(ed.panel()?.clientRights, ctx.offset());
      fill();
      sendRights();
    } else {
      ed.setFields({ clientRights: undefined });
    }
  });
  function fill() {
    for (const l of lists) {
      l.only.checked = !draft[l.kind].all;
      l.text.value = draft[l.kind].text;
    }
    hidden.input.checked = draft.visibility === 'hidden';
  }

  return {
    node,
    update() {
      const p = ed.panel();
      const ro = ed.readOnly();
      const f = ed.fields();
      if (asking && p?.id !== askedFor) {
        asking = null;
        guard.cancel();
      }
      acts.set(asking ?? f.acts);
      // Switching to Session mode is open to the owner; setting up or widening a client needs a Trusted user (server rule).
      acts.disable(ro || (!me.trusted && !hasClient(p)));
      actsHint.textContent = f.acts === 'session' && !me.trusted && !ro ? 'Only a Trusted user can make it act as an API client.' : '';
      clientBox.hidden = !hasClient(f);
      if (hasClient(f)) {
        const had = hasClient(p) && p.client;
        current.textContent = had
          ? `Client ${p.client}. ${f.clientRights ? 'Unsaved Rights.' : 'Rights.'}`
          : 'Saving the panel makes its client, with your own Rights unless you limit them below.';
        table.hidden = !had;
        if (had) {
          const rows = rightsRows(f.clientRights || p.clientRights);
          table.replaceChildren(el('tbody', null, ...rows.map(([k, v]) => el('tr', null, el('th', null, k), el('td', null, v)))));
        }
        limit.input.disabled = ro;
        limit.input.checked = !!f.clientRights;
        limits.hidden = !limit.input.checked;
        for (const l of lists) l.only.disabled = l.text.disabled = ro;
        hidden.input.disabled = ro;
      }
      const stored = !!p && !ro;
      shareLine.textContent = p ? 'Who may use it:' : 'Save the panel first, then choose who may use it.';
      const who = p?.share?.mode === 'named' && p.share.users?.length ? p.share.users : p ? [shareLabel(p.share)] : [];
      shareList.replaceChildren(...who.map((u) => el('li', null, u)));
      shareBtn.disabled = !stored;
    },
  };
}
