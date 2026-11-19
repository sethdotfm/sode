// The step-in look (spec #543, Step in). The page border has a fixed colour, never the Branding colour, inside the
// lost-link border. "<name> stepped in" and Step out show below the lost-link bar. Compact view never fades them.
import { el, on } from './core.js';
import { steppedIn, stepOut } from './stepin.js';

const frame = el('div', 'sode__stepframe');
frame.setAttribute('aria-hidden', 'true');
frame.hidden = true;
const name = el('span', 'sode__stepbar-name');
const out = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Step out');
out.type = 'button';
out.id = 'stepbar-out';
out.addEventListener('click', () => stepOut());
const bar = el('div', 'sode__stepbar', name, out);
bar.id = 'stepbar';
bar.setAttribute('role', 'status');
bar.hidden = true;
document.body.append(frame, bar);

let who = '';
function paint() {
  frame.hidden = bar.hidden = !steppedIn();
  name.textContent = `${who} stepped in`;
}
on('access', (L) => {
  who = L.user || '';
  paint();
});
// stepin.js forgets the step-in on a dropped connection without a new access.
on('conn', paint);
