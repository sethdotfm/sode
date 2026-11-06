// The bottom row of Settings, built once for its two hosts (the locked view's
// .sode__lock-exit and the save bar's .sode__foot): the Sode version, the
// GitHub and bug-report links, and View Logs, in that order after whatever the
// host already holds (Exit, or a spacer).

import { el, S, icon, on } from './core.js';

const link = (href, tip, cls, ...kids) => {
  const a = el('a', cls, ...kids);
  a.href = href;
  a.target = '_blank';
  a.rel = 'noopener noreferrer';
  a.dataset.pbTip = tip;
  return a;
};

// id: View Logs' id (the save bar's copy is the searchable one, #open-log).
export function fillAbout(host, { id } = {}) {
  const version = el('span', 'sode__hint');
  version.dataset.version = '';
  const github = link('https://github.com/sethdotfm/sode', 'GitHub', 'pb-tip--top', icon('github', false));
  github.setAttribute('aria-label', 'GitHub');
  const bug = link('https://github.com/sethdotfm/sode/issues/new', 'Open an issue to report a bug or suggest a feature!', 'pb-tip--top', 'Report a bug');
  const logs = el('a', 'pb-btn pb-btn--outline sode__logs', icon('external-link', false), 'View Logs');
  logs.href = 'log';
  if (id) {
    logs.id = id;
    logs.dataset.find = id;
    logs.dataset.label = 'View Logs';
  }
  host.append(el('span', 'sode__about', version, el('span', 'sode__links', github, bug)), logs);
  paintVersion();
}

function paintVersion() {
  for (const v of document.querySelectorAll('[data-version]')) v.textContent = S.version ? `Sode ${S.version}` : '';
}
on('hello', paintVersion);
