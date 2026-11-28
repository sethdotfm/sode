// The Share modal on the panels page, with People first and Link with key (panelkeytab.js). Sharing is only the share
// field of PUT /api/v1/panels/{id}.
import { el } from './core.js';
import { btn, check } from './formbits.js';
import { linkRow } from './panellinkrow.js';
import { question } from './modal.js';
import { makeTabs } from './tabs.js';
import { firstShareTab } from './panelkeyrules.js';
import { keyTab } from './panelkeytab.js';
import { shareDraft, pickList, toggleUser, shareBody, shareChanged } from './panelsrules.js';

const MODES = [
  ['me', 'Only me'],
  ['named', 'Named users'],
  ['everyone', 'Everyone'],
];

// ctx.onSaved gets the panel the server answered with.
export function openShare(panel, ctx) {
  const opener = document.activeElement;
  let draft = shareDraft(panel.share);
  let people = null; // null while loading, [] when the list failed
  let busy = false;

  const note = el('span', { class: 'sode__hint', role: 'status' });
  const saveBtn = btn('sode-btn--primary', 'Save');
  const modes = el('div', { class: 'pb-btn-group', role: 'radiogroup', 'aria-label': 'Who can use this panel' });
  const picker = el('div', 'sode-ov__people');
  const peopleTab = el('div', 'sode-ov__sharebody', modes, picker, linkRow(panel));

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
        if (mode !== 'me') l.dataset.pbTip = 'Use, not change';
        return [r, l];
      }),
    );
    picker.hidden = draft.mode !== 'named';
    picker.replaceChildren();
    if (draft.mode === 'named') {
      if (people === null) picker.append(el('p', 'sode__hint', 'Loading…'));
      else {
        const list = pickList(people, panel.owner, draft.users);
        if (!list.length) picker.append(el('p', 'sode__hint', 'No other users'));
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

  const items = [{ id: 'people', text: 'People', panel: peopleTab }, keyTab({ panel, api: ctx.api })];
  const tabs = makeTabs(items);
  if (items.some((i) => i.id === firstShareTab(panel))) tabs.select(firstShareTab(panel));
  const q = question({
    back: opener,
    titleId: 'share-title',
    title: `Share ${panel.name}`,
    cls: 'sode-ov__share',
    body: [tabs.node, ...items.map((i) => i.panel)],
    foot: [btn('pb-btn--outline', 'Cancel', () => close()), note, saveBtn],
  });
  const { root } = q;
  const close = () => q.close(null);
  saveBtn.addEventListener('click', async () => {
    busy = true;
    sync();
    note.textContent = '';
    try {
      // PUT replaces the whole panel, so it starts from the stored copy. It is possible that the one this modal opened
      // with is older than an edit or an Authentication mode switch saved since.
      const fresh = await ctx.api(`api/v1/panels/${panel.id}`);
      const saved = await ctx.api(`api/v1/panels/${panel.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(shareBody(fresh, draft)),
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
  q.dialog.focus();
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
