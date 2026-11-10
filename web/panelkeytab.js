// The Link with key tab of the Share modal (panelshare.js shareTabs). A panel that acts as an API client holds one key
// per named instance. The tab lists them, rotates or revokes one at a time, and makes a link for a name. A link is
// shown once, when it is made. Rotate, Revoke and any make that replaces a key each ask for a Confirm first (the
// churning letter and the short lockout). Server side: internal/app/panelkey.go.
import { el, icon, on } from './core.js';
import { btn, check, field } from './formbits.js';
import { confirmBox } from './confirmbox.js';
import { qrNode, secretNode } from './keyview.js';
import { trapDialog } from './dialogkeys.js';
import { shareTabs } from './panelshare.js';
import {
  MODES,
  MODE_HINT,
  CONFIRM_TEXT,
  SESSION_WARNING,
  SESSION_NEXT,
  maskLink,
  makeAction,
  confirmKind,
  instanceCheck,
  keyRow,
  parseNetworks,
  networksText,
  networksChanged,
} from './panelkeyrules.js';
import { linkFor } from './panellink.js';
import { linkBase, shareHostField } from './sharehost.js';
import { linkRow } from './panellinkrow.js';
import { hasClient } from './panelsrules.js';

const COPIED_MS = 2000;
const REFRESH_MS = 5000; // how often the list asks again, so Online follows the instances
const FAIL = Symbol('failed'); // a call that said why in the note

// The open key dialog's close. The Lock closes it, as it does the Share modal, so the key is not left on screen.
let shutKey = null;
on('locking', () => shutKey?.());

const dot = (kind) => {
  const d = el('span', `pb-dot pb-dot--${kind}`);
  d.setAttribute('aria-hidden', 'true');
  return d;
};

shareTabs.push(({ panel, api }) => {
  const root = el('div', 'sode-ov__keytab');
  const tab = { id: 'key', text: 'Link with key', panel: root };
  if (!hasClient(panel)) {
    // Greyed, still readable: a Session panel has no key to make, rotate or revoke.
    tab.muted = true;
    root.append(el('p', 'sode__hint', SESSION_WARNING), linkRow(panel), el('p', 'sode__hint', SESSION_NEXT));
    return tab;
  }

  let mode = 'key';
  let kioskLink = false; // the link shown, its copy and its QR code all follow this switch
  let keys = []; // the panel's instances as the server lists them
  let max = 20;
  let made = null; // {name, secret, mode, link}: held only while this modal is open
  let revealed = false;
  let busy = false;

  const note = el('p', 'sode__hint');
  note.setAttribute('role', 'status');
  const say = (text) => {
    note.textContent = text;
  };

  const status = el('p', 'sode__hint');
  const list = el('ul', 'sode-ov__keys');
  const modes = el('div', 'pb-btn-group');
  modes.setAttribute('role', 'radiogroup');
  modes.setAttribute('aria-label', 'Kind of link');
  const modeHint = el('p', 'sode__hint');
  const actions = el('div', 'sode-ov__keyrow');
  const guard = confirmBox(root);
  const linkBox = el('div', 'sode-ov__keylink');
  const nameIn = el('input', 'pb-input');
  nameIn.id = 'key-instance';
  nameIn.autocomplete = 'off';
  nameIn.placeholder = 'Lobby tablet';
  nameIn.addEventListener('input', () => {
    guard.cancel();
    paintTop();
  });
  const nameBox = field(
    'key-instance',
    'Instance name',
    nameIn,
    'Required. It names this instance in the list and is its device name, so the page does not ask for one. It is not a secret.',
  );
  const check1 = () => instanceCheck(nameIn.value, keys, max);

  function paintList() {
    const now = Date.now();
    list.replaceChildren(
      ...keys.map((k) => {
        const row = keyRow(k, now);
        const rotate = btn('pb-btn--outline pb-btn--sm', 'Rotate', () => {
          nameIn.value = k.name;
          mode = 'key';
          paintTop();
          ask('rotate', () => makeLink(k.name, false));
        });
        rotate.hidden = !!k.setup;
        rotate.disabled = busy;
        const revoke = btn('pb-btn--outline pb-btn--sm', 'Revoke', () => ask('revoke', () => revokeKey(k.name)));
        revoke.disabled = busy;
        rotate.setAttribute('aria-label', `Rotate ${k.name}`);
        revoke.setAttribute('aria-label', `Revoke ${k.name}`);
        return el(
          'li',
          'sode-ov__keyitem',
          el('span', 'sode-ov__keyname', el('strong', null, k.name), el('span', 'sode-ov__muted', dot(row.state.kind), ` ${row.state.text}`)),
          el('span', 'sode-ov__muted', `${row.created}. ${row.used}.`),
          el('span', 'sode-ov__keyrow', rotate, revoke),
        );
      }),
    );
    if (!keys.length) list.append(el('li', 'sode-ov__muted', 'No instances yet. Name one below to make its link.'));
  }

  function paintTop() {
    status.replaceChildren(
      dot(keys.length ? 'ok' : 'idle'),
      keys.length
        ? ` ${keys.length} of ${max} instances. A link shows once, when it is made. Rotate makes a new one for that instance.`
        : ' This panel has no instances.',
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
    const c = check1();
    const act = makeAction(mode, c.held);
    const make = btn(c.held && mode === 'key' ? 'pb-btn--outline' : 'pb-btn--ok', act.label, () =>
      act.confirm ? ask(confirmKind(mode, 'make'), () => makeLink(c.name, mode === 'setup')) : makeLink(c.name, mode === 'setup'),
    );
    make.disabled = busy || !!c.error;
    actions.replaceChildren(make);
    if (c.error && nameIn.value) actions.append(el('span', 'sode__hint', c.error));
    panel.hasKey = keys.some((k) => !k.setup);
  }

  modes.addEventListener('change', (e) => {
    mode = e.target.dataset.mode;
    guard.cancel();
    say('');
    paintTop();
    modes.querySelector(`[data-mode="${mode}"]`).focus();
  });

  // ---- Confirm
  function ask(kind, run) {
    const verb = kind === 'revoke' ? 'Revoke' : kind === 'setup' ? 'Replace the key' : 'Rotate';
    guard.ask(CONFIRM_TEXT[kind], verb).then((ok) => (ok ? run() : paintTop()));
  }

  // ---- calls
  async function call(path, init) {
    busy = true;
    paintTop();
    paintList();
    say('');
    try {
      return await api(path, init);
    } catch (e) {
      say(e.message);
      return FAIL;
    } finally {
      busy = false;
      paintTop();
      paintList();
      syncNets();
    }
  }
  async function loadKeys() {
    try {
      const res = await api(`api/v1/panels/${panel.id}/keys`);
      keys = res.keys;
      max = res.max;
    } catch (e) {
      say(e.message);
    }
    paintTop();
    paintList();
  }
  const timer = setInterval(() => (root.isConnected ? !busy && loadKeys() : clearInterval(timer)), REFRESH_MS);

  const buildLink = (kiosk = false) => linkFor(panel, { [made.mode === 'setup' ? 'setup' : 'key']: made.secret, instance: made.name, kiosk, base: linkBase() });
  async function makeLink(name, setup) {
    const res = await call(`api/v1/panels/${panel.id}/key`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ instance: name, setup }),
    });
    if (res === FAIL) return;
    made = { name: res.instance, secret: res.key || res.setup, mode: setup ? 'setup' : 'key' };
    made.link = buildLink(kioskLink);
    revealed = false;
    await loadKeys();
    paintLink();
    if (!setup) showKey(made.name, res.key);
    say(
      setup
        ? `${made.name}: the old key is ended. Open the link once on that device to set it up.`
        : res.rotated
          ? `${made.name} rotated. That instance signed out and its old link stopped working.`
          : `Link made for ${made.name}.`,
    );
  }
  async function revokeKey(name) {
    if ((await call(`api/v1/panels/${panel.id}/key?instance=${encodeURIComponent(name)}`, { method: 'DELETE' })) === FAIL) return;
    if (made?.name.toLowerCase() === name.toLowerCase()) made = null;
    await loadKeys();
    paintLink();
    say(`Revoked ${name}. Other instances are not affected.`);
  }

  // The key itself, for a client that takes it apart from the link (a Bearer header). Built per make and removed on
  // close, so the secret lives no longer than the modal; trapDialog drops it from the Esc stack once hidden.
  function showKey(name, secret) {
    const back = document.activeElement;
    const keyRoot = el('div', 'pb-modal-backdrop sode__scrim');
    const dialog = el('div', 'pb-modal sode-ov__keyshow');
    dialog.setAttribute('role', 'dialog');
    dialog.setAttribute('aria-modal', 'true');
    dialog.setAttribute('aria-labelledby', 'key-show-title');
    dialog.tabIndex = -1;
    const done = btn('pb-btn--ok', 'Done', () => shut());
    dialog.append(
      el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', `API key for ${name}`), { id: 'key-show-title' })),
      el(
        'div',
        'pb-modal__body',
        el('p', 'sode__hint', 'The link already holds this key. Use the key on its own where an app asks for one, in an Authorization: Bearer header.'),
        secretNode(secret),
      ),
      el('div', 'pb-modal__footer', done),
    );
    keyRoot.append(dialog);
    document.body.append(keyRoot);
    const dlg = trapDialog(keyRoot, dialog, () => shut());
    function shut() {
      shutKey = null;
      dlg.hide();
      keyRoot.remove();
      if (back?.isConnected) back.focus({ preventScroll: true });
    }
    shutKey = shut;
    dlg.show();
    done.focus();
  }

  function paintLink() {
    linkBox.replaceChildren();
    if (!made) return;
    const input = el('input', 'pb-input sode__secret-key');
    input.readOnly = true;
    input.spellcheck = false;
    input.value = revealed ? made.link : maskLink(made.link);
    input.setAttribute('aria-label', made.mode === 'setup' ? `Setup link for ${made.name}` : `Link for ${made.name}`);
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
        await navigator.clipboard.writeText(made.link);
        say('Copied.');
        setTimeout(() => say(''), COPIED_MS);
      } catch {
        revealed = true;
        paintLink();
        linkBox.querySelector('.sode__secret-key').select();
        say('Copy failed. The link is selected: copy it by hand.');
      }
    });
    // Kiosk adds `kiosk=1` after the key and keeps `?instance=`.
    const view = el('div', 'pb-btn-group');
    view.setAttribute('role', 'radiogroup');
    view.setAttribute('aria-label', 'How the link opens');
    view.replaceChildren(
      ...[
        ['regular', 'Regular'],
        ['kiosk', 'Kiosk'],
      ].flatMap(([v, text]) => {
        const r = el('input', 'pb-btn-check');
        r.type = 'radio';
        r.name = 'key-view';
        r.id = `key-view-${v}`;
        r.dataset.view = v;
        r.checked = (v === 'kiosk') === kioskLink;
        const l = el('label', 'pb-btn pb-btn--outline', text);
        l.htmlFor = r.id;
        return [r, l];
      }),
    );
    view.addEventListener('change', (e) => {
      kioskLink = e.target.dataset.view === 'kiosk';
      made.link = buildLink(kioskLink);
      paintLink();
      linkBox.querySelector(`[data-view="${kioskLink ? 'kiosk' : 'regular'}"]`).focus();
    });
    const host = shareHostField(() => {
      made.link = buildLink(kioskLink);
      paintLink();
    });
    linkBox.append(
      el('p', 'sode__hint', `Link for ${made.name}. It is shown only now. Closing this window loses it.`),
      el('div', 'sode__secret-row', input, reveal.label),
      view,
      el('div', 'sode-ov__keyrow', copy),
      ...(host ? [host] : []),
    );
    if (revealed) {
      const qr = qrNode(made.link);
      linkBox.append(qr || el('p', 'sode__hint', 'This link is too long for a QR code. Copy it instead.'));
    }
  }

  // ---- allowed networks
  const nets = el('textarea', 'pb-textarea');
  nets.id = 'key-networks';
  nets.rows = 2;
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
    say(res.networks.length ? 'Saved. Every key works only from these networks.' : 'Saved. Keys work from any address.');
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
      "One address or CIDR such as 192.168.1.0/24 per line. A request with any of this panel's keys from anywhere else is refused. Leave it empty to allow any address.",
    ),
    netSave,
  );

  root.append(status, list, el('h3', 'sode-side__h', 'New link'), nameBox, modes, modeHint, actions, guard.node, linkBox, netBox, note);
  paintTop();
  paintList();
  syncNets();
  loadKeys();
  return tab;
});
