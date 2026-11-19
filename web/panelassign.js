// The Assign to modal on the panels page. The server checks who can (a Trusted user or higher) and sends the key to the instance alone.
import { el, icon } from './core.js';
import { btn, segmented } from './formbits.js';
import { question } from './modal.js';
import { assignRows, assignWays, rankAssign, curRow, stepRow, enterRow, clashOf, assignNameProblem } from './panelassignrules.js';
import { MON_POLL_MS } from './panelsmon.js';
import { INSTANCE_MAX } from './panelkeyrules.js';
import { resultStep } from './keys.js';

// The name clash dialog, over the Assign to modal or the panels page's rename. go(name) sends again under the edited suggestion.
export function askName(panel, clashed, suggest, go, kind = 'assign') {
  const renaming = kind === 'rename';
  const input = el('input', {
    class: 'pb-input',
    id: 'assign-name',
    autocomplete: 'off',
    spellcheck: false,
    value: suggest,
    maxLength: INSTANCE_MAX,
    'aria-label': `Instance name on ${panel.name}`,
  });
  const err = el('div', { class: 'pb-note pb-note--error', role: 'alert', hidden: true });
  function submit() {
    const problem = assignNameProblem(input.value, clashed);
    err.textContent = problem;
    err.hidden = !problem;
    if (problem) return input.focus();
    q.close(null);
    go(input.value.trim());
  }
  const q = question({
    back: document.activeElement,
    titleId: 'assign-name-title',
    title: `Name taken on ${panel.name}`,
    body: [input, err],
    foot: [
      btn('pb-btn--outline', 'Cancel', () => q.close(null)),
      btn('pb-btn--ok', [icon(renaming ? 'text-cursor-input' : 'send'), renaming ? ' Rename' : ' Assign'], submit, 'assign-name-go'),
    ],
  });
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      submit();
    }
  });
  input.focus();
  input.select();
}

// Answers true for Send anyway; Cancel, Escape and the Lock answer null.
function askAnyway(panel, row) {
  const cancel = btn('pb-btn--outline', 'Cancel', () => q.close(null));
  const q = question({
    back: document.activeElement,
    titleId: 'assign-anyway-title',
    title: `${panel.name} is not shared with ${row.user}`,
    body: [el('p', 'sode__hint', `${row.device} is signed in as ${row.user}. It will show no panel.`)],
    foot: [cancel, btn('', [icon('send'), ' Send anyway'], () => q.close(true), 'assign-anyway')],
  });
  cancel.focus();
  return q.answer;
}

// ctx: {api, panels() (every panel the page has, with instances where the user can edit them), onDone() (re-reads them)}.
export function openAssign(panel, ctx) {
  const opener = document.activeElement;
  let busy = false;
  const note = el('span', { class: 'sode__hint', role: 'status' });
  const say = (t) => (note.textContent = t);
  const find = el('input', {
    class: 'pb-input',
    type: 'search',
    id: 'assign-find',
    placeholder: 'Find an instance',
    autocomplete: 'off',
    'aria-label': 'Find an instance',
  });
  let cur = '';
  let shownRows = [];

  const ways = assignWays(panel);
  let way = ways[0];
  const intro = el('p', 'sode__hint');
  const sayIntro = () =>
    (intro.textContent =
      way === 'user'
        ? `The instance you pick opens ${panel.name} immediately and shows its sign-in, where the person there types their own PIN. Its key for the panel it shows now keeps working.`
        : `The instance you pick opens ${panel.name} immediately, with a key of its own for it. Its key for the panel it shows now keeps working.`);
  sayIntro();
  const choice =
    ways.length > 1
      ? segmented(
          'assign-way',
          'Opens with',
          [
            ['client', 'Its own key'],
            ['user', 'Sign in'],
          ],
          (v) => {
            way = v;
            sayIntro();
            paint();
          },
        )
      : null;
  choice?.set(way);

  const list = el('div', 'sode-ov__people');
  list.id = 'assign-list';
  let shown = '';
  // The page re-reads the instances every MON_POLL_MS; only a change is redrawn, so a button that a person presses stays.
  function paint(force = true) {
    const all = assignRows(ctx.panels(), panel.id, Date.now(), way === 'user');
    const rows = rankAssign(all, find.value);
    cur = curRow(rows, cur);
    shownRows = rows;
    const now = JSON.stringify([rows, busy, cur]);
    if (!force && now === shown) return;
    shown = now;
    list.replaceChildren();
    if (!rows.length) return list.append(el('p', 'sode__hint', all.length ? 'No instance matches.' : 'No panel page open in the last 24 hours.'));
    for (const r of rows) {
      const b = el('button', 'pb-btn pb-btn--outline pb-btn--sm', icon('send'), ' Assign');
      b.type = 'button';
      b.dataset.row = r.key;
      b.dataset.from = r.panel;
      b.dataset.instance = r.device;
      if (r.keyless) b.dataset.keyless = r.keyless;
      b.disabled = busy || !r.assignable;
      const facts = [r.addr, r.state, r.uptime].filter(Boolean).join(', ');
      const who = el(
        'span',
        `sode-ov__who${r.assignable ? '' : ' sode-ov__muted'}`,
        el('span', null, el('strong', null, r.device), el('span', 'sode__hint', ` on ${r.panelName}`)),
        el('span', 'sode__hint', facts),
      );
      if (r.notShared) who.append(el('span', null, el('span', 'pb-badge pb-badge--warn', `Not shared with ${r.user}`)));
      const item = el('div', 'sode-ov__cp', who, b);
      item.dataset.key = r.key;
      if (r.key === cur) item.setAttribute('aria-current', 'true');
      list.append(item);
    }
  }

  const q = question({
    back: opener,
    titleId: 'assign-title',
    title: `Assign to ${panel.name}`,
    cls: 'sode-ov__share',
    body: [intro, ...(choice ? [choice.node] : []), find, list],
    foot: [note, btn('pb-btn--outline', 'Close', () => q.close(null))],
  });
  const { root } = q;
  const timer = setInterval(() => paint(false), MON_POLL_MS);
  q.answer.then(() => clearInterval(timer));

  // name is the instance's name on the target, chosen in the clash dialog; '' keeps its own.
  async function send(row, name = '') {
    if (!row.assignable || busy) return;
    if (!name && row.notShared && !(await askAnyway(panel, row))) return;
    if (busy || !root.isConnected) return;
    busy = true;
    paint();
    say('');
    let clash = null;
    try {
      const r = await ctx.api(`api/v1/panels/${panel.id}/assign`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...(row.keyless ? { keyless: row.keyless } : { from: row.panel, instance: row.device }), name, as: way }),
      });
      say(`Sent ${r.instance} to ${panel.name}.`);
      await ctx.onDone();
    } catch (err) {
      clash = clashOf(err);
      say(clash === null ? err.message : '');
    }
    busy = false;
    if (!root.isConnected) return;
    paint();
    if (clash !== null) askName(panel, name || row.device, clash, (chosen) => send(row, chosen));
  }

  list.addEventListener('click', (e) => {
    const b = e.target.closest('[data-row]');
    const row = b && shownRows.find((r) => r.key === b.dataset.row);
    if (row) send(row);
  });

  find.addEventListener('input', () => paint());
  find.addEventListener('keydown', (e) => {
    const step = resultStep(e, shownRows.length > 0);
    if (step) {
      e.preventDefault();
      cur = stepRow(shownRows, cur, step);
      paint();
      list.querySelector('[aria-current="true"]')?.scrollIntoView({ block: 'nearest' });
    } else if (e.key === 'Enter' && !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey) {
      e.preventDefault();
      const row = enterRow(shownRows, cur);
      if (row) send(row);
    }
  });

  paint();
  find.focus();
}
