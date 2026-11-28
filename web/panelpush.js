// The Push modal on the panels page. The server decides who can push to what and keeps a checkpoint of every target first.
import { el, icon } from './core.js';
import { btn, check, opt } from './formbits.js';
import { question } from './modal.js';
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
  ownerSections,
} from './panelpushrules.js';

const JSON_HEAD = { 'Content-Type': 'application/json' };
const post = (api, path, body) => api(path, { method: 'POST', headers: JSON_HEAD, body: JSON.stringify(body ?? {}) });

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

  const note = el('span', { class: 'sode__hint', role: 'status' });
  const say = (t) => (note.textContent = t);

  const targets = el('div', 'sode-ov__people');
  const pushBtn = btn('sode-btn--primary', [icon('send'), ' Push']);
  const pushTab = el(
    'div',
    'sode-ov__tabbody',
    el('p', 'sode__hint', "Push copies this panel's layout to the panels you tick. Each panel keeps a checkpoint of what it had."),
    targets,
    el('div', 'sode-ov__acts', pushBtn),
  );

  function paintTargets() {
    targets.replaceChildren();
    if (!cand.length) return targets.append(el('p', 'sode__hint', 'No other panels'));
    const row = (p) => {
      const c = check(p.name, {
        checked: selected.has(p.id),
        onChange: (v) => (v ? selected.add(p.id) : selected.delete(p.id), sync()),
      });
      c.input.dataset.panel = p.id;
      return c.label;
    };
    for (const sec of ownerSections(cand, ctx.me)) {
      if (sec.label) targets.append(el('h3', 'sode-ov__group', sec.label));
      paintSection(sec.panels, row);
    }
  }
  function paintSection(panels, row) {
    const { groups, single } = pushGroups(panels);
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

  const setSel = el('select', 'pb-select');
  setSel.setAttribute('aria-label', 'Checkpoint set');
  const setPush = btn('sode-btn--primary', [icon('send'), ' Push set']);
  const setDel = btn('pb-btn--ghost', [icon('trash-2'), ' Delete set']);
  const setHint = el('p', 'sode__hint');
  const nameIn = el('input', 'pb-input');
  nameIn.setAttribute('aria-label', 'Set name');
  nameIn.placeholder = 'Set name';
  nameIn.autocomplete = 'off';
  const scopeSel = el('select', 'pb-select');
  scopeSel.setAttribute('aria-label', 'Who can use the set');
  scopeSel.append(opt('me', 'Me'), ...(ctx.trusted ? [opt('everyone', 'Everyone')] : []));
  const incl = check(`Include ${panel.name}`, { checked: true });
  const saveSet = btn('pb-btn--outline', [icon('copy-plus'), ' Save as a set']);
  const saveHint = el('p', 'sode__hint');
  const setsTab = el(
    'div',
    'sode-ov__tabbody',
    el(
      'p',
      'sode__hint',
      'A checkpoint set keeps one saved layout per panel, so one Push gives each panel its own. Tick panels on the Push tab to pick some, or leave them unticked to push all of a set.',
    ),
    el('div', 'sode-ov__acts', el('span', 'pb-select-wrap', setSel), setPush, setDel),
    setHint,
    el('h2', 'sode-ov__sub', 'New set'),
    el('div', 'sode-ov__acts', nameIn, el('span', 'pb-select-wrap', scopeSel), incl.label, saveSet),
    saveHint,
  );

  function paintSets() {
    setSel.replaceChildren(...(sets === null ? [opt('', 'Loading…')] : sets.length ? sets.map((s) => opt(s.id, setLabel(s))) : [opt('', 'No sets')]));
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

  const restoreSel = el('select', 'pb-select');
  restoreSel.setAttribute('aria-label', 'Panel to restore');
  restoreSel.append(...everyone.map((p) => opt(p.id, p.name + ownerNote(p, ctx.me))));
  const cpList = el('div', 'sode-ov__people');
  const restoreTab = el(
    'div',
    'sode-ov__tabbody',
    el('p', 'sode__hint', 'Each Push and restore keeps what it replaced, up to 10 per panel. A restore puts a layout back and keeps the one it replaces.'),
    el('div', 'sode-ov__acts', el('span', 'pb-select-wrap', restoreSel)),
    cpList,
  );

  function paintCps() {
    cpList.replaceChildren();
    if (cps === null) return cpList.append(el('p', 'sode__hint', 'Loading…'));
    if (!cps.length) return cpList.append(el('p', 'sode__hint', 'No checkpoints'));
    for (const c of cps) {
      const b = btn('pb-btn--outline pb-btn--sm', [icon('rotate-ccw'), ' Restore']);
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
    pushBtn.lastChild.textContent = selected.size ? ` Push to ${selected.size} panel${selected.size === 1 ? '' : 's'}` : ' Push';
    const s = sets?.find((x) => x.id === setId);
    setPush.disabled = busy || !s;
    setDel.disabled = busy || !s || !s.canEdit;
    setDel.lastChild.textContent = armed ? ' Press again to delete' : ' Delete set';
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
  const q = question({
    back: opener,
    titleId: 'push-title',
    title: `Push ${panel.name}`,
    cls: 'sode-ov__share',
    body: [tabs.node, ...items.map((i) => i.panel)],
    foot: [note, btn('pb-btn--outline', 'Close', () => q.close(null))],
  });
  const { root } = q;
  // A push reloads the list behind the modal, which replaces the opener.
  q.answer.then(() => opener?.isConnected || document.querySelector('#panels-detail [data-act="push"]')?.focus());

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
      say('Set deleted.');
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
      say(`Saved set ${made.name}.`);
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
      const text = `Restored ${name}. Checkpoint kept.`;
      say(text);
      ctx.onDone(text);
      await loadCps();
    });
  });

  paintTargets();
  paintSets();
  paintCps();
  sync();
  q.dialog.focus();
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
