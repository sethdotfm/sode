// The Link with key tab of the Share modal (panelshare.js shareTabs). A panel that acts as an API client has one key,
// shown once when it is made, so a link opened later says so and offers Rotate. Rotate, Revoke and a setup link
// that replaces a key each ask for a Confirm first (the churning letter and the short lockout).
import { el, icon, S } from './core.js';
import { btn, check, field } from './formbits.js';
import { churnKey } from './lockrules.js';
import { qrNode } from './keyview.js';
import { shareTabs } from './panelshare.js';
import { MODES, MODE_HINT, CONFIRM_TEXT, maskLink, makeAction, confirmKind, parseNetworks, networksText, networksChanged } from './panelkeyrules.js';
import { linkFor } from './panellink.js';

const LOCKOUT_FALLBACK_MS = 500; // the panels page has no hello, so S.cfg can be unset
const COPIED_MS = 2000;
const FAIL = Symbol('failed'); // a call that said why in the note

const dot = (kind) => {
  const d = el('span', `pb-dot pb-dot--${kind}`);
  d.setAttribute('aria-hidden', 'true');
  return d;
};

shareTabs.push(({ panel, api }) => {
  const root = el('div', 'sode-ov__keytab');
  const tab = { id: 'key', text: 'Link with key', panel: root };
  if (panel.acts !== 'api') {
    root.append(
      el(
        'p',
        'sode__hint',
        'This panel acts as the signed-in user, so it has no key. It asks for a PIN when it is launched. Make it act as an API client in the editor to give it a link with a key.',
      ),
    );
    return tab;
  }

  let mode = 'key';
  let link = ''; // the absolute link, held only while this modal is open
  let secret = ''; // what the server sent for it, so a screen name can change the link
  let screen = ''; // the optional ?instance= name
  let linkMode = 'key';
  let revealed = false;
  let confirming = null; // {kind, key, lock, ready}
  let busy = false;

  const note = el('p', 'sode__hint');
  note.setAttribute('role', 'status');
  const say = (text) => {
    note.textContent = text;
  };

  const status = el('p', 'sode__hint');
  const modes = el('div', 'pb-btn-group');
  modes.setAttribute('role', 'radiogroup');
  modes.setAttribute('aria-label', 'Kind of link');
  const modeHint = el('p', 'sode__hint');
  const actions = el('div', 'sode-ov__keyrow');
  const confirmBox = el('div', 'sode-ov__keyconfirm');
  confirmBox.hidden = true;
  confirmBox.tabIndex = -1;
  const linkBox = el('div', 'sode-ov__keylink');
  const screenIn = el('input', 'pb-input');
  screenIn.id = 'key-screen';
  screenIn.maxLength = 40;
  screenIn.autocomplete = 'off';
  screenIn.placeholder = 'Lobby tablet';
  screenIn.addEventListener('input', () => {
    screen = screenIn.value;
    link = buildLink();
    paintLink();
  });
  const screenBox = field(
    'key-screen',
    'Screen name (optional)',
    screenIn,
    'Skips the question that screen asks the first time it opens the link. It is not a secret.',
  );
  screenBox.hidden = true;

  function paintTop() {
    const hasKey = !!panel.hasKey;
    status.replaceChildren(
      dot(hasKey ? 'ok' : 'idle'),
      hasKey ? ' This panel has a key. Its link was shown once, when it was made. Rotate makes a new one.' : ' This panel has no key.',
    );
    modes.replaceChildren(
      ...MODES.flatMap(([m, text]) => {
        const r = el('input', 'pb-btn-check');
        r.type = 'radio';
        r.name = 'key-mode';
        r.id = `key-mode-${m}`;
        r.dataset.mode = m;
        r.checked = mode === m;
        const l = el('label', 'pb-btn pb-btn--outline', text);
        l.htmlFor = r.id;
        return [r, l];
      }),
    );
    modeHint.textContent = MODE_HINT[mode];
    const act = makeAction(mode, hasKey);
    const make = btn(hasKey && mode === 'key' ? 'pb-btn--outline' : 'pb-btn--ok', act.label, () =>
      act.confirm ? ask(confirmKind(mode, 'make'), makeLink) : makeLink(),
    );
    make.disabled = busy;
    actions.replaceChildren(make);
    if (hasKey) {
      const revoke = btn('pb-btn--outline', 'Revoke', () => ask('revoke', revokeKey));
      revoke.disabled = busy;
      actions.append(revoke);
    }
  }

  modes.addEventListener('change', (e) => {
    mode = e.target.dataset.mode;
    cancelAsk();
    say('');
    paintTop();
    modes.querySelector(`[data-mode="${mode}"]`).focus();
  });

  // ---- Confirm
  function cancelAsk() {
    confirming?.lock && clearTimeout(confirming.lock);
    confirming = null;
    confirmBox.hidden = true;
    confirmBox.replaceChildren();
  }
  function ask(kind, run) {
    cancelAsk();
    const key = churnKey();
    const keyEl = el('kbd', 'sode__key sode__key--locked', key.toUpperCase());
    const verb = kind === 'revoke' ? 'Revoke' : kind === 'setup' ? 'Replace the key' : 'Rotate';
    const ok = btn('pb-btn--ok', `${verb} (${key.toUpperCase()})`, () => go());
    ok.disabled = true;
    const cancel = btn('pb-btn--outline', 'Cancel', () => {
      cancelAsk();
      paintTop();
    });
    const c = { kind, key, ready: false, lock: 0 };
    c.lock = setTimeout(() => {
      c.ready = true;
      keyEl.classList.remove('sode__key--locked');
      ok.disabled = false;
    }, S.cfg?.confirmLockoutMs ?? LOCKOUT_FALLBACK_MS);
    function go() {
      if (!c.ready || confirming !== c) return;
      cancelAsk();
      run();
    }
    c.go = go;
    confirming = c;
    confirmBox.replaceChildren(el('p', null, CONFIRM_TEXT[kind]), el('div', 'sode__prompt', 'Press', keyEl, 'to confirm', ok, cancel));
    confirmBox.hidden = false;
    confirmBox.focus();
  }
  // The letter key answers the Confirm, as in the main page.
  root.addEventListener('keydown', (e) => {
    if (!confirming || e.ctrlKey || e.metaKey || e.altKey || e.key.toLowerCase() !== confirming.key) return;
    e.preventDefault();
    e.stopPropagation();
    confirming.go();
  });

  // ---- link
  async function call(path, init) {
    busy = true;
    paintTop();
    say('');
    try {
      return await api(path, init);
    } catch (e) {
      say(e.message);
      return FAIL;
    } finally {
      busy = false;
      paintTop();
      syncNets();
    }
  }
  const buildLink = (kiosk = false) => linkFor(panel, { [linkMode === 'setup' ? 'setup' : 'key']: secret, instance: screen, kiosk, base: document.baseURI });
  async function makeLink() {
    const res = await call(`api/v1/panels/${panel.id}/key`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ setup: mode === 'setup' }),
    });
    if (res === FAIL) return;
    secret = res.key || res.setup;
    linkMode = mode;
    link = buildLink();
    revealed = false;
    panel.hasKey = mode === 'key'; // a setup link is not a key until a browser trades it
    paintTop();
    paintLink();
    say(
      mode === 'setup'
        ? 'The old key is ended. Open the link once on the device to set it up.'
        : res.rotated
          ? 'Rotated. Open instances signed out and old links stopped working.'
          : 'Link made.',
    );
  }
  async function revokeKey() {
    if ((await call(`api/v1/panels/${panel.id}/key`, { method: 'DELETE' })) === FAIL) return;
    link = secret = '';
    panel.hasKey = false;
    paintTop();
    paintLink();
    say('Revoked. The panel has no key.');
  }

  function paintLink() {
    linkBox.replaceChildren();
    screenBox.hidden = !link;
    if (!link) return;
    const input = el('input', 'pb-input sode__secret-key');
    input.readOnly = true;
    input.spellcheck = false;
    input.value = revealed ? link : maskLink(link);
    input.setAttribute('aria-label', linkMode === 'setup' ? 'Setup link' : 'Link with key');
    input.addEventListener('focus', () => revealed && input.select());
    const reveal = check('Show link', {
      checked: revealed,
      onChange: (on) => {
        revealed = on;
        paintLink();
      },
    });
    const copy = el('button', 'pb-btn pb-btn--outline', icon('copy'), 'Copy link');
    copy.type = 'button';
    copy.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(link);
        say('Copied.');
        setTimeout(() => say(''), COPIED_MS);
      } catch {
        revealed = true;
        paintLink();
        linkBox.querySelector('input').select();
        say('Copy failed. The link is selected: copy it by hand.');
      }
    });
    const kiosk = btn('pb-btn--outline', 'Open as kiosk', () => window.open(buildLink(true), '_blank', 'noopener'));
    linkBox.append(el('div', 'sode__secret-row', input, reveal.label), el('div', 'sode-ov__keyrow', copy, kiosk));
    if (revealed) {
      const qr = qrNode(link);
      linkBox.append(qr || el('p', 'sode__hint', 'This link is too long for a QR code. Copy it instead.'));
    }
  }

  // ---- allowed networks
  const nets = el('textarea', 'pb-textarea');
  nets.id = 'key-networks';
  nets.rows = 3;
  nets.spellcheck = false;
  nets.placeholder = '192.168.1.0/24';
  nets.value = networksText(panel.clientNetworks);
  const netSave = btn('pb-btn--outline', 'Save networks', async () => {
    const res = await call(`api/v1/panels/${panel.id}/networks`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ networks: parseNetworks(nets.value) }),
    });
    if (res === FAIL) return;
    panel.clientNetworks = res.networks;
    nets.value = networksText(res.networks);
    syncNets();
    say(res.networks.length ? 'Saved. The key works only from these networks.' : 'Saved. The key works from any address.');
  });
  const syncNets = () => {
    netSave.disabled = busy || !networksChanged(panel.clientNetworks, nets.value);
  };
  nets.addEventListener('input', syncNets);
  const netBox = el(
    'div',
    'sode-ov__keynets',
    Object.assign(el('label', null, 'Allowed networks'), { htmlFor: 'key-networks' }),
    nets,
    el(
      'p',
      'sode__hint',
      "One address or CIDR such as 192.168.1.0/24 per line. A request with this panel's key from anywhere else is refused. Leave it empty to allow any address.",
    ),
    netSave,
  );

  root.append(status, modes, modeHint, actions, confirmBox, screenBox, linkBox, netBox, note);
  paintTop();
  syncNets();
  return tab;
});
