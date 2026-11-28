// A Grid panel's header, laid out as on the main page. On the panel page it shares one band, the deck, with the Levels
// box and the search boxes (gridpanel.css). The run (gridpanelrun.js) makes it, and its stop puts the page back.
import { el, icon, on, S } from './core.js';
import { connBox, paintConn } from './connview.js';
import { layout, onLayout } from './layout.js';
import { setResult, resultNode, dismissResult } from './banner.js';
import { actionsFor, editHref } from './panelsrules.js';
import { bannerStep } from './gridpanelhead.js';

// core.js's on() has no unsubscribe, so one subscription at import paints every head a run has made.
const conns = new Set();
on('conn', () => {
  for (const c of conns) paintConn(c);
});

const iconLink = (cls, name, label) => {
  const n = el('a', `pb-btn pb-btn--ghost pb-btn--icon ${cls}`, icon(name, false));
  n.setAttribute('aria-label', label);
  n.dataset.pbTip = label;
  return n;
};
const rule = (cls) => {
  const r = el('div', `pb-divider pb-divider--vertical ${cls}`);
  r.setAttribute('aria-hidden', 'true');
  return r;
};

// count is the run's "N staged" badge; the note beside Take shows the count, so the deck hides this one. onCopy is
// the page's Copy (gridpanelrun.js).
export function gridHead(ui, count, { onCopy } = {}) {
  const page = !ui.back;
  // The Live Editor shows the editor's page header (panelsbar.js), so its run has no header row of its own. Full screen
  // and the status slot go in that header. The run's title and status block do not show.
  const bar = page ? null : ui.topBar;
  // The panel page puts a rule after the instance name, before the buttons.
  const nameRule = page ? rule('sode-gridpanel__namerule') : null;
  const id = el('div', 'sode-gridpanel__id', el('div', 'sode-gridpanel__name', ...[ui.title, ui.name, nameRule].filter(Boolean)), count);
  ui.head.prepend(id);
  const conn = bar ? null : connBox();
  if (conn) {
    conns.add(conn);
    paintConn(conn);
  }
  const screenCls = ['pb-btn--outline', 'pb-btn--sm'].filter((c) => ui.screen?.classList.contains(c));
  ui.screen?.classList.remove(...screenCls);
  ui.screen?.classList.add('pb-btn--ghost');
  // Settings on Home, as a link: settings.js opens the view at #settings.
  const settings = iconLink('sode-gridpanel__settings', 'settings', 'Settings');
  settings.href = './#settings';
  // The Panels link joins this group (panelfull.js). Kiosk mode and Full width stay for their code; gridpanel.css hides them.
  // Order on the panel page: Settings, Panels, Edit or Copy, a rule, Full screen. Edit and Copy use the rules of /panels (actionsFor, see access).
  // In the Live Editor: a rule and Full screen, after the page header's links.
  const btns = [ui.kiosk, ui.screen, ui.full].filter(Boolean);
  const after = btns.at(-1)?.nextSibling ?? null;
  const home = page ? el('span', 'sode-gridpanel__home') : null;
  const edit = page ? iconLink('sode-gridpanel__edit', 'pencil', 'Edit panel') : null;
  const copy = page ? el('button', 'pb-btn pb-btn--ghost pb-btn--icon sode-gridpanel__copy', icon('copy', false)) : null;
  if (copy) {
    copy.type = 'button';
    copy.setAttribute('aria-label', 'Save a copy');
    copy.dataset.pbTip = 'Save a copy';
    copy.addEventListener('click', () => onCopy?.());
  }
  for (const n of [edit, copy]) if (n) n.hidden = true;
  const parts = page ? [settings, home, edit, copy, rule('sode-gridpanel__toolrule'), ...btns] : [rule('sode-gridpanel__toolrule'), ...btns];
  const tools = el('div', 'sode-gridpanel__tools', ...parts);
  if (bar) bar.querySelector('.sode-bar__tools').after(tools);
  else ui.head.insertBefore(tools, after);
  // On the panel page the link joins the buttons even with the Title bar setting off; panelfull.js moves it out in kiosk mode.
  if (ui.home && (home || ui.head.contains(ui.home))) (home || tools).append(ui.home);
  ui.tools = home || tools;
  // The deck: the header and the Levels box are display: contents in it, so their parts are cells of one grid.
  const box = ui.levels;
  const finds = el('div', 'sode-gridpanel__finds');
  const deck = el('div', 'sode-gridpanel__deck');
  // The Live Editor's status slot is outside the header, since the Title bar setting hides the header. Its height is
  // fixed (gridpanel.css), so a line or the banner that shows or goes never moves the grid.
  const slot = page ? null : el('div', 'sode-gridpanel__slot');
  // The panel page's message box, in the gap before Clear and Take. Its size is the gap's (gridpanel.css), so what it
  // shows never moves the grid. A result (res) shows over the "This panel was changed" banner.
  const res = page ? el('div', 'sode-gridpanel__res') : null;
  // The count of messages that a higher one held back (bannerStep), in the box's corner.
  const held = page ? el('span', 'pb-badge sode-gridpanel__msg-held') : null;
  if (held) held.hidden = true;
  // One live region gets only each message's words, so a screen reader reads a result one time and never the buttons.
  const said = page ? el('div', 'pb-sr-only sode-gridpanel__said') : null;
  said?.setAttribute('role', 'status');
  const msg = page ? el('div', 'sode-gridpanel__msg', res, held, said) : null;
  const top = bar ? null : el('div', 'sode-gridpanel__headrow');
  ui.head.before(deck);
  // A rule follows the Lock chip's "Signed in as" (gridpanel.css shows it only beside the chip).
  if (top) top.append(rule('sode-gridpanel__whorule'), ui.head, ...[slot, conn?.root].filter(Boolean));
  else {
    ui.head.remove();
    // In the gap before Inbox, where a message that shows or goes moves nothing.
    bar.querySelector('.sode-inbox__slot').before(slot);
  }
  // The Live Editor's bar (editorlive.js) goes at the top of the deck, under the page header.
  if (ui.liveBar) deck.classList.add('sode-gridpanel__deck--live');
  deck.append(
    ...(top ? [top] : []),
    ...(ui.liveBar ? [ui.liveBar] : []),
    ...(box ? [box] : []),
    rule('sode-gridpanel__rule sode-gridpanel__rule--1'),
    finds,
    rule('sode-gridpanel__rule sode-gridpanel__rule--2'),
  );
  if (msg) deck.append(rule('sode-gridpanel__rule sode-gridpanel__rule--3'), msg);
  // The phone's band (gridpanel.css): More and the levels in words open the rest of the deck under it.
  const more = el('button', 'pb-btn pb-btn--ghost pb-btn--icon sode-gridpanel__more', icon('menu', false));
  more.type = 'button';
  more.id = 'panel-more';
  more.setAttribute('aria-label', 'More');
  more.setAttribute('aria-expanded', 'false');
  const lvText = document.createTextNode('');
  const lv = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode-gridpanel__lv', el('span', 'sode-gridpanel__lv-word', 'Levels'), lvText);
  lv.type = 'button';
  const open = (on) => {
    deck.toggleAttribute('data-open', on);
    more.setAttribute('aria-expanded', String(on));
  };
  const toggle = () => open(!deck.hasAttribute('data-open'));
  more.addEventListener('click', toggle);
  lv.addEventListener('click', toggle);
  // text is the levels a press stages, in words; the band is the only place they show while the rest is closed.
  const paintLevels = (text) => {
    const t = text || 'none';
    if (lvText.data !== t) lvText.data = t;
  };
  box?.prepend(more, lv);
  // The desktop layout moves the banner into the deck, and in the Live Editor the status line too. On the panel page it
  // moves the buttons into the header row, so they show with the Title bar setting off. The phone keeps them in place.
  const moved = (page ? [ui.update] : [ui.status, ui.update]).filter(Boolean).map((n) => [n, n.parentNode, n.nextSibling]);
  const toolsHome = [tools.parentNode, tools.nextSibling];
  // Back in reverse order, since the status line's next sibling is the banner.
  const place = (l) => {
    if (l !== 'phone') {
      (msg || slot).append(...moved.map(([n]) => n));
      if (page) ui.head.after(tools);
    } else {
      for (const [n, home, next] of [...moved].reverse()) if (n.parentNode !== home) home.insertBefore(n, next);
      if (page && tools.parentNode !== toolsHome[0]) toolsHome[0].insertBefore(tools, toolsHome[1]);
    }
  };
  place(layout());
  const offLayout = onLayout(place);
  // With the buttons out of the header, kiosk mode alone hides them (panelfull.js).
  if (page) ui.toolsShown = (kiosk) => !kiosk && (tools.parentNode !== ui.head || !ui.head.hidden);

  // The banner in msg: the main page's result banner and its timed dismiss (banner.js). bannerStep (gridpanelhead.js)
  // decides what shows. held marks a message from the run's state, which goes when the state clears. The others
  // follow the main page's rules: an ok one goes on its timer, an error by its X.
  let shown = null;
  let r = null;
  let updating = false;
  const back = new Set();
  const paintBack = () => {
    held.hidden = !back.size;
    if (back.size) held.textContent = `${back.size} ${back.size === 1 ? 'message' : 'messages'} not shown`;
  };
  const drop = () => {
    if (r) dismissResult(r);
    r = null;
    shown = null;
    back.clear();
    if (held) paintBack();
  };
  // behind is the key of a message that the state's message (held) keeps from showing.
  const say = ({ text, kind, key, held: isHeld, behind }) => {
    if (!msg || layout() === 'phone') {
      drop();
      return false;
    }
    // A "This panel was changed" banner that comes up is the newest message, so it replaces a result.
    const up = !ui.update.hidden;
    if (up && !updating) drop();
    updating = up;
    // dismissResult emits nothing, so a closed or timed out result shows here as S.result moved on.
    if (shown && !shown.gone && S.result !== r) {
      shown = { ...shown, gone: true };
      r = null;
      back.clear();
    }
    if (behind !== undefined && shown && !shown.gone) back.add(behind);
    const step = bannerStep(shown, { text, kind, key, held: isHeld });
    if (step.back) back.add(key);
    if (step.changed) {
      if (r) dismissResult(r);
      r = null;
      shown = step.shown;
      if (shown) {
        setResult({ kind: shown.kind, title: shown.title, text: shown.line });
        r = S.result;
        res.replaceChildren(resultNode(r));
        said.textContent = shown.line ? `${shown.title}. ${shown.line}` : shown.title;
      } else back.clear();
    }
    paintBack();
    return true;
  };
  return {
    // The run's search boxes go here (gridpanelfindbox.js).
    finds,
    // The run calls this on each render and on each Levels box change.
    paintLevels,
    // msg is { text, kind ('ok' or 'error'), key, held, behind }. It gives false on the phone and in the Live Editor, which have
    // no banner here, so the run shows its status line.
    say,
    // p is the panel as read; mayOwn is true when a signed-in user runs the page, since only a user owns a panel.
    access(p, mayOwn) {
      if (!edit) return;
      const acts = actionsFor(p);
      edit.href = editHref(p.id);
      edit.hidden = !acts.includes('edit');
      copy.hidden = !(mayOwn && acts.includes('copy'));
    },
    stop() {
      drop();
      more.remove();
      lv.remove();
      conns.delete(conn);
      delete ui.tools;
      delete ui.toolsShown;
      id.replaceWith(ui.title, ui.name);
      ui.screen?.classList.remove('pb-btn--ghost');
      ui.screen?.classList.add(...screenCls);
      offLayout();
      place('phone');
      if (bar) {
        tools.remove();
        slot.remove();
      } else tools.replaceWith(...btns);
      if (ui.home && (ui.home.parentNode === tools || ui.home.parentNode === home)) ui.head.append(ui.home);
      if (ui.liveBar) deck.before(ui.liveBar);
      deck.replaceWith(ui.head);
      if (box) ui.grid.before(box);
    },
  };
}
