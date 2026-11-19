// The Import tab of the Export & Import dialog (exportdialog.js) is one long review of the file. Review… puts the
// one Save review in the dialog body. The server plans every step (POST api/import, docs/agents/profiles.md);
// the page sends its ticks and choices and shows what the server answers. Pure rules are in importrules.js.

import { el, off, S } from './core.js';
import { call } from './exportcalls.js';
import { recheckPin } from './recheck.js';
import { segmented } from './formbits.js';
import { CATEGORIES, sizeText } from './exportrules.js';
import * as R from './importrules.js';

const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};

// The Artwork and This browser steps are not on the server yet; the review lists them as not imported.
const NOT_YET = new Set(['artwork', 'browser']);

// opts: setNote(text, error), setReview(title or ''), backup() downloads Back up everything.
export function importTab({ setNote, setReview, backup }) {
  const panel = el('div', 'sode__xi-import');
  const fileInput = el('input');
  fileInput.type = 'file';
  fileInput.accept = '.zip,application/zip';
  fileInput.id = 'xi-file';
  fileInput.hidden = true;
  const chooseBtn = btn('pb-btn--ok', 'Choose a file…', () => fileInput.click());
  chooseBtn.id = 'xi-choose';
  const brokenBadge = el('span', 'pb-badge pb-badge--error');
  const reviewBtn = btn('pb-btn--ok', 'Review…', () => review());
  reviewBtn.id = 'xi-review';
  const backBtn = btn('pb-btn--outline', 'Back', () => back());
  backBtn.id = 'xi-back';
  const importBtn = btn('pb-btn--ok', 'Import', () => apply());
  importBtn.id = 'xi-import';
  const anotherBtn = btn('pb-btn--outline', 'Import another', () => reset());
  const acts = el('span', 'sode__xi-acts', brokenBadge, chooseBtn, reviewBtn, backBtn, importBtn, anotherBtn);

  let file = null;
  let check = null;
  let pick = null;
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
    const broken = step === 'list' ? R.brokenItems(check.plan).filter((it) => pick.items[it.cat]?.has(it.key)).length : 0;
    brokenBadge.hidden = !broken;
    brokenBadge.textContent = `${broken} Broken`;
    reviewBtn.disabled = step === 'list' && !R.importReq(pick).categories.length;
  };

  function form(req) {
    const fd = new FormData();
    fd.append('plan', JSON.stringify(req)); // the server reads the plan first
    fd.append('file', file);
    return fd;
  }

  async function load(f) {
    file = f;
    const my = ++seq;
    setNote('Reading the file…');
    try {
      const res = await call('api/import', { method: 'POST', body: form({ mode: 'add', categories: R.PLANNED }) });
      const got = await res.json();
      if (my !== seq) return;
      check = got;
      pick = R.newPick(check);
      expanded.clear();
      for (const cat of R.PLANNED) if (check.plan.categories[cat]?.items.length) expanded.add(cat);
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

  // ---- the list

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
    const seg = segmented(`xi-choice-${cat}-${i}`, `${it.name} clash`, R.choicesFor(cat), (v) => {
      pick.choices[cat][it.key] = v;
      render();
    });
    seg.set(pick.choices[cat][it.key]);
    for (const r of seg.node.querySelectorAll('input')) r.dataset.key = `choice:${cat}:${it.key}:${r.value}`;
    seg.node.classList.add('sode__xi-choice');
    return el('div', 'sode__xi-sub', seg.node);
  }

  const missing = (it) => it.broken.map((m) => R.missingText(m, off())).join(', ');

  function itemRow(cat, c, it, i) {
    const ticked = pick.items[cat].has(it.key);
    const hint = it.refused ? it.refused : it.same ? 'No change' : it.note || '';
    const row = el(
      'div',
      'sode__xi-item',
      box(`item:${cat}:${it.key}`, it.name, ticked, false, !!c.refused, (on) => {
        if (on) pick.items[cat].add(it.key);
        else pick.items[cat].delete(it.key);
        render();
      }),
      el('span', 'sode__hint', ...(it.broken?.length ? [el('span', 'pb-badge pb-badge--error', 'Broken'), ' '] : []), hint),
      el('span', 'sode__xi-size', sizeText(it.size || 0)),
    );
    row.dataset.item = it.key;
    if (ticked && R.hasChoice(c, it)) row.append(choices(cat, it, i));
    if (it.broken?.length) row.append(el('span', 'sode__hint sode__xi-sub sode__error-text', `Missing ${missing(it)}`));
    return row;
  }

  function catRows(cat, text) {
    const c = check.plan.categories[cat];
    const notYet = NOT_YET.has(cat);
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
    const why = notYet ? 'Not imported yet' : c?.refused || '';
    head.append(
      caret,
      box(`cat:${cat}`, text + count, state !== 'none', state === 'some', notYet || !!c?.refused || !items.length, (on) => {
        R.setCat(check, pick, cat, on);
        render();
      }),
      el('span', `sode__hint${c?.refused ? ' sode__error-text' : ''}`, why),
      el('span', 'sode__xi-size', sizeText(R.catSize(check, pick, cat))),
    );
    if (!expanded.has(cat) || caret.hidden) return [head];
    return [head, el('div', 'sode__xi-items', ...items.map((it, i) => itemRow(cat, c, it, i)))];
  }

  // The file's routers against this one. Remapping onto another router is a later step.
  function routerNames() {
    const live = check.plan.categories.routers?.items || [];
    const lines = (check.routers || []).map((r) => {
      const here = live.find((it) => it.key === r.UID);
      const name = r.Title || r.Prefix;
      return el('li', null, here && !here.note ? `${name}: this router` : `${name}: another router. Items that use it arrive Broken.`);
    });
    if (!lines.length) return [];
    return [el('h3', 'sode__label sode__xi-h', 'Router names'), el('ul', 'sode__xi-list', ...lines)];
  }

  function listView() {
    const rows = [];
    for (const [cat, text] of CATEGORIES) if (check.categories.includes(cat)) rows.push(...catRows(cat, text));
    return [
      el(
        'div',
        'sode__xi-file',
        el('strong', null, file.name),
        el('span', 'sode__xi-size', sizeText(file.size)),
        btn('pb-btn--ghost pb-btn--sm', 'Change', () => fileInput.click()),
        el('span', 'sode__hint sode__xi-sub', `Sode ${check.sode}, ${check.sameInstall ? 'this install' : 'another install'}`),
      ),
      el('div', 'sode__xi-mode', el('strong', null, 'Add import'), el('span', 'sode__hint', 'Adds and updates. Deletes nothing.')),
      el('div', 'sode__xi-cats', ...rows),
      ...routerNames(),
    ];
  }

  // ---- the review

  async function review() {
    const my = ++seq;
    setNote('Checking your choices…');
    reviewBtn.disabled = true;
    try {
      const res = await call('api/import', { method: 'POST', body: form(R.importReq(pick)) });
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
  pinInput.id = 'xi-pin';
  pinInput.autocomplete = 'current-password';
  pinInput.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    apply();
  });
  const pinErr = el('span', 'sode__hint sode__error-text');
  pinErr.id = 'xi-pin-err';
  pinErr.setAttribute('role', 'alert');

  const needsPin = () => planned?.critical && S.users.length > 0;
  const label = (cat) => CATEGORIES.find(([c]) => c === cat)?.[1] || cat;

  function reviewView() {
    const counts = R.reviewCounts(planned);
    const broken = R.brokenItems(planned);
    const notYet = [...(planned.deferred || [])];
    for (const cat of NOT_YET) if (check.categories.includes(cat)) notYet.push(label(cat));
    const out = [
      el('p', null, el('strong', null, 'Add import'), check.sameInstall ? ' from this install' : ' from another install'),
      el(
        'table',
        'pb-table pb-table--compact sode__xi-counts',
        el('tbody', null, ...counts.map((c) => el('tr', null, el('th', null, label(c.cat)), el('td', null, c.text)))),
      ),
    ];
    if (broken.length)
      out.push(
        el('h3', 'sode__label sode__xi-h', `Broken (${broken.length})`),
        el('ul', 'sode__xi-list sode__xi-broken', ...broken.map((it) => el('li', null, `${it.as || it.name}: missing ${missing(it)}`))),
      );
    if (notYet.length) out.push(el('p', 'sode__hint', `Not imported yet: ${notYet.join(', ')}.`));
    if (check.unknown?.length) out.push(el('p', 'sode__hint', `Dropped, unknown to this Sode: ${check.unknown.join(', ')}.`));
    if (planned.critical) {
      const row = el('div', 'sode__xi-crit', btn('pb-btn--outline pb-btn--sm', 'Back up everything first', backup));
      if (needsPin()) row.append(Object.assign(el('label', 'sode__label', 'PIN'), { htmlFor: 'xi-pin' }), pinInput);
      out.push(el('div', 'pb-note pb-note--warn', 'Use new replaces what is here. This is a Critical action.'), row, pinErr);
    }
    return out;
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
      const res = await call('api/import/apply', { method: 'POST', body: form(R.importReq(pick)) });
      const got = await res.json();
      if (my !== seq) return;
      planned = got.plan;
      step = 'done';
      setNote('');
      failed = Object.keys(got.failed || {}).length;
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
      el('div', 'pb-alert pb-alert--ok', failed ? `Imported. ${failed} panels were not written.` : 'Imported.'),
      el(
        'table',
        'pb-table pb-table--compact sode__xi-counts',
        el('tbody', null, ...counts.map((c) => el('tr', null, el('th', null, label(c.cat)), el('td', null, c.text)))),
      ),
    ];
  }

  function render() {
    const focused = panel.contains(document.activeElement) ? document.activeElement.dataset.key : '';
    if (step === 'file') panel.replaceChildren(el('p', 'sode__hint', 'Choose an export file (.zip).'), fileInput);
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
    file = check = pick = planned = null;
    failed = 0;
    step = 'file';
    pinInput.value = '';
    pinErr.textContent = '';
    render();
  }

  reset();
  return { panel, acts, reset };
}
