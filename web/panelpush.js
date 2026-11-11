// The Push modal on the panels page: push a panel's saved layout to other panels, keep and push
// checkpoint sets, and restore a checkpoint. The server decides who may push to what and keeps a
// checkpoint of every target first. Calls: POST /panels/{id}/push, /panel-sets, /panels/{id}/checkpoints.
import { el, icon, on } from './core.js';
import { check, opt } from './formbits.js';
import { trapDialog } from './dialogkeys.js';
import { makeTabs } from './tabs.js';
import {
  pushCandidates,
  pushGroups,
  groupState,
  setGroup,
  pushBody,
  setBody,
  setTargets,
  pushSummary,
  checkpointLabel,
  setLabel,
  canSaveSet,
  ownerNote,
} from './panelpushrules.js';

// The Lock covers the page, so an open modal must not stay in front of it.
let closeOpen = null;
on('locking', () => closeOpen?.());

const JSON_HEAD = { 'Content-Type': 'application/json' };
const post = (api, path, body) => api(path, { method: 'POST', headers: JSON_HEAD, body: JSON.stringify(body ?? {}) });

function button(text, ico, cls) {
  const b = el('button', `pb-btn ${cls}`, ...(ico ? [icon(ico), ` ${text}`] : [text]));
  b.type = 'button';
  return b;
}

// ctx: {api, panels (every panel the page has), me, trusted, onDone(text)}.
export function openPush(panel, ctx) {
  const opener = document.activeElement;
  const cand = pushCandidates(ctx.panels, panel.id);
  const everyone = [panel, ...cand];
  const selected = new Set();
  let sets = null; // null while loading
  let setId = '';
  let armed = false;
  let restoreId = panel.id;
  let cps = null;
  let busy = false;

  const note = el('span', 'sode__hint');
  note.setAttribute('role', 'status');
  const say = (t) => (note.textContent = t);
  const closeBtn = button('Close', '', 'pb-btn--outline');

  // Push tab
  const targets = el('div', 'sode-ov__people');
  const pushBtn = button('Push', 'send', 'pb-btn--ok');
  const pushTab = el(
    'div',
    null,
    el(
      'p',
      'sode__hint',
      `Copies the saved layout of ${panel.name} to the panels you tick. Each keeps its own name, id and sharing, and a checkpoint of it is kept first. Open panels update like a saved edit.`,
    ),
    targets,
    el('div', 'sode-ov__acts', pushBtn),
  );

  function paintTargets() {
    targets.replaceChildren();
    if (!cand.length) return targets.append(el('p', 'sode__hint', 'You have no other panels to push to.'));
    const { groups, single } = pushGroups(cand);
    const row = (p) => {
      const c = check(p.name + ownerNote(p, ctx.me), {
        checked: selected.has(p.id),
        onChange: (v) => (v ? selected.add(p.id) : selected.delete(p.id), sync()),
      });
      c.input.dataset.panel = p.id;
      return c.label;
    };
    for (const g of groups) {
      const state = groupState(selected, g);
      const head = check(`All ${g.key} panels (${g.ids.length})`, {
        checked: state === 'all',
        onChange: (v) => {
          const next = setGroup(selected, g, v);
          selected.clear();
          for (const id of next) selected.add(id);
          paintTargets();
          sync();
        },
      });
      head.input.indeterminate = state === 'some';
      head.input.dataset.group = g.key;
      targets.append(head.label, el('div', 'sode-ov__indent', ...g.panels.map(row)));
    }
    for (const p of single) targets.append(row(p));
  }

  // Sets tab
  const setSel = el('select', 'pb-select');
  setSel.setAttribute('aria-label', 'Checkpoint set');
  const setPush = button('Push set', 'send', 'pb-btn--ok');
  const setDel = button('Delete set', 'trash-2', 'pb-btn--ghost');
  const setHint = el('p', 'sode__hint');
  const nameIn = el('input', 'pb-input');
  nameIn.setAttribute('aria-label', 'Name for a new set');
  nameIn.placeholder = 'Name of the set';
  nameIn.autocomplete = 'off';
  const scopeSel = el('select', 'pb-select');
  scopeSel.setAttribute('aria-label', 'Who may use the set');
  scopeSel.append(opt('me', 'Me'), ...(ctx.trusted ? [opt('everyone', 'Everyone')] : []));
  const incl = check(`Include ${panel.name}`, { checked: true });
  const saveSet = button('Save as a set', 'copy-plus', 'pb-btn--outline');
  const saveHint = el('p', 'sode__hint');
  const setsTab = el(
    'div',
    null,
    el(
      'p',
      'sode__hint',
      'A checkpoint set keeps one saved layout per panel, so one Push gives each panel its own. Tick panels on the Push tab to pick some, or leave them unticked to push all of a set.',
    ),
    el('div', 'sode-ov__acts', el('span', 'pb-select-wrap', setSel), setPush, setDel),
    setHint,
    el('h2', 'sode-ov__sub', 'Save the saved layouts of the ticked panels'),
    el('div', 'sode-ov__acts', nameIn, el('span', 'pb-select-wrap', scopeSel), incl.label, saveSet),
    saveHint,
  );

  function paintSets() {
    setSel.replaceChildren(...(sets === null ? [opt('', 'Loading sets')] : sets.length ? sets.map((s) => opt(s.id, setLabel(s))) : [opt('', 'No sets yet')]));
    if (sets?.some((s) => s.id === setId)) setSel.value = setId;
    else setId = setSel.value;
    const s = sets?.find((x) => x.id === setId);
    setHint.textContent = s
      ? `Pushes to ${setTargets(s, selected)
          .map((id) => everyone.find((p) => p.id === id)?.name || s.entries.find((e) => e.panel === id)?.name || id)
          .join(', ')}.`
      : '';
    sync();
  }

  // Restore tab
  const restoreSel = el('select', 'pb-select');
  restoreSel.setAttribute('aria-label', 'Panel to restore');
  restoreSel.append(...everyone.map((p) => opt(p.id, p.name + ownerNote(p, ctx.me))));
  const cpList = el('div', 'sode-ov__people');
  const restoreTab = el(
    'div',
    null,
    el('p', 'sode__hint', 'Each Push and restore keeps what it replaced, up to 10 per panel. Restoring puts a layout back and keeps the one it replaces.'),
    el('div', 'sode-ov__acts', el('span', 'pb-select-wrap', restoreSel)),
    cpList,
  );

  function paintCps() {
    cpList.replaceChildren();
    if (cps === null) return cpList.append(el('p', 'sode__hint', 'Loading checkpoints.'));
    if (!cps.length) return cpList.append(el('p', 'sode__hint', 'This panel has no checkpoints yet.'));
    for (const c of cps) {
      const b = button('Restore', 'rotate-ccw', 'pb-btn--outline pb-btn--sm');
      b.dataset.cp = c.id;
      b.disabled = busy;
      cpList.append(el('div', 'sode-ov__cp', el('span', null, checkpointLabel(c), c.why ? el('span', 'sode__hint', ` ${c.why}`) : ''), b));
    }
  }
  async function loadCps() {
    const id = restoreId;
    cps = null;
    paintCps();
    try {
      const r = await ctx.api(`api/v1/panels/${id}/checkpoints`);
      if (id === restoreId) cps = r.checkpoints;
    } catch (e) {
      if (id === restoreId) cps = [];
      say(`Checkpoints could not be loaded. ${e.message}`);
    }
    if (root.isConnected) paintCps();
  }

  function sync() {
    pushBtn.disabled = busy || !selected.size;
    const s = sets?.find((x) => x.id === setId);
    setPush.disabled = busy || !s;
    setDel.disabled = busy || !s || !s.canEdit;
    setDel.lastChild.textContent = armed ? ' Delete? Press again' : ' Delete set';
    saveSet.disabled = busy || !canSaveSet(nameIn.value, new Set([...selected, ...(incl.input.checked ? [panel.id] : [])]));
    saveHint.textContent = selected.size ? '' : 'Tick panels on the Push tab first.';
  }

  async function run(work) {
    busy = true;
    sync();
    say('');
    try {
      await work();
    } catch (e) {
      say(e.message);
    }
    busy = false;
    if (root.isConnected) {
      sync();
      paintCps();
    }
  }

  const root = el('div', 'pb-modal-backdrop sode__scrim');
  const dialog = el('div', 'pb-modal sode-ov__share');
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', 'push-title');
  dialog.tabIndex = -1;
  const items = [
    { id: 'push', text: 'Push', panel: pushTab },
    { id: 'sets', text: 'Checkpoint sets', panel: setsTab },
    { id: 'restore', text: 'Restore', panel: restoreTab },
  ];
  const tabs = makeTabs(items, (id) => {
    say('');
    if (id === 'sets') paintSets();
    if (id === 'restore' && cps === null) loadCps();
  });
  dialog.append(
    el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', `Push ${panel.name}`), { id: 'push-title' })),
    el('div', 'pb-modal__body', tabs.node, ...items.map((i) => i.panel)),
    el('div', 'pb-modal__footer', note, closeBtn),
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
  closeBtn.addEventListener('click', close);

  pushBtn.addEventListener('click', () =>
    run(async () => {
      const r = await post(ctx.api, `api/v1/panels/${panel.id}/push`, pushBody(selected));
      const text = pushSummary(r, everyone);
      say(text);
      cps = null;
      ctx.onDone(text);
    }),
  );
  setSel.addEventListener('change', () => {
    setId = setSel.value;
    armed = false;
    paintSets();
  });
  setPush.addEventListener('click', () =>
    run(async () => {
      const s = sets.find((x) => x.id === setId);
      const r = await post(ctx.api, `api/v1/panel-sets/${setId}/push`, setBody(s, selected));
      const text = pushSummary(r, [...everyone, ...s.entries.map((e) => ({ id: e.panel, name: e.name }))]);
      say(text);
      cps = null;
      ctx.onDone(text);
    }),
  );
  setDel.addEventListener('click', () => {
    if (!armed) {
      armed = true;
      return sync();
    }
    armed = false;
    run(async () => {
      await ctx.api(`api/v1/panel-sets/${setId}`, { method: 'DELETE' });
      sets = sets.filter((s) => s.id !== setId);
      setId = '';
      paintSets();
      say('The set was deleted.');
    });
  });
  nameIn.addEventListener('input', sync);
  incl.input.addEventListener('change', sync);
  saveSet.addEventListener('click', () =>
    run(async () => {
      const ids = [...selected, ...(incl.input.checked ? [panel.id] : [])];
      const made = await post(ctx.api, 'api/v1/panel-sets', { name: nameIn.value, scope: scopeSel.value, panels: ids });
      sets = [...(sets || []), made];
      setId = made.id;
      nameIn.value = '';
      paintSets();
      say(`Saved the set ${made.name}.`);
    }),
  );
  restoreSel.addEventListener('change', () => {
    restoreId = restoreSel.value;
    loadCps();
  });
  cpList.addEventListener('click', (e) => {
    const b = e.target.closest('[data-cp]');
    if (!b) return;
    run(async () => {
      const name = everyone.find((p) => p.id === restoreId)?.name || restoreId;
      await post(ctx.api, `api/v1/panels/${restoreId}/checkpoints/${b.dataset.cp}/restore`);
      cps = null;
      const text = `Restored ${name} from a checkpoint. What it replaced was kept as a checkpoint.`;
      say(text);
      ctx.onDone(text);
      await loadCps();
    });
  });

  paintTargets();
  paintSets();
  paintCps();
  sync();
  dlg.show();
  dialog.focus();
  ctx
    .api('api/v1/panel-sets')
    .then((r) => (sets = r.sets))
    .catch((e) => {
      sets = [];
      say(`Checkpoint sets could not be loaded. ${e.message}`);
    })
    .finally(() => {
      if (root.isConnected) paintSets();
    });
}
