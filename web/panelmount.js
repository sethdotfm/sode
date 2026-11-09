// The elements runPanel (panelrun.js) draws into. The panel page has them as ids in panel.html; the editor's
// live test builds the same shape inside #ed-live-view, so the one runtime serves both.
import { $, el } from './core.js';

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
    grid: $('panel-grid'),
    offline: $('panel-offline'),
    offlineDetail: $('panel-offline-detail'),
    full: $('panel-full'),
  };
}

// Fills host with a live panel. onBack is the way out to editing.
export function liveUi(host, onBack) {
  const back = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Back to editing');
  back.id = 'ed-live-back';
  back.type = 'button';
  back.addEventListener('click', onBack);
  const title = el('h2', 'sode-panels__title');
  const head = el('div', 'sode-panels__head', title, el('span', 'sode-panels__spacer'), back);
  const note = el('p', 'sode__hint');
  const status = el('p', 'sode-panel__status');
  status.setAttribute('role', 'status');
  const updateText = el('span');
  const snooze = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Snooze 60 s');
  snooze.type = 'button';
  const update = el(
    'div',
    'pb-alert pb-alert--warn sode-panel__update',
    el('div', 'sode__banner-body', el('div', 'pb-alert__title', 'This panel was changed'), updateText),
    snooze,
  );
  update.setAttribute('role', 'status');
  const grid = el('div', 'sode-panel__grid');
  const offlineDetail = el('p', 'sode-panel__offline-detail');
  const offline = el('div', 'sode-panel__offline pb-state-offline', el('p', 'sode-panel__offline-title', 'Offline'), offlineDetail);
  offline.setAttribute('role', 'alert');
  for (const n of [status, update, offline]) n.hidden = true;
  note.hidden = true;
  const main = el('div', 'sode-live', head, note, status, update, grid, offline);
  host.replaceChildren(main);
  return { main, head, title, note, status, update, updateText, snooze, grid, offline, offlineDetail, back };
}
