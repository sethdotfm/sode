// The tie line editor of Settings > Routers > Tie lines: Add tie line at the top of the list, and Edit in the row. Each
// end has a find box that accepts a name or a number, as the patch boxes do (core.js boxSearch, routerprefix.js), with
// a match list. The name fills from the name of the downstream source until a person types another (fillName). One
// editor is open at a time, so the ids of its boxes are unique.

import { el, S, boxSearch, numText, routerData } from './core.js';
import { btn } from './formbits.js';
import { combobox } from './widgets.js';
import { fillName, otherRouter, endOfItem } from './tielinerules.js';

const MATCHES = 8;

const firstUid = () => S.routers[0]?.uid || '';
const uids = () => S.routers.map((r) => r.uid);
const itemOf = (end, kind) => {
  if (!end) return null;
  const rd = routerData(end.router);
  return (kind === 'dst' ? rd.dstBy : rd.srcBy).get(end.n) || null;
};
// The text of a picked end in its box ("A.5 MV 1").
const boxText = (end, kind) => {
  if (!end) return '';
  const it = itemOf(end, kind);
  const rd = routerData(end.router);
  return it ? `${numText(it)}${it.name ? ` ${it.name}` : ''}` : `${rd.prefix}.${end.n + rd.off}`;
};
// The words under a box: the Router title of the picked end.
const endWords = (end) => {
  if (!end) return '';
  const rd = routerData(end.router);
  return rd.title || `Router ${rd.prefix}`;
};

// One find box for an end. kind is 'dst' (the From end) or 'src' (the To end). def() gives the router that a bare
// number searches. onChange runs after each pick or edit; onEnter and onEscape when the list is closed.
function endBox(kind, id, text, end0, { def, onChange, onEnter, onEscape }) {
  let end = end0;
  const input = el('input', {
    class: 'pb-input',
    id,
    type: 'text',
    placeholder: 'Name or number, such as A.5',
    spellcheck: false,
    autocomplete: 'off',
    role: 'combobox',
    'aria-autocomplete': 'list',
    'aria-expanded': 'false',
  });
  input.value = boxText(end, kind);
  const menu = el('ul', { class: 'pb-menu sode__menu', role: 'listbox', id: `${id}-menu`, 'aria-label': text, hidden: true });
  input.setAttribute('aria-controls', menu.id);
  const words = el('span', { class: 'sode__hint sode__tfind-end', id: `${id}-end` }, endWords(end));
  const set = (next) => {
    end = next;
    words.textContent = endWords(end);
    onChange();
  };
  const cb = combobox(input, menu, {
    source: (t) => boxSearch(kind, t, { def: def(), n: MATCHES }),
    onPick: (it) => {
      input.value = `${numText(it)}${it.name ? ` ${it.name}` : ''}`;
      cb.close();
      set(endOfItem(it, firstUid()));
    },
  });
  input.addEventListener('input', () => {
    cb.refresh();
    set(null);
  });
  input.addEventListener('blur', () => cb.close());
  input.addEventListener('keydown', (e) => {
    if (cb.nav(e)) return;
    if (e.key === 'Enter') {
      // Enter must never submit the Settings form.
      e.preventDefault();
      if (!(cb.open && cb.accept())) onEnter();
      return;
    }
    if (e.key === 'Tab' && !e.shiftKey && cb.open) {
      cb.accept();
      return;
    }
    if (e.key !== 'Escape') return;
    e.preventDefault();
    e.stopPropagation();
    if (cb.open) cb.close();
    else onEscape();
  });
  const label = Object.assign(el('label', null, text), { htmlFor: id });
  const node = el('div', 'sode__field sode__tfind', label, input, menu, words);
  return {
    node,
    input,
    get end() {
      return end;
    },
    // The end to save: the pick, else the first match of the typed text. null with no match.
    resolve() {
      if (end) return end;
      const top = input.value.trim() ? boxSearch(kind, input.value, { def: def(), n: 1 })[0] : null;
      return endOfItem(top, firstUid());
    },
    // The source name at the picked end, for the Tie line name.
    name: () => (itemOf(end, kind)?.name || '').trim(),
  };
}

// t: the tie line to edit, or null for a new one. defRouter: the router of the router filter, or ''. onSave(t) saves
// and gives '' or why not. onClose ends the editor.
export function tieEditor(t, { defRouter = '', onSave, onClose }) {
  const editing = t?.id || '';
  let auto = t ? (itemOf(t.to, 'src')?.name || '').trim() : '';
  const fromDef = () => from.end?.router || defRouter || firstUid();
  const err = el('p', { class: 'sode__presult sode__tedit-err', role: 'alert' });
  const nameBox = el('input', { class: 'pb-input', id: 'tieline-name', maxLength: 32, spellcheck: false, autocomplete: 'off' });
  nameBox.value = t?.name || '';
  const go = () => submit();
  const stop = () => onClose();
  const from = endBox('dst', 'tieline-from', 'From (destination)', t ? { router: t.from.router, n: t.from.n } : null, {
    def: () => defRouter || firstUid(),
    onChange: () => (err.textContent = ''),
    onEnter: go,
    onEscape: stop,
  });
  const to = endBox('src', 'tieline-to', 'To (source)', t ? { router: t.to.router, n: t.to.n } : null, {
    def: () => otherRouter(uids(), fromDef()) || firstUid(),
    onChange: () => {
      err.textContent = '';
      const next = fillName({ name: nameBox.value, auto }, to.name());
      auto = next.auto;
      if (nameBox.value !== next.name) nameBox.value = next.name;
    },
    onEnter: go,
    onEscape: stop,
  });
  nameBox.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      go();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      stop();
    }
  });
  const name = el(
    'div',
    'sode__field sode__tedit-name',
    Object.assign(el('label', { 'data-pb-tip': 'Sode fills it from the source name. You can change it' }, 'Tie line name'), { htmlFor: 'tieline-name' }),
    nameBox,
  );
  const save = btn('sode-btn--primary', editing ? 'Save tie line' : 'Add tie line', go, 'tieline-add');
  const cancel = btn('pb-btn--outline', 'Cancel', stop, 'tieline-cancel');
  const node = el(
    'div',
    { class: 'sode__tedit', role: 'group', 'aria-label': editing ? `Edit tie line ${t.name}` : 'Add tie line' },
    el('div', 'sode__tedit-fields', from.node, to.node, name),
    el('div', 'sode__tielines-acts', save, cancel),
    err,
  );
  let busy = false;
  async function submit() {
    if (busy) return;
    const f = from.resolve();
    const s = to.resolve();
    if (!f)
      return void (err.textContent = from.input.value.trim()
        ? `Not saved: no destination matches "${from.input.value.trim()}".`
        : 'Not saved: find the destination.');
    if (!s) return void (err.textContent = to.input.value.trim() ? `Not saved: no source matches "${to.input.value.trim()}".` : 'Not saved: find the source.');
    const srcName = (itemOf(s, 'src')?.name || '').trim();
    const typed = nameBox.value.trim();
    busy = true;
    save.disabled = true;
    err.textContent = '';
    const why = await onSave({ id: editing, name: typed || srcName, from: f, to: s });
    busy = false;
    save.disabled = false;
    if (why) err.textContent = why;
  }
  return {
    node,
    id: editing,
    focus: () => from.input.focus(),
    // True while focus is in the editor, so a redraw of the list waits.
    get active() {
      return node.contains(document.activeElement);
    },
  };
}
