// The editor's Test it live: the saved panel runs in place of the grid and sidebar (panelrun.js, embedded), over
// the editor's own session. It is not an instance of the panel, so it sends no keepalive.
import { sessionFetch } from './sessionkey.js';
import { runPanel } from './panelrun.js';
import { liveUi } from './panelmount.js';

// host holds the live panel, body is the editor it replaces, button is the toolbar's Test it live.
export function createLive(host, body, button) {
  const label = button.textContent;
  let run = null;
  const api = {
    on: () => !!run,
    start(id) {
      api.stop();
      const ui = liveUi(host, () => api.stop());
      host.hidden = false;
      body.hidden = true;
      button.textContent = 'Back to editing';
      run = runPanel(id, { ui, call: sessionFetch });
      ui.back.focus();
    },
    stop() {
      if (!run) return;
      run.stop();
      run = null;
      host.replaceChildren();
      host.hidden = true;
      body.hidden = false;
      button.textContent = label;
    },
  };
  return api;
}
