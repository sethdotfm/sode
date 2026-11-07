// This browser section of Settings and the Everyone defaults selects: the same choices (prefs.js). A browser
// one starts at "Everyone's default (value)" ("My" with their own), which follows the default, and
// applies at once. A default one is saved with the form. Also the signed-in user block.

import { $, el, on, emit } from './core.js';
import { ownPref, globalPref, defaultPref, myPref, setPref, pref } from './device.js';
import { myDefaultButtons } from './mydefaults.js';
import { profile, profileName, saveProfile } from './profile.js';
import { PREFS } from './prefs.js';
import { signInOffered } from './lockrules.js';

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
    const mk = (id, label) => {
      const b = el('button', 'pb-btn pb-btn--ghost pb-btn--sm sode__my-default', label);
      b.type = 'button';
      b.dataset.find = id;
      b.dataset.label = label;
      return b;
    };
    const save = mk(`my-default-${name}`, 'Save as my default');
    const clear = mk(`my-default-clear-${name}`, 'Clear my default');
    const note = el('span', 'sode__hint');
    note.hidden = true;
    note.setAttribute('role', 'status');
    const run = (change, after) => async () => {
      const p = profile();
      if (!p.name) return;
      const prefs = { ...p.prefs };
      change(prefs);
      save.disabled = clear.disabled = true;
      const r = await saveProfile({ prefs });
      save.disabled = clear.disabled = false;
      note.hidden = r.ok;
      note.textContent = r.ok ? '' : r.err || 'Could not save.';
      if (r.ok) after();
    };
    save.addEventListener(
      'click',
      run(
        (prefs) => {
          prefs[name] = pref(name);
        },
        () => setPref(name, ''),
      ),
    );
    clear.addEventListener(
      'click',
      run(
        (prefs) => {
          delete prefs[name];
        },
        () => renderPrefs(),
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

  // On any change of settings in force, so "Global (value)" stays true under an open form, and
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
  on('access', (L) => {
    acctUser = L.user || '';
    const acct = !!L.user || signInOffered(L);
    $('account-name').hidden = !acct;
    $('account-name').replaceChildren(...(L.user ? ['Signed in as ', el('strong', null, L.user)] : ['Not signed in']));
    for (const id of ['account-profile', 'account-transfer', 'account-pin', 'account-logout']) $(id).hidden = !L.user;
    $('account-login').hidden = !!L.user || !acct;
    $('lock-btn').hidden = L.enforced && !L.user;
  });
  $('lock-btn').addEventListener('click', () => emit('localLock'));
  $('account-pin').addEventListener('click', () => emit('changePin'));
  $('account-profile').addEventListener('click', () => emit('openProfile', { name: acctUser }));
  $('account-transfer').addEventListener('click', () => emit('openTransfer', { scope: 'user', user: acctUser }));
  $('account-logout').addEventListener('click', () => emit('logout'));
  $('account-login').addEventListener('click', () => emit('login'));
  for (const s of form.querySelectorAll('select[data-pref]')) s.addEventListener('change', () => setPref(s.dataset.pref, s.value));
  return { renderPrefs };
}
