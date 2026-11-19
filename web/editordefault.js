// The Cell tab's Default artwork row. It makes the cell's artwork and Hide text its item's default, clears that
// default, or puts the cell back on it. Wording is defaultNote in editorrules.js. The server refuses a user who is not
// Trusted, and the reason shows below.
import { el } from './core.js';
import { btn } from './formbits.js';
import { sessionFetch, jsonCall } from './sessionkey.js';
import { defaultFor, artOf, artBOf } from './panellook.js';
import { defaultNote, defaultBody, takesDefault } from './editorrules.js';

const send = jsonCall(sessionFetch, {
  down: 'Sode did not answer, so the Default artwork was not changed.',
  failed: (s) => `The Default artwork was not changed (${s}).`,
});

// patch(fields) changes the selected cell; done() repaints the Cell tab after the defaults are read again.
export function createDefaultRow({ ed, patch, done }) {
  const text = el('p', 'sode__hint');
  const warn = el('div', 'pb-note pb-note--warn');
  warn.id = 'ed-default-warn';
  const said = el('p', 'sode__hint');
  said.setAttribute('role', 'status');
  let cell = null;

  async function change(set) {
    const body = defaultBody(cell, ed.panel()?.id || '', set);
    said.textContent = '';
    try {
      await send('api/v1/default-artwork', { method: set ? 'PUT' : 'DELETE', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
      said.textContent = set ? 'Saved as the Default artwork.' : 'Default artwork cleared.';
      await ed.loadDefaults();
      done();
    } catch (e) {
      said.textContent = e.message;
    }
  }

  const setBtn = btn('pb-btn--outline pb-btn--sm', 'Set as default', () => change(true));
  setBtn.id = 'ed-default-set';
  const clearBtn = btn('pb-btn--outline pb-btn--sm', 'Clear default', () => change(false));
  clearBtn.id = 'ed-default-clear';
  // Drops what the cell sets itself, so it shows the default again, Hide text included.
  const follow = btn('pb-btn--outline pb-btn--sm', 'Use default', () => patch({ art: undefined, artB: undefined, hideText: undefined }));
  follow.id = 'ed-default-use';
  const node = el(
    'div',
    'sode__field',
    el('span', 'sode__field-label', 'Default artwork'),
    text,
    warn,
    el('div', 'sode-side__row', setBtn, follow, clearBtn),
    said,
  );

  return {
    node,
    // c is the stored cell, never one drawn with its default.
    update(c) {
      if (c !== cell && (c?.kind !== cell?.kind || c?.n !== cell?.n || c?.id !== cell?.id || c?.name !== cell?.name)) said.textContent = '';
      cell = c;
      const item = takesDefault(c);
      node.hidden = !item;
      if (!item) return;
      const d = defaultFor(c, ed.defaults());
      const own = !!artOf(c) || !!artBOf(c);
      const n = defaultNote(d, own, ed.names().routerName(c.kind, c.n));
      text.textContent = n.text || 'No Default artwork for this item yet.';
      warn.textContent = n.warn;
      warn.hidden = !n.warn;
      const ro = ed.readOnly();
      setBtn.disabled = ro || !own;
      clearBtn.disabled = ro || !d;
      follow.hidden = !d;
      follow.disabled = ro || (!own && typeof c.hideText !== 'boolean');
    },
  };
}
