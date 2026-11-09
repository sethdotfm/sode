// Find in the panel editor's smart menu: one search over Sources, Destinations and Presets, with a preview of
// the fill in the grid before anything changes. Ranking, Rights and the fill itself are findrules.js; this file is
// the DOM. initFind(editor) takes what runEditor returns.
import { el, icon, onMsg, on, $ } from './core.js';
import { sessionFetch, jsonCall } from './sessionkey.js';
import { resultStep } from './keys.js';
import { selSquares, single } from './gridmodel.js';
import { createOrderPicker } from './editororder.js';
import { CATEGORIES, CATEGORY_LABEL, CATEGORY_ICON, findEntries, rankFind, fillRun, findPlan, applyFind, afterFill } from './findrules.js';

const SHOWN = 200; // rows drawn; typing narrows the rest
const STALE_MS = 5000;

// The user's Rights arrive as soon as they sign in, which can be before the editor has loaded, so they are
// caught at import time and handed to the editor when it starts.
let myRights = null;
let rightsChanged = () => {};
onMsg('myRights', (m) => {
  myRights = m.rights || null;
  rightsChanged();
});
on('wsclose', () => {
  myRights = null;
  rightsChanged();
});

export function initFind(editor) {
  let rights = null;
  let data = { names: null, presets: null, at: 0, loading: false };
  let entries = [];
  let results = [];
  let chip = ''; // '' is all categories
  let hi = 0; // index into results
  let anchor = -1; // where Shift started a picked set
  let picked = []; // result keys picked with Shift, in list order
  let order = 'across'; // one of gridmodel's FILL_ORDERS; the order picker sets it
  let hovered = false;
  let rebuilding = false;
  let preview = [];
  let quiet = false; // a group fill just landed, so no preview of filling it again until the user moves

  const input = el('input', 'pb-input');
  input.type = 'search';
  input.placeholder = 'Find a Source, Destination or Preset';
  input.setAttribute('aria-label', 'Find');
  input.autocomplete = 'off';
  const chips = el('div', 'sode-find__chips');
  chips.setAttribute('role', 'group');
  chips.setAttribute('aria-label', 'Narrow Find to');
  const list = el('ul', 'sode-find__list');
  list.setAttribute('role', 'listbox');
  list.setAttribute('aria-label', 'Find results');
  list.setAttribute('aria-multiselectable', 'true');
  const note = el('p', 'sode-find__note');
  note.setAttribute('role', 'status');
  const fill = el('button', 'pb-btn pb-btn--ok pb-btn--sm', 'Fill');
  fill.type = 'button';
  const foot = el('div', 'sode-find__foot', note, fill);
  const orders = createOrderPicker(order, (id) => api.setOrder(id));
  const panel = el('div', 'pb-menu pb-menu__sub sode-find', input, chips, list, orders, foot);

  rightsChanged = () => {
    rights = myRights;
    rebuild();
  };
  rights = myRights;

  function rebuild() {
    entries = findEntries(data.names, data.presets, rights);
    draw();
  }

  async function load(force = false) {
    if (data.loading || (!force && Date.now() - data.at < STALE_MS)) return;
    data.loading = true;
    const get = (path) => jsonCall(sessionFetch)(path).catch(() => null);
    const [names, presets] = await Promise.all([get('api/v1/names'), get('api/v1/presets')]);
    data = { names, presets, at: Date.now(), loading: false };
    rebuild();
  }

  const active = () => panel.dataset.open === '1' || hovered;
  const usable = (e) => e && !e.off;
  const picks = () => picked.map((k) => entries.find((e) => e.key === k)).filter(Boolean);

  // The entries a choice would place: the picked set, else the result at index i and every later one by number.
  function runFor(i) {
    const p = picks();
    if (p.length > 1) return fillRun(entries, p[0], p);
    return fillRun(entries, results[i], null);
  }
  function planFor(i) {
    const g = editor.grid();
    return g.sel ? findPlan(g, runFor(i), order) : null;
  }

  // ---- preview in the grid

  function clearPreview() {
    for (const p of preview) {
      p.node.className = p.cls;
      p.node.replaceChildren(...p.kids);
    }
    preview = [];
  }
  function showPreview() {
    clearPreview();
    const plan = active() && !quiet ? planFor(hi) : null;
    note.textContent = describe(plan);
    if (!plan) return;
    for (const s of plan.slots) {
      const node = $('ed-grid')?.querySelector(`.sode-ed__sq[data-r="${s.r}"][data-c="${s.c}"]`);
      if (!node) continue;
      preview.push({ node, cls: node.className, kids: [...node.childNodes] });
      node.classList.add('is-preview');
      if (s.replace) node.classList.add('is-replaced');
      node.replaceChildren(icon(CATEGORY_ICON[plan.kind]), el('span', 'sode-ed__text', s.entry.label));
    }
  }
  function describe(plan) {
    if (!plan) return picked.length > 1 ? '' : results[hi]?.off ? 'Outside your rights, so it cannot be placed.' : '';
    const n = plan.slots.length;
    const first = plan.slots[0].entry.label;
    const last = plan.slots[n - 1].entry.label;
    const span = n > 1 ? `${first} to ${last}` : first;
    return `Fills ${n} square${n > 1 ? 's' : ''} with ${span}.${plan.dropped && picked.length > 1 ? ` ${plan.dropped} picked do not fit.` : ''}`;
  }

  // ---- list

  function draw() {
    results = rankFind(entries, input.value, chip ? [chip] : []);
    hi = Math.max(0, Math.min(hi, results.length - 1));
    picked = picked.filter((k) => results.some((e) => e.key === k));
    chips.replaceChildren(
      ...['', ...CATEGORIES].map((c) => {
        const b = el('button', 'pb-chip', c ? icon(CATEGORY_ICON[c]) : null, c ? CATEGORY_LABEL[c] : 'All');
        b.type = 'button';
        b.setAttribute('aria-pressed', String(chip === c));
        b.addEventListener('click', () => {
          chip = c;
          hi = 0;
          clearPicks();
          draw();
          input.focus();
        });
        return b;
      }),
    );
    list.replaceChildren(
      ...results.slice(0, SHOWN).map((e, i) => {
        const li = el(
          'li',
          `sode-find__row${e.off ? ' sode__menu-off' : ''}`,
          icon(CATEGORY_ICON[e.category]),
          el('span', 'sode__num', e.num === undefined ? '' : String(e.num)),
          el('span', 'sode-find__name', e.label),
        );
        if (e.off) li.append(el('span', 'sode__menu-tag', 'Outside rights'));
        li.dataset.i = String(i);
        li.setAttribute('role', 'option');
        li.setAttribute('aria-selected', String(i === hi || picked.includes(e.key)));
        li.setAttribute('aria-disabled', String(!!e.off));
        if (i === hi) li.classList.add('is-hi');
        if (picked.includes(e.key)) li.classList.add('is-pick');
        return li;
      }),
    );
    if (!results.length) list.append(el('li', 'sode-find__none', data.names ? 'Nothing matches.' : 'Find needs a signed-in user.'));
    else if (results.length > SHOWN) list.append(el('li', 'sode-find__none', `${results.length - SHOWN} more. Keep typing to narrow.`));
    fill.hidden = picked.length < 2;
    list.querySelector('.is-hi')?.scrollIntoView({ block: 'nearest' });
    showPreview();
  }

  const clearPicks = () => {
    quiet = false; // every change of choice (typing, a chip, a step, a new pick) is a move
    picked = [];
    anchor = -1;
  };
  function move(step, extend) {
    if (!results.length) return;
    quiet = false;
    if (extend && anchor < 0) anchor = hi;
    hi = (hi + step + results.length) % results.length;
    if (extend)
      picked = results
        .slice(Math.min(anchor, hi), Math.max(anchor, hi) + 1)
        .filter(usable)
        .map((e) => e.key);
    else clearPicks();
    draw();
  }

  // ---- choosing

  function choose(i) {
    const e = results[i];
    if (!usable(e)) return;
    hi = i;
    clearPicks();
    place(i);
  }
  function place(i) {
    const g = editor.grid();
    const plan = planFor(i);
    if (!plan || !plan.slots.length) return;
    const one = selSquares(g) === 1 || !!single(g);
    clearPreview();
    let next = applyFind(g, plan);
    if (one && picked.length < 2) {
      next = afterFill(next);
      // The next result is the next one that can be placed, so a row of clicks walks the list.
      const j = results.findIndex((e, k) => k > i && usable(e));
      if (j >= 0) hi = j;
    }
    clearPicks();
    quiet = !one;
    editor.setGrid(next);
  }

  list.addEventListener('click', (e) => {
    const li = e.target.closest('.sode-find__row');
    if (!li) return;
    const i = Number(li.dataset.i);
    if (e.shiftKey) {
      quiet = false;
      if (anchor < 0) anchor = hi;
      hi = i;
      picked = results
        .slice(Math.min(anchor, i), Math.max(anchor, i) + 1)
        .filter(usable)
        .map((x) => x.key);
      draw();
    } else choose(i);
  });
  list.addEventListener('pointerdown', (e) => {
    if (e.target.closest('.sode-find__row')) e.preventDefault(); // keeps focus in the box
  });
  list.addEventListener('pointerover', (e) => {
    const li = e.target.closest('.sode-find__row');
    if (!li || picked.length > 1) return;
    const i = Number(li.dataset.i);
    if (i === hi) return;
    quiet = false;
    hi = i;
    list.querySelector('.is-hi')?.classList.remove('is-hi');
    li.classList.add('is-hi');
    showPreview();
  });
  fill.addEventListener('click', () => place(hi));

  input.addEventListener('input', () => {
    hi = 0;
    clearPicks();
    draw();
  });
  input.addEventListener('keydown', (e) => {
    const step = resultStep(e, results.length);
    if (step) {
      e.preventDefault();
      e.stopPropagation();
      move(step, e.shiftKey);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      e.stopPropagation();
      place(hi);
    }
  });

  // ---- open and close

  const close = () => {
    delete panel.dataset.open;
    hovered = false;
    clearPreview();
    note.textContent = '';
  };
  panel.addEventListener('pointerenter', () => {
    hovered = true;
    load();
    showPreview();
  });
  panel.addEventListener('pointerleave', () => {
    hovered = false;
    if (!panel.contains(document.activeElement)) close();
  });
  panel.addEventListener('focusin', () => {
    panel.dataset.open = '1';
    load();
    showPreview();
  });
  panel.addEventListener('focusout', (e) => {
    if (rebuilding || (e.relatedTarget && panel.contains(e.relatedTarget))) return;
    if (!hovered) close();
  });

  // text null keeps the box as it is.
  function open(text) {
    panel.dataset.open = '1';
    if (text !== null) {
      input.value = text;
      hi = 0;
      clearPicks();
    }
    load();
    draw();
    input.focus();
  }

  editor.addKeyHook((e) => {
    if (!editor.grid().sel) return false;
    const typing = e.key.length === 1 && e.key !== ' ';
    if (e.key !== 'Enter' && !typing) return false;
    e.preventDefault();
    open(typing ? e.key : null);
    return true;
  });

  // The menu rebuilds itself after every grid change and detaches this panel, which drops focus: put it back.
  editor.addMenuSection(() => {
    const had = panel.contains(document.activeElement);
    rebuilding = true;
    preview = []; // the grid has repainted, so the squares the preview changed are gone
    queueMicrotask(() => {
      rebuilding = false;
      if (had && !panel.contains(document.activeElement)) input.focus({ preventScroll: true });
      draw();
    });
    return [{ label: 'Find', icon: 'search', name: 'Find', panel, run: () => open(null) }];
  });
  editor.refresh();

  const api = {
    setOrder(id) {
      order = id;
      orders.set(id);
      draw();
      input.focus({ preventScroll: true });
    },
    order: () => order,
  };
  return api;
}
