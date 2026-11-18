// The phone's desktop-only message (spec #543, Phone). Use desktop layout sets This browser's layout, so a large
// tablet can still go on; layout.js redraws every page that listens to onLayout.
import { el } from './core.js';
import { setPref } from './device.js';

// action reads after "to", for example 'edit panels'. lead: an optional first line.
export function desktopOnly(action, lead = '') {
  const go = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Use desktop layout');
  go.type = 'button';
  go.dataset.desktopLayout = '';
  go.addEventListener('click', () => setPref('layout', 'desktop'));
  return el(
    'div',
    'sode__desktop-only',
    lead ? el('p', 'sode__desktop-only-lead', lead) : null,
    el('p', 'sode__hint', `Open Sode on a computer to ${action}.`),
    go,
  );
}
