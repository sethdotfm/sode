// The short name field of the editor's Panel tab. It writes the panel's slug through ed.setFields, so Save sends it
// with the rest. Rules and wording: slugrules.js.
import { el } from './core.js';
import { field } from './formbits.js';
import { cleanSlug, slugProblem, SLUG_HINT } from './slugrules.js';

export function createSlugField(ed) {
  const input = el('input', 'pb-input');
  input.id = 'ed-side-slug';
  input.maxLength = 32;
  input.autocomplete = 'off';
  input.spellcheck = false;
  input.placeholder = 'studio-a';
  const note = el('span', 'sode__hint');
  note.setAttribute('role', 'status');
  const paintNote = () => {
    note.textContent = slugProblem(input.value);
  };
  input.addEventListener('input', () => {
    const clean = cleanSlug(input.value);
    if (clean !== input.value) input.value = clean;
    paintNote();
    // A bad short name stays out of the unsaved fields; the server would refuse it.
    if (!slugProblem(clean)) ed.setFields({ slug: clean });
  });
  const node = field('ed-side-slug', 'Short name', input, SLUG_HINT);
  node.append(note);
  return {
    node,
    update() {
      input.disabled = ed.readOnly();
      if (document.activeElement !== input) input.value = ed.fields().slug || '';
      paintNote();
    },
  };
}
