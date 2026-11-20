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

// Fills host with a live panel; onBack goes back to editing. A Grid panel also gets panel.html's instance name, Levels box
// and take confirm, made again for each run because keyconfirm.js never removes its listeners.
// edit: a Grid panel's { bar, open, aside, onPicks, leave } (editorlive.js). Its bar goes under the Grid head row, which
// gets Full screen; stopScreen ends that button.
export function liveUi(host, onBack, edit = null) {
  const back = el('button', `pb-btn pb-btn--outline ${edit ? 'pb-btn--lg' : 'pb-btn--sm'}`, 'Back to editing');
  back.id = 'ed-live-back';
  back.type = 'button';
  back.addEventListener('click', onBack);
  const title = el('h2', 'sode-panels__title');
  const name = el('span');
  name.hidden = true;
  const head = el('div', 'sode-panels__head', title, name, el('span', 'sode-panels__spacer'));
  if (edit) edit.open.before(back);
  else head.append(back);
  const note = el('p', 'sode__hint');
  const status = el('p', 'sode-panel__status');
  status.setAttribute('role', 'status');
  const updateText = el('span');
  const snooze = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Snooze 60 s');
  snooze.type = 'button';
  const applyNow = el('button', 'pb-btn pb-btn--ok pb-btn--sm', 'Apply now');
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
  // A Grid panel's list drawer sits beside the grid, under the band, so the grid narrows and Take stays in reach.
  const field = edit ? el('div', 'sode-live__row', grid, edit.aside) : grid;
  const main = el('div', 'sode-live', head, note, el('div', 'sode-panel__foot', status, update), levels, field, offline, takeConfirm());
  host.replaceChildren(...(edit ? [edit.bar] : []), main);
  // home: the run puts it with the header buttons, where the panel page has its Panels link.
  return {
    main,
    head,
    title,
    name,
    home: edit ? null : back,
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
    leave: edit?.leave,
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
    el('div', 'pb-modal__footer', btn('pb-btn--outline', 'Cancel', null, 'confirm-cancel'), btn('pb-btn--danger', 'Take', null, 'confirm-ok')),
  );
  return el('div', { class: 'pb-modal-backdrop sode__scrim', id: 'confirm', hidden: true }, dialog);
}
