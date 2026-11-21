// The link stays in the address until itemLinkStep can judge it, so a reload under a lock keeps it.
import { S, on, emit } from './core.js';
import { showResult } from './plans.js';
import { profileName } from './profile.js';
import { allPresets } from './presetstore.js';
import { readItemLink, itemLinkStep, OTHER_INSTALL, MISSING_PRESET } from './itemlink.js';

let link = null;
let access = null; // lock.js's lock state, after it has said
let namesIn = false;
let lockUp = false; // lock.js's overlay is up (its 'lockcover' event)

function tryOpen() {
  if (!link) return;
  const r = itemLinkStep(link, {
    install: S.installUid,
    access,
    pageLocked: lockUp,
    namesIn,
    profileWait: !!access?.user && !profileName(),
    presets: allPresets(),
  });
  if (r.step === 'wait') return;
  history.replaceState(null, '', location.pathname + location.search + link.hash);
  link = null;
  if (r.step === 'other') return showResult('warn', 'Item link', OTHER_INSTALL);
  if (r.step === 'missing') return showResult('warn', 'Item link', MISSING_PRESET);
  emit('presetPopup', { scope: r.preset.scope, id: r.preset.id });
}

function takeLink() {
  link = readItemLink(location.hash);
  tryOpen();
}

export function openItemLinks() {
  on('helloData', () => {
    namesIn = true;
    tryOpen();
  });
  on('access', (l) => {
    access = l;
    tryOpen();
  });
  on('lockcover', ({ on: up }) => {
    lockUp = up;
    tryOpen();
  });
  on('profile', tryOpen);
  on('presets', tryOpen);
  window.addEventListener('hashchange', takeLink);
  takeLink();
}
