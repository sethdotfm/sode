// The Share modal on the panels page: tabs, with People first. Sharing is only the share field of
// PUT /api/v1/panels/{id}, which the server enforces for open lists and launches.
// Another tab joins by pushing a function onto shareTabs that returns {id, text, panel}, or
// null when it does not apply to that panel (ctx: {panel, api, close}).
import { el, on } from './core.js';
import { check } from './formbits.js';
import { trapDialog } from './dialogkeys.js';
import { makeTabs } from './tabs.js';
import { shareDraft, pickList, toggleUser, shareBody, shareChanged } from './panelsrules.js';

export const shareTabs = [];

// The Lock covers the page, so an open modal must not stay in front of it.
let closeOpen = null;
on('locking', () => closeOpen?.());

const MODES = [
  ['me', 'Only me'],
  ['named', 'Named users'],
  ['everyone', 'Everyone'],
];
const HINT = {
  me: 'Only you and Trusted users can see this panel.',
  named: 'The users you tick can launch this panel. They can use it, not change it.',
  everyone: 'Every user can launch this panel. They can use it, not change it.',
};

// ctx.onSaved gets the panel the server answered with.
export function openShare(panel, ctx) {
  const opener = document.activeElement;
  let draft = shareDraft(panel.share);
  let people = null; // null while loading, [] when the list failed
  let busy = false;

  const note = el('span', 'sode__hint');
  note.setAttribute('role', 'status');
  const saveBtn = el('button', 'pb-btn pb-btn--ok', 'Save');
  saveBtn.type = 'button';
  const cancelBtn = el('button', 'pb-btn pb-btn--outline', 'Cancel');
  cancelBtn.type = 'button';
  const modes = el('div', 'pb-btn-group');
  modes.setAttribute('role', 'radiogroup');
  modes.setAttribute('aria-label', 'Who may use this panel');
  const hint = el('p', 'sode__hint');
  const picker = el('div', 'sode-ov__people');
  const peopleTab = el('div', null, modes, hint, picker);

  function paint() {
    modes.replaceChildren(
      ...MODES.flatMap(([mode, text]) => {
        const r = el('input', 'pb-btn-check');
        r.type = 'radio';
        r.name = 'share-mode';
        r.id = `share-mode-${mode}`;
        r.dataset.mode = mode;
        r.checked = draft.mode === mode;
        const l = el('label', 'pb-btn pb-btn--outline', text);
        l.htmlFor = r.id;
        return [r, l];
      }),
    );
    hint.textContent = HINT[draft.mode];
    picker.hidden = draft.mode !== 'named';
    picker.replaceChildren();
    if (draft.mode === 'named') {
      if (people === null) picker.append(el('p', 'sode__hint', 'Loading users.'));
      else {
        const list = pickList(people, panel.owner, draft.users);
        if (!list.length) picker.append(el('p', 'sode__hint', 'There are no other users.'));
        for (const u of list) {
          const c = check(u.name, {
            checked: u.on,
            onChange: (on) => {
              draft = { ...draft, users: toggleUser(draft.users, u.name, on) };
              sync();
            },
          });
          picker.append(c.label);
        }
      }
    }
    sync();
  }
  function sync() {
    saveBtn.disabled = busy || !shareChanged(panel.share, draft);
  }

  modes.addEventListener('change', (e) => {
    draft = { ...draft, mode: e.target.dataset.mode };
    note.textContent = '';
    paint();
    modes.querySelector(`[data-mode="${draft.mode}"]`).focus();
  });

  const root = el('div', 'pb-modal-backdrop sode__scrim');
  const dialog = el('div', 'pb-modal sode-ov__share');
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', 'share-title');
  dialog.tabIndex = -1;
  const items = [{ id: 'people', text: 'People', panel: peopleTab }];
  const extra = { panel, api: ctx.api, close };
  for (const make of shareTabs) {
    const t = make(extra);
    if (t) items.push(t);
  }
  const tabs = makeTabs(items);
  dialog.append(
    el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', `Share ${panel.name}`), { id: 'share-title' })),
    el('div', 'pb-modal__body', tabs.node, ...items.map((i) => i.panel)),
    el('div', 'pb-modal__footer', note, cancelBtn, saveBtn),
  );
  root.append(dialog);
  document.body.append(root);
  const dlg = trapDialog(root, dialog, close);

  function close() {
    closeOpen = null;
    dlg.hide();
    root.remove();
    if (opener?.isConnected) opener.focus();
  }
  closeOpen = close;
  cancelBtn.addEventListener('click', close);
  saveBtn.addEventListener('click', async () => {
    busy = true;
    sync();
    note.textContent = '';
    try {
      const saved = await ctx.api(`api/v1/panels/${panel.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(shareBody(panel, draft)),
      });
      close();
      ctx.onSaved(saved);
    } catch (e) {
      busy = false;
      note.textContent = e.message;
      sync();
    }
  });

  paint();
  dlg.show();
  dialog.focus();
  ctx
    .api('api/v1/panels/people')
    .then((r) => (people = r.users))
    .catch((e) => {
      people = [];
      note.textContent = `Users could not be loaded. ${e.message}`;
    })
    .finally(() => {
      if (root.isConnected) paint();
    });
}
