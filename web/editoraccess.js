// The sidebar's Access tab: who the panel acts as, the Rights limits of its own API client, and sharing.
// The server decides who may set up a client (ADR 0006): a Trusted user, within their own Rights.
// Sharing is the Share modal's job (panelshare.js).
import { el, on } from './core.js';
import { sessionFetch, jsonCall } from './sessionkey.js';
import { btn, check, opt } from './formbits.js';
import { openShare } from './panelshare.js';
import { shareLabel } from './panelsrules.js';
import { draftOf, rightsOf } from './rightsrules.js';
import { rightsLine, watchView } from './sidebarrules.js';
import { watchBox } from './panelsmon.js';

const LISTS = [
  ['sources', 'Sources', 'Router numbers, such as 1-8, 12'],
  ['dsts', 'Destinations', 'Router numbers, such as 1-8, 12'],
  ['levels', 'Levels', 'Level numbers, such as 1, 2'],
  ['presets', 'Presets', 'One name per line'],
];

const api = jsonCall(sessionFetch, { failed: (s) => `Sode answered ${s}` });

let me = { user: '', trusted: false };
on('access', (a) => (me = { user: a.user || '', trusted: !!a.trusted && !!a.user }));

// offset is the router's numberOffset and levelCount how many levels exist, for typing Rights lists.
export function createAccessTab(ed, ctx) {
  const node = el('div', 'sode-side__tab');
  const acts = el('select', 'pb-select', opt('session', 'Whoever opens it (Session mode)'), opt('api', 'Its own API client'));
  acts.id = 'ed-acts';
  const actsHint = el('p', 'sode__hint');
  const actsField = el('div', 'sode__field', el('label', null, 'Acts as'), el('span', 'pb-select-wrap', acts), actsHint);

  const clientBox = el('div', 'sode-side__sec');
  const current = el('p', 'sode-side__rights');
  const limit = check('Limit its Rights below mine', { id: 'ed-limit' });
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
  clientBox.append(current, limit.label, limits, note);

  const shareLine = el('p', 'sode__hint');
  const shareBtn = btn('pb-btn--outline pb-btn--sm', 'Share', () => {
    openShare(ed.panel(), { api, onSaved: (p) => ed.replaceLoaded(p) });
  });
  const share = el('section', 'sode-side__sec', el('h3', 'sode-side__h', 'Sharing'), shareLine, shareBtn);
  const watch = el('section', 'sode-side__sec');
  watch.hidden = true;
  const watchNote = el('p', 'sode__hint');
  watchNote.setAttribute('role', 'status');
  const known = {};
  let watchKey = '';
  const setWatch = async (id, on) => {
    try {
      await api(`api/v1/panels/${id}/watch`, { method: on ? 'POST' : 'DELETE' });
      known[id] = on;
      watchKey = `${id}|${on}`;
      watchNote.textContent = '';
    } catch (e) {
      watchNote.textContent = e.message;
      watchKey = '';
      paintWatch();
    }
  };
  // Rebuilt only when what it shows changes, so a refresh does not take the checkbox from under a hand.
  function paintWatch() {
    const v = watchView(ed.panel(), known);
    watch.hidden = !v;
    const key = v ? `${v.id}|${v.on}` : '';
    if (!v || key === watchKey) return;
    watchKey = key;
    watch.replaceChildren(el('h3', 'sode-side__h', 'Monitoring'), watchBox({ id: v.id, canEdit: true, watching: v.on }, setWatch), watchNote);
  }
  node.append(el('section', 'sode-side__sec', el('h3', 'sode-side__h', 'Identity of the panel'), actsField), clientBox, share, watch);

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
  acts.addEventListener('change', () => ed.setFields({ acts: acts.value }));

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
      acts.value = f.acts;
      // Switching to Session mode is open to the owner; setting up or widening a client needs a Trusted user (server rule).
      acts.disabled = ro || (!me.trusted && (p?.acts ?? 'session') === 'session');
      actsHint.textContent =
        f.acts === 'api'
          ? 'The panel has its own API client and keeps working after anyone signs out or their PIN changes.'
          : me.trusted || ro
            ? 'The panel uses the Rights of the user who opens it.'
            : 'The panel uses the Rights of the user who opens it. Only a Trusted user can make it act as an API client.';
      clientBox.hidden = f.acts !== 'api';
      if (f.acts === 'api') {
        const had = p?.acts === 'api' && p.client;
        current.textContent = had
          ? `Client ${p.client}. ${f.clientRights ? 'Unsaved Rights: ' : 'Rights: '}${rightsLine(f.clientRights || p.clientRights)}`
          : 'Saving the panel makes its client, with your own Rights unless you limit them below.';
        limit.input.disabled = ro;
        limit.input.checked = !!f.clientRights;
        limits.hidden = !limit.input.checked;
        for (const l of lists) l.only.disabled = l.text.disabled = ro;
        hidden.input.disabled = ro;
      }
      const stored = !!p && !ro;
      shareLine.textContent = p ? `Who may use it: ${shareLabel(p.share)}.` : 'Save the panel first, then choose who may use it.';
      shareBtn.disabled = !stored;
      paintWatch();
    },
  };
}
