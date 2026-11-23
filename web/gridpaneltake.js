// The Grid panel's Take button (the main page's amber Take) and Clear note. They are at the end of the Levels box,
// since Kiosk mode hides the header, not the box. An embedded run has neither.
import { el } from './core.js';
import { takeButton } from './gridpanelphone.js';
import { clearNote } from './gridpanelnote.js';
import { joinPulse } from './gridpanelnodes.js';

export function gridTake(ctx, ui, { onTake, onClear }) {
  const take = ctx.embedded ? null : el('button', 'pb-btn pb-btn--commit sode-panel__take sode-gridpanel__take', 'Take');
  if (take) {
    take.type = 'button';
    take.id = 'panel-take';
    take.hidden = true;
    take.addEventListener('click', onTake);
    ui.levels?.append(take);
  }
  const clearer = take?.parentNode ? clearNote(take.parentNode, take, onClear) : null;
  return {
    // state is takeButton's { mode, staged, busy } and canSend: with a press policy (gridpaneladd.js) there is no Take.
    // note and canClear are the Clear note's.
    paint(state, note, canClear) {
      if (take) {
        const t = takeButton(state);
        const shown = t.shown && state.canSend;
        if (take.hidden === shown) take.hidden = !shown;
        if (take.disabled !== t.disabled) take.disabled = t.disabled;
        if (take.textContent !== t.label) take.textContent = t.label;
        const starts = t.armed && !take.classList.contains('sode-panel__take--armed');
        take.classList.toggle('sode-panel__take--armed', t.armed);
        if (starts) joinPulse(take);
      }
      clearer?.paint(note, canClear);
    },
    stop() {
      take?.remove();
      clearer?.remove();
    },
  };
}
