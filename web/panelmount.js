// The elements runPanel (panelrun.js) draws into. The panel page has them as ids in panel.html; the editor's
// live test builds the same shape inside #ed-live-view, so the one runtime serves both.
import { $, el } from './core.js';
import { btn } from './formbits.js';
import { dialogAttrs } from './modal.js';
import { initScreen } from './panelfull.js';

export function pageUi() {
  return {
    main: document.querySelector('.sode-panels'),
    head: document.querySelector('.sode-panels__head'),
    title: $('panel-title'),
    note: $('panels-note'),
    signin: $('panels-signin'),
    device: $('panel-device'),
    name: $('panel-instance'),
    foot: $('panel-foot'),
    status: $('panel-status'),
    update: $('panel-update'),
    updateText: $('panel-update-text'),
    snooze: $('panel-snooze'),
    applyNow: $('panel-apply'),
    bar: $('panel-update-bar'),
    grid: $('panel-grid'),
    offline: $('panel-offline'),
    offlineDetail: $('panel-offline-detail'),
    full: $('panel-full'),
    screen: $('panel-screen'),
    kiosk: $('panel-kiosk'),
    home: $('panel-home'),
    levels: $('panel-levels'),
  };
}

// Fills host with a live panel; onBack goes back to the editor. A Grid panel also gets panel.html's instance name, Levels box
// and take confirm, made again for each run because keyconfirm.js never removes its listeners.
// top is the Live Editor's bar (editorlive.js), which gets Back to the editor. edit: a Grid panel's { bar, open, aside,
// onPicks, leave, topBar } (editorlive.js). Its bar goes over the band, and Full screen goes in the editor's page header
// (topBar); stopScreen ends that button.
export function liveUi(host, onBack, edit, top) {
  const back = el('button', 'pb-btn pb-btn--outline pb-btn--lg', 'Back to the editor');
  back.id = 'ed-live-back';
  back.type = 'button';
  back.addEventListener('click', onBack);
  const title = el('h2', 'sode-panels__title');
  const name = el('span');
  name.hidden = true;
  const head = el('div', 'sode-panels__head', title, name, el('span', 'sode-panels__spacer'));
  if (edit) edit.open.before(back);
  else top.append(back);
  // A Buttons panel's title would repeat the panel picker over it, and the bar says what runs.
  head.hidden = !edit;
  const note = el('p', 'sode__hint');
  const status = el('p', 'sode-panel__status');
  status.setAttribute('role', 'status');
  const updateText = el('span');
  const snooze = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Snooze 60 s');
  snooze.type = 'button';
  const applyNow = el('button', 'pb-btn sode-btn--primary pb-btn--sm', 'Apply now');
  applyNow.type = 'button';
  const bar = el('div', 'pb-progress sode__banner-time');
  const update = el(
    'div',
    'sode-panel__update',
    el('div', 'pb-alert pb-alert--warn', el('div', 'sode__banner-body', el('div', 'pb-alert__title', 'This panel was changed'), updateText), snooze, applyNow),
    bar,
  );
  update.setAttribute('role', 'status');
  const grid = el('div', 'sode-panel__grid');
  const offlineDetail = el('p', 'sode-panel__offline-detail');
  const offline = el('div', 'sode-panel__offline pb-state-offline', el('p', 'sode-panel__offline-title', 'Offline'), offlineDetail);
  offline.setAttribute('role', 'alert');
  for (const n of [status, update, bar, offline]) n.hidden = true;
  const screen = edit ? el('button', 'pb-btn pb-btn--ghost pb-btn--icon') : null;
  if (screen) {
    screen.type = 'button';
    screen.id = 'ed-live-screen';
  }
  const stopScreen = screen ? initScreen(screen) : () => {};
  note.hidden = true;
  const levels = levelsBox();
  // A Grid panel's list drawer sits beside the grid, under the band, so the grid narrows and Take stays in reach. A
  // Buttons panel's grid fits the room left under the bar (panels.css .sode-live__fit).
  const field = edit ? el('div', 'sode-live__row', grid, edit.aside) : el('div', 'sode-live__fit', grid);
  const main = el('div', 'sode-live', head, note, el('div', 'sode-panel__foot', status, update), levels, field, offline, takeConfirm());
  host.replaceChildren(top, main);
  return {
    main,
    head,
    title,
    name,
    home: null,
    note,
    status,
    update,
    updateText,
    snooze,
    applyNow,
    bar,
    grid,
    offline,
    offlineDetail,
    back,
    levels,
    aside: edit?.aside,
    onPicks: edit?.onPicks,
    screen,
    stopScreen,
    liveBar: edit?.bar,
    topBar: edit?.topBar,
    leave: edit?.leave,
    // The Live Editor's keys and its hold on the run's picks (editorlivemenu.js); the panel page has neither.
    editKey: edit?.editKey,
    useRun: edit && ((r) => edit.useRun(r, grid)),
  };
}

// Keep in step with #panel-levels in panel.html. Only a Grid panel and the Grid popup show it (gridpanellevelbox.js).
export function levelsBox() {
  const input = el('input', 'pb-input sode__input');
  Object.assign(input, { id: 'panel-lvl', spellcheck: false, autocomplete: 'off' });
  input.setAttribute('aria-keyshortcuts', 'f');
  const pick = el('div', 'sode__levels');
  pick.id = 'panel-lvl-pick';
  pick.setAttribute('role', 'group');
  pick.setAttribute('aria-label', 'Pick levels');
  const all = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode__all', el('span', null, 'Select'), el('span', null, 'All'));
  Object.assign(all, { type: 'button', id: 'panel-lvl-all', tabIndex: -1 });
  all.dataset.pbTip = 'All levels, or none';
  const label = el('label', 'sode__label', 'Levels');
  label.htmlFor = 'panel-lvl';
  const box = el('div', 'sode__box sode__box--levels sode-gridpanel__levels', el('div', 'sode__box-head', label), input, el('div', 'sode__picker', pick, all));
  box.id = 'panel-levels';
  box.hidden = true;
  return box;
}

// Keep in step with #confirm in panel.html; the ids are keyconfirm.js's.
function takeConfirm() {
  const dialog = el(
    'div',
    { class: 'pb-modal', id: 'confirm-dialog', ...dialogAttrs('confirm-title') },
    el('div', 'pb-modal__header pb-modal__header--error', el('span', { class: 'pb-modal__title', id: 'confirm-title' }, 'Confirm take')),
    el('div', { class: 'pb-modal__body sode__confirm', id: 'confirm-body' }),
    el('div', 'pb-modal__footer', btn('pb-btn--outline', 'Cancel', null, 'confirm-cancel'), btn('pb-btn--commit', 'Take', null, 'confirm-ok')),
  );
  return el('div', { class: 'pb-modal-backdrop sode__scrim', id: 'confirm', hidden: true }, dialog);
}
