// A status in words only: the owner wants no colored square beside a status (base.css). data-tone keeps the tone
// (ok, info, warn, error, idle) for code that sorts by it.
import { el } from './core.js';

export function statusText(tone, ...kids) {
  const s = el('span', 'pb-status', ...kids);
  s.dataset.tone = tone;
  return s;
}
