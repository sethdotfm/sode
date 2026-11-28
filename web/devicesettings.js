// The This browser selects and the Everyone default selects offer the same choices (prefs.js). A This browser select
// applies immediately; a default saves with the form.

import { $, el, on, emit, S } from './core.js';
import { listedUser } from './namepickrules.js';
import { ownPref, globalPref, defaultPref, myPref, setPref, pref } from './device.js';
import { myDefaultButtons } from './mydefaults.js';
import { profile, profileName, saveProfile } from './profile.js';
import { PREFS } from './prefs.js';
import { signInOffered } from './lockrules.js';
import { initStepIn } from './stepindialog.js';

// reindex: rebuilds the search index when a my-default button appears (made on the first profile).
export function initDeviceSettings({ form, reindex }) {
  const prefOptions = (name, globalText) => {
    const opt = (value, text) => Object.assign(el('option', null, text), { value });
    return [...(globalText ? [opt('', globalText)] : []), ...Object.entries(PREFS[name].values).map(([v, text]) => opt(v, text))];
  };

  for (const s of form.querySelectorAll('select[data-gpref]')) s.replaceChildren(...prefOptions(s.dataset.gpref));

  const originText = (name, v) => {
    const text = PREFS[name].values[v];
    if (myPref(name) === v) return `${text} (my default)`;
    if (globalPref(name) === v) return `${text} (everyone's default)`;
    return text;
  };

  const myButtons = new Map();
  function myButton(select) {
    const name = select.dataset.pref;
    let mb = myButtons.get(name);
    if (mb) return mb;
    // The search name holds the setting, since each preference has the same two buttons (the Palette, Settings search).
    const what = form.querySelector(`label[for="${select.id}"]`)?.textContent.trim() || name;
    // Outline, so the column reads as buttons and not as labels. The tip holds the long words.
    const mk = (id, label, tip, findLabel) => {
      const b = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode__my-default', label);
      b.type = 'button';
      b.dataset.pbTip = tip;
      // The id lets a Palette row press it.
      b.id = id;
      b.dataset.find = id;
      b.dataset.label = findLabel;
      return b;
    };
    const save = mk(`my-default-${name}`, 'Save for me', 'Save as my default in every browser', `Save ${what} as my default`);
    const clear = mk(`my-default-clear-${name}`, 'Clear mine', 'Clear my default in every browser', `Clear my default for ${what}`);
    const note = el('span', 'sode__hint');
    note.hidden = true;
    note.setAttribute('role', 'status');
    // btn: the button. Its done event says what changed, for the result banner of a Palette row (palettehome.js).
    const run = (change, after, btn) => async () => {
      const p = profile();
      if (!p.name) return;
      const prefs = { ...p.prefs };
      change(prefs);
      const text = btn === save ? `My default for ${what} is ${PREFS[name].values[prefs[name]]} now.` : `${what} uses Everyone's default now.`;
      save.disabled = clear.disabled = true;
      const r = await saveProfile({ prefs });
      save.disabled = clear.disabled = false;
      note.hidden = r.ok;
      note.textContent = r.ok ? '' : r.err || 'Could not save.';
      if (!r.ok) return;
      after();
      emit('myDefaultDone', { id: btn.id, field: select.id, text });
    };
    save.addEventListener(
      'click',
      run(
        (prefs) => {
          prefs[name] = pref(name);
        },
        () => setPref(name, ''),
        save,
      ),
    );
    clear.addEventListener(
      'click',
      run(
        (prefs) => {
          delete prefs[name];
        },
        () => renderPrefs(),
        clear,
      ),
    );
    const row = el('div', 'sode__my-defaults');
    row.append(save, clear, note);
    select.closest('.sode__field').append(row);
    mb = { save, clear, note, row };
    myButtons.set(name, mb);
    reindex();
    return mb;
  }

  // Runs on any change of the settings in force, so "Everyone's default (value)" stays true under an open form, and
  // Alt+Shift+T shows.
  function renderPrefs() {
    const user = profileName();
    for (const s of form.querySelectorAll('select[data-pref]')) {
      const name = s.dataset.pref;
      const opt = (value, text) => Object.assign(el('option', null, text), { value });
      s.replaceChildren(
        opt('', `${myPref(name) ? 'My' : "Everyone's"} default (${PREFS[name].values[defaultPref(name)]})`),
        ...Object.keys(PREFS[name].values).map((v) => opt(v, originText(name, v))),
      );
      s.value = ownPref(name);
      const mb = myButtons.get(name) || (user ? myButton(s) : null);
      if (!mb) continue;
      const show = myDefaultButtons(pref(name), myPref(name));
      mb.save.hidden = !user || !show.save;
      mb.clear.hidden = !user || !show.clear;
      mb.row.hidden = !user;
      if (!user) mb.note.hidden = true;
    }
  }
  renderPrefs();
  on('prefs', renderPrefs);
  on('profile', renderPrefs);

  let acctUser = '';
  let lastAccess = null;
  let noUsers = false;
  function paintAccount(L) {
    acctUser = L.user || '';
    noUsers = !L.user && !S.users?.length;
    const acct = !!L.user || signInOffered(L) || noUsers;
    $('account-name').hidden = !acct;
    const who = noUsers ? ['No users yet. Add one in Access > Users.'] : ['Not signed in'];
    $('account-name').replaceChildren(...(L.user ? ['Signed in as ', el('strong', null, L.user)] : who));
    $('account-guest-note').hidden = !acct || !!L.user || noUsers;
    for (const id of ['account-profile', 'account-copy', 'account-transfer', 'account-pin', 'account-keys', 'account-clients', 'account-logout'])
      $(id).hidden = !L.user;
    // A user without a PIN holds no personal API keys or own API clients.
    const noPin = listedUser(S.users, L.user)?.len === 0;
    for (const id of ['account-keys', 'account-clients']) if (noPin) $(id).hidden = true;
    $('account-login').hidden = !!L.user || !acct || noUsers;
    // With no users a Lock has nobody to end it but the page itself.
    $('lock-btn').hidden = (L.enforced && !L.user) || noUsers;
  }
  on('access', (L) => {
    lastAccess = L;
    paintAccount(L);
  });
  // The user list comes with the hello, which can come after the access state.
  on('hello', () => {
    if (lastAccess && noUsers !== (!lastAccess.user && !S.users?.length)) paintAccount(lastAccess);
  });
  $('lock-btn').addEventListener('click', () => emit('localLock'));
  $('account-pin').addEventListener('click', () => emit('changePin'));
  $('account-keys').addEventListener('click', () => emit('openKeys'));
  $('account-clients').addEventListener('click', () => emit('openClients'));
  $('account-profile').addEventListener('click', () => emit('openProfile', { name: acctUser }));
  $('account-copy').addEventListener('click', () => emit('openBrowserCopy'));
  $('account-transfer').addEventListener('click', () => emit('openOwnExport'));
  $('account-logout').addEventListener('click', () => emit('logout'));
  $('account-login').addEventListener('click', () => emit('login'));
  initStepIn(); // after the access handler above, which it overrides while stepped in
  for (const s of form.querySelectorAll('select[data-pref]')) s.addEventListener('change', () => setPref(s.dataset.pref, s.value));
  return { renderPrefs };
}
