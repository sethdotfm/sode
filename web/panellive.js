// Keeps an open panel current: polls the panel's rev, shows the banner while an update waits and calls
// apply(panel) when panelupdate.js says so. Polled because a browser cannot send Bearer on an event stream.
import { $ } from './core.js';
import { createUpdate, noticeRev, snooze, updateStep, applied, bannerLine } from './panelupdate.js';

const CHECK_MS = 5000;

// get(path) reads JSON with the panel's key; state() is { staged, busy }.
export function watchPanel({ id, rev, get, state, apply }) {
  let u = createUpdate(rev);
  let checking = false;

  const show = (text) => {
    $('panel-update').hidden = !text;
    $('panel-update-text').textContent = text;
  };

  let latest = null; // the changed copy fetched, applied when the rules allow

  function tick() {
    if (!latest) return;
    const s = updateStep(u, state(), Date.now());
    if (s.step === 'apply') {
      const fresh = latest;
      latest = null;
      u = applied(u, fresh.rev);
      show('');
      apply(fresh);
    } else show(s.step === 'banner' ? bannerLine(s) : '');
  }

  async function check() {
    if (checking) return;
    checking = true;
    try {
      const p = await get(`api/v1/panels/${id}`);
      u = noticeRev(u, p.rev, Date.now());
      latest = u.since ? p : null;
    } catch {
      // A failed read leaves the panel as it is; the next check tries again.
    }
    checking = false;
    tick();
  }

  $('panel-snooze').addEventListener('click', () => {
    u = snooze(u, Date.now());
    tick();
  });
  setInterval(check, CHECK_MS);
  setInterval(tick, 1000);
}
