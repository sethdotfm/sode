// The server plans every step (POST api/import, docs/agents/profiles.md); the page sends ticks and choices.

import { el, off, S } from './core.js';
import { call } from './exportcalls.js';
import { recheckPin } from './recheck.js';
import { check as checkBox, segmented } from './formbits.js';
import { CATEGORIES, OWN_CATEGORIES, ADMIN_ONLY, sizeText } from './exportrules.js';
import * as R from './importrules.js';
import { stillBroken } from './remaprules.js';
import { remapState, remapReq, remapView, remapReview, rowOf } from './importremap.js';
import { applyBrowser } from './browserapply.js';
import { fmtTime } from './device.js';

const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};

const MODES = [
  ['add', 'Add import'],
  ['replace', 'Replace import'],
];

// opts: setNote(text, error), setReview(title or ''), backup() downloads Back up everything. own: My user's own items
// (api/own/import), with the file's other categories shown as Administrators only.
export function importTab({ setNote, setReview, backup, own = false }) {
  const route = own ? 'api/own/import' : 'api/import';
  const cats = own ? OWN_CATEGORIES : CATEGORIES;
  const ids = own ? 'own-xi' : 'xi'; // both dialogs are in the page
  const panel = el('div', 'sode__xi-import');
  const fileInput = el('input');
  fileInput.type = 'file';
  fileInput.accept = '.zip,application/zip';
  fileInput.id = `${ids}-file`;
  fileInput.hidden = true;
  const chooseBtn = btn('pb-btn--ok', 'Choose a file…', () => fileInput.click());
  chooseBtn.id = `${ids}-choose`;
  const brokenBadge = el('span', 'pb-badge pb-badge--error');
  const reviewBtn = btn('pb-btn--ok', 'Review…', () => review());
  reviewBtn.id = `${ids}-review`;
  const backBtn = btn('pb-btn--outline', 'Back', () => back());
  backBtn.id = `${ids}-back`;
  const importBtn = btn('pb-btn--ok', 'Import', () => apply());
  importBtn.id = `${ids}-import`;
  const anotherBtn = btn('pb-btn--outline', 'Import another', () => reset());
  const acts = el('span', 'sode__xi-acts', brokenBadge, chooseBtn, reviewBtn, backBtn, importBtn, anotherBtn);

  let file = null;
  let check = null;
  let pick = null;
  let remap = null; // importremap.js state
  let planned = null; // the review's plan
  let step = 'file'; // file, list, review, done
  let seq = 0;
  let failed = 0; // panels the apply did not write
  let reviewing = false;
  const expanded = new Set();

  const showActs = () => {
    chooseBtn.hidden = step !== 'file';
    reviewBtn.hidden = step !== 'list';
    backBtn.hidden = importBtn.hidden = step !== 'review';
    anotherBtn.hidden = step !== 'done';
    const broken = step === 'list' ? R.brokenItems(check.plan).filter((it) => pick.items[it.cat]?.has(it.key) && brk(it).length).length : 0;
    brokenBadge.hidden = !broken;
    brokenBadge.textContent = `${broken} Broken`;
    reviewBtn.disabled = step === 'list' && !R.importReq(pick).categories.length;
    importBtn.disabled = step === 'review' && !!planned?.refused;
    importBtn.textContent = own ? 'Import' : pick?.mode === 'replace' ? 'Replace and import' : 'Import (add)';
  };

  // In the list the plan is the check's. Later remap choices remove references from it.
  const brk = (it) => stillBroken(it.broken, remap.rowsBy, remap.targets, remap.choices);
  const fullReq = () => ({ ...R.importReq(pick), ...remapReq(remap) });

  function form(req) {
    const fd = new FormData();
    fd.append('plan', JSON.stringify(req)); // the server reads the plan first
    fd.append('file', file);
    return fd;
  }

  // again: a router target changed, so the check plans again with the ticks and choices kept.
  async function load(f, again = false) {
    file = f;
    const my = ++seq;
    setNote('Reading…');
    try {
      const routers = again ? { routers: { ...remap.targets } } : {};
      const res = await call(route, { method: 'POST', body: form({ mode: 'add', categories: R.PLANNED, ...routers }) });
      const got = await res.json();
      if (my !== seq) return;
      check = got;
      remap = remapState(check, again ? remap : null);
      if (!again) {
        pick = R.newPick(check);
        expanded.clear();
        for (const cat of R.PLANNED) if (check.plan.categories[cat]?.items.length) expanded.add(cat);
      }
      step = 'list';
      setNote('');
      render();
    } catch (e) {
      if (my === seq) setNote(e.message, true);
    }
  }
  fileInput.addEventListener('change', () => {
    if (fileInput.files[0]) load(fileInput.files[0]);
    fileInput.value = '';
  });

  function box(key, text, checked, mixed, disabled, onChange) {
    const b = el('input', 'pb-check__input');
    b.type = 'checkbox';
    b.dataset.key = key;
    b.checked = checked;
    b.indeterminate = mixed;
    b.disabled = disabled;
    b.addEventListener('change', () => onChange(b.checked));
    return el('label', 'pb-check', b, el('span', 'pb-check__label', text));
  }

  function choices(cat, it, i) {
    const seg = segmented(`${ids}-choice-${cat}-${i}`, `${it.name} clash`, R.choicesFor(cat, it), (v) => {
      pick.choices[cat][it.key] = v;
      render();
    });
    seg.set(pick.choices[cat][it.key]);
    for (const r of seg.node.querySelectorAll('input')) r.dataset.key = `choice:${cat}:${it.key}:${r.value}`;
    seg.node.classList.add('sode__xi-choice');
    return el('div', 'sode__xi-sub', seg.node);
  }

  const missing = (it, list = it.broken) => list.map((m) => R.missingText(m, off())).join(', ');

  function itemRow(cat, c, it, i) {
    const ticked = pick.items[cat].has(it.key);
    const hint = it.refused ? it.refused : it.same ? 'No change' : it.note || '';
    const broken = brk(it);
    const row = el(
      'div',
      'sode__xi-item',
      box(`item:${cat}:${it.key}`, it.name, ticked, false, !!c.refused || !!it.refused, (on) => {
        if (on) pick.items[cat].add(it.key);
        else pick.items[cat].delete(it.key);
        render();
      }),
      el('span', 'sode__hint', ...(broken.length ? [el('span', 'pb-badge pb-badge--error', 'Broken'), ' '] : []), hint),
      el('span', 'sode__xi-size', sizeText(it.size || 0)),
    );
    row.dataset.item = it.key;
    if (ticked && R.hasChoice(c, it, pick)) row.append(choices(cat, it, i));
    if (broken.length) row.append(el('span', 'sode__hint sode__xi-sub sode__error-text', `Missing ${missing(it, broken)}`));
    return row;
  }

  function catRows(cat, text) {
    const c = check.plan.categories[cat];
    const items = c?.items || [];
    const state = R.catState(check, pick, cat);
    const head = el('div', 'sode__xi-head');
    head.dataset.cat = cat;
    const caret = btn('pb-btn--ghost pb-btn--icon pb-btn--sm sode__xi-caret', expanded.has(cat) ? '▾' : '▸', () => {
      if (expanded.has(cat)) expanded.delete(cat);
      else expanded.add(cat);
      render();
    });
    caret.dataset.key = `caret:${cat}`;
    caret.setAttribute('aria-expanded', String(expanded.has(cat)));
    caret.setAttribute('aria-label', `Show the ${text.toLowerCase()} items`);
    caret.hidden = !items.length;
    const count = items.length > 1 ? ` ${pick.items[cat].size}/${items.length}` : '';
    // Replace import can empty a category the file holds with no items.
    const why = c?.refused || (!items.length && pick.mode === 'replace' ? 'Empties it here' : '');
    head.append(
      caret,
      box(`cat:${cat}`, text + count, state !== 'none', state === 'some', !!c?.refused || (!items.length && pick.mode !== 'replace'), (on) => {
        R.setCat(check, pick, cat, on);
        render();
      }),
      el('span', `sode__hint${c?.refused ? ' sode__error-text' : ''}`, why),
      el('span', 'sode__xi-size', sizeText(R.catSize(check, pick, cat))),
    );
    if (!expanded.has(cat) || caret.hidden) return [head];
    return [head, el('div', 'sode__xi-items', ...items.map((it, i) => itemRow(cat, c, it, i)))];
  }

  function routerNames() {
    // An own import plans no Routers category and no remap, so the check names the live router.
    if (own) {
      const lines = (check.routers || []).map((r) => {
        const name = r.Title || r.Prefix;
        return el('li', null, r.UID === check.router ? `${name}: this router` : `${name}: another router. Items that use it arrive Broken.`);
      });
      return lines.length ? [el('h3', 'sode__label sode__xi-h', 'Router names'), el('ul', 'sode__xi-list', ...lines)] : [];
    }
    return remapView(remap, { off, fmt: fmtTime, onChange: () => render(), onRouter: () => load(file, true) });
  }

  function adminRow(cat, text) {
    const head = el(
      'div',
      'sode__xi-head',
      el('span'),
      box(`admin:${cat}`, text, false, false, true, () => {}),
    );
    head.append(el('span', 'sode__hint', 'Administrators only'), el('span', 'sode__xi-size'));
    head.dataset.cat = `admin:${cat}`;
    return head;
  }

  function fileHint() {
    const where = `Sode ${check.sode}, ${check.sameInstall ? 'this install' : 'another install'}`;
    if (!own) return where;
    return check.from ? `${where}, from ${check.from}` : `${where}. No profile for you.`;
  }

  // Add import or Replace import. Replace import is for Administrators, and the dialog is only theirs.
  function modeRow() {
    if (own) return el('div', 'sode__xi-mode', el('strong', null, 'Add import'));
    const replace = pick.mode === 'replace';
    const seg = segmented('xi-mode', 'Import mode', MODES, (v) => {
      pick.mode = v;
      render();
    });
    seg.set(pick.mode);
    for (const r of seg.node.querySelectorAll('input')) r.dataset.key = `mode:${r.value}`;
    const row = el('div', 'sode__xi-mode', seg.node);
    if (replace) {
      const keep = checkBox('Keep current Administrators', {
        id: 'xi-keep-admins',
        checked: pick.keepAdmins,
        onChange: (on) => {
          pick.keepAdmins = on;
        },
      });
      keep.input.dataset.key = 'keepAdmins';
      row.append(el('div', 'sode__xi-sub', keep.label));
    }
    return row;
  }

  function listView() {
    const rows = [];
    if (own) {
      for (const [cat, text] of OWN_CATEGORIES) if (check.plan.categories[cat]) rows.push(...catRows(cat, text));
      for (const [cat, text] of ADMIN_ONLY) if (check.admin?.includes(cat)) rows.push(adminRow(cat, text));
    } else for (const [cat, text] of CATEGORIES) if (check.categories.includes(cat)) rows.push(...catRows(cat, text));
    return [
      el(
        'div',
        'sode__xi-file',
        el('strong', null, file.name),
        el('span', 'sode__xi-size', sizeText(file.size)),
        btn('pb-btn--ghost pb-btn--sm', 'Change', () => fileInput.click()),
        el('span', 'sode__hint sode__xi-sub', fileHint()),
      ),
      modeRow(),
      el('div', 'sode__xi-cats', ...rows),
      ...routerNames(),
    ];
  }

  async function review() {
    const my = ++seq;
    setNote('Checking…');
    reviewBtn.disabled = true;
    try {
      const res = await call(route, { method: 'POST', body: form(fullReq()) });
      const got = await res.json();
      if (my !== seq) return;
      planned = got.plan;
      step = 'review';
      setNote('');
      render();
    } catch (e) {
      if (my === seq) setNote(e.message, true);
    } finally {
      showActs();
    }
  }

  const pinInput = el('input', 'pb-input sode__xi-pin');
  pinInput.type = 'password';
  pinInput.inputMode = 'numeric';
  pinInput.id = `${ids}-pin`;
  pinInput.autocomplete = 'current-password';
  pinInput.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    apply();
  });
  const pinErr = el('span', 'sode__hint sode__error-text');
  pinErr.id = `${ids}-pin-err`;
  pinErr.setAttribute('role', 'alert');

  const needsPin = () => planned?.critical && S.users.length > 0;
  const label = (cat) => cats.find(([c]) => c === cat)?.[1] || cat;

  function reviewView() {
    const counts = R.reviewCounts(planned);
    const broken = R.brokenItems(planned);
    const replace = pick.mode === 'replace';
    const ends = (planned.categories.users?.items || []).filter((it) => it.note?.startsWith('Ends ')).map((it) => it.note);
    const out = [
      el('p', null, el('strong', null, replace ? 'Replace import' : 'Add import'), check.sameInstall ? ' from this install' : ' from another install'),
      el(
        'table',
        'pb-table pb-table--compact sode__xi-counts',
        el('tbody', null, ...counts.map((c) => el('tr', null, el('th', null, label(c.cat)), el('td', null, c.text)))),
      ),
    ];
    out.push(...remapReview(remap, goTo));
    if (broken.length)
      out.push(el('h3', 'sode__label sode__xi-h', `Broken (${broken.length})`), el('ul', 'sode__xi-list sode__xi-broken', ...broken.map(brokenLine)));
    if (ends.length) out.push(el('ul', 'sode__xi-list', ...ends.map((t) => el('li', null, `${t}.`))));
    if (check.unknown?.length) out.push(el('p', 'sode__hint', `Dropped, unknown to this Sode: ${check.unknown.join(', ')}.`));
    if (planned.refused) out.push(el('div', 'pb-alert pb-alert--error', `Sode refuses this import: ${planned.refused}.`));
    else if (planned.critical) {
      const row = el('div', 'sode__xi-crit', btn('pb-btn--outline pb-btn--sm', 'Back up everything first', backup));
      if (needsPin()) row.append(Object.assign(el('label', 'sode__label', 'PIN'), { htmlFor: pinInput.id }), pinInput);
      const warn = replace ? 'Critical: removes what the file does not hold.' : 'Critical: Use new overwrites current items.';
      out.push(el('div', 'pb-note pb-note--warn', warn), row, pinErr);
    }
    return out;
  }

  function brokenLine(it) {
    const li = el('li', null, `${it.as || it.name}: missing ${missing(it)}`);
    const at = it.broken.map((m) => rowOf(remap, m)).find(Boolean);
    if (at)
      li.append(
        ' ',
        btn('pb-btn--ghost pb-btn--sm', 'Show', () => goTo(...at)),
      );
    return li;
  }

  function goTo(uid, key) {
    if (remap.rowsBy[uid]?.some((x) => x.key === key && x.state === 'moved')) remap.open.add(uid);
    back();
    const sel = [...panel.querySelectorAll('[data-row], [data-term]')].find((x) => (x.dataset.row ?? x.dataset.term) === key)?.querySelector('select');
    sel?.scrollIntoView({ block: 'center' });
    sel?.focus();
  }

  function back() {
    step = 'list';
    planned = null;
    render();
  }

  async function apply() {
    if (step !== 'review' || importBtn.disabled) return;
    pinErr.textContent = '';
    importBtn.disabled = true;
    const my = ++seq;
    try {
      if (needsPin()) {
        const r = await recheckPin(pinInput.value);
        if (my !== seq) return;
        if (!r.ok) {
          pinErr.textContent = r.err;
          pinInput.select();
          return;
        }
      }
      setNote('Importing…');
      const res = await call(`${route}/apply`, { method: 'POST', body: form(fullReq()) });
      const got = await res.json();
      if (my !== seq) return;
      planned = got.plan;
      step = 'done';
      setNote('');
      failed = Object.keys(got.failed || {}).length;
      if (planned.categories.browser?.items.length) applyBrowser(check.browser, pick.mode === 'replace');
      render();
    } catch (e) {
      if (my !== seq) return;
      if (e.code === 'recheck') {
        setNote('');
        pinErr.textContent = e.message;
      } else setNote(e.message, true);
    } finally {
      importBtn.disabled = false;
    }
  }

  function doneView() {
    const counts = R.reviewCounts(planned);
    return [
      el('div', 'pb-alert pb-alert--ok', failed ? `Imported. ${failed} items were not written.` : 'Imported.'),
      el(
        'table',
        'pb-table pb-table--compact sode__xi-counts',
        el('tbody', null, ...counts.map((c) => el('tr', null, el('th', null, label(c.cat)), el('td', null, c.text)))),
      ),
    ];
  }

  function render() {
    const focused = panel.contains(document.activeElement) ? document.activeElement.dataset.key : '';
    if (step === 'file') panel.replaceChildren(fileInput);
    else if (step === 'list') panel.replaceChildren(...listView(), fileInput);
    else if (step === 'review') panel.replaceChildren(...reviewView());
    else panel.replaceChildren(...doneView());
    if (reviewing !== (step === 'review')) {
      reviewing = step === 'review';
      setReview(reviewing ? 'Import from file?' : '');
    }
    showActs();
    if (focused) [...panel.querySelectorAll('[data-key]')].find((x) => x.dataset.key === focused)?.focus();
    if (step === 'review') (needsPin() ? pinInput : importBtn).focus();
  }

  function reset() {
    seq++;
    file = check = pick = planned = remap = null;
    failed = 0;
    step = 'file';
    pinInput.value = '';
    pinErr.textContent = '';
    render();
  }

  reset();
  return { panel, acts, reset };
}
