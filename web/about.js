import { el, S, icon, on } from './core.js';

const link = (href, tip, cls, ...kids) => {
  const a = el('a', cls, ...kids);
  a.href = href;
  a.target = '_blank';
  a.rel = 'noopener noreferrer';
  a.dataset.pbTip = tip;
  return a;
};

// id: the Communication Log link's id; the save bar's copy is the one settings search finds (#open-log).
export function fillAbout(host, { id } = {}) {
  const version = el('span', 'sode__hint');
  version.dataset.version = '';
  const github = link('https://github.com/sethdotfm/sode', 'GitHub', 'pb-tip--top', icon('github', false));
  github.setAttribute('aria-label', 'GitHub');
  const bug = link('https://github.com/sethdotfm/sode/issues/new', 'New GitHub issue', 'pb-tip--top', 'Report a bug');
  const logs = el('a', 'pb-btn pb-btn--outline sode__logs', icon('external-link', false), 'Communication Log');
  logs.href = 'log';
  if (id) {
    logs.id = id;
    logs.dataset.find = id;
    logs.dataset.label = 'Communication Log';
  }
  host.append(el('span', 'sode__about', version, el('span', 'sode__links', github, bug)), logs);
  paintVersion();
}

function paintVersion() {
  for (const v of document.querySelectorAll('[data-version]')) v.textContent = S.version ? `Sode ${S.version}` : '';
}
on('hello', paintVersion);
