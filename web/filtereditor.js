// One filter's rule editor (Sources or Destinations) in the Filters dialog: rows or code, with a
// preview. read() gives the filter being edited, write(next) stores it, mode() is 'visual' or
// 'code'; filters.js decides where those point (the browser's own filter or a profile draft).

import { $, el, S, off } from './core.js';
import { parseFilter, filterText, compileFilter } from './filter.js';

const KINDS = [
  ['name', 'Name contains'],
  ['exact', 'Name is'],
  ['num', 'Number is'],
  ['range', 'Number between'],
  ['re', 'Name matches regex'],
];

function complete(r) {
  if (r.kind === 'num') return Number.isInteger(r.a);
  if (r.kind === 'range') return Number.isInteger(r.a) && Number.isInteger(r.b);
  if (r.kind === 're') {
    if (!r.text) return false;
    try {
      new RegExp(r.text, 'i');
      return true;
    } catch {
      return false;
    }
  }
  return !!r.text;
}

export function editor(key, { read, write, mode }) {
  const noun = key === 'src' ? 'sources' : 'destinations';
  const box = $(`flt-${key}`);
  const ed = { text: '', rows: [], error: null };
  let timer = 0;

  function changed(now = false) {
    const p = parseFilter(ed.text);
    ed.error = p.error ? `${p.error}${p.at != null ? ` at “${ed.text.slice(p.at, p.at + 12)}”` : ''}` : null;
    clearTimeout(timer);
    const go = () => {
      if (!ed.error) write({ ...read(), [key]: ed.text });
      preview();
    };
    if (now) go();
    else timer = setTimeout(go, 150);
    if (ed.error) preview();
  }

  function preview() {
    const note = box.querySelector('.sode__filters-preview');
    const err = box.querySelector('.pb-note');
    const input = box.querySelector('.sode__filters-code');
    input?.classList.toggle('pb-input--invalid', !!ed.error);
    err.hidden = !ed.error;
    err.textContent = ed.error ? `${ed.error}` : '';
    const list = key === 'src' ? S.srcs : S.dsts;
    const p = parseFilter(ed.text);
    if (p.error) return;
    if (!p.rules.length) {
      note.textContent = `All ${noun}`;
      return;
    }
    if (!list.length) {
      note.textContent = '';
      return;
    }
    const fn = compileFilter(p.rules);
    const hits = list.filter((i) => fn(i.n + off(), i.name));
    const shown = hits
      .slice(0, 4)
      .map((i) => i.label)
      .join(', ');
    note.textContent = hits.length ? `${hits.length} of ${list.length}: ${shown}${hits.length > 4 ? ', …' : ''}` : `No ${noun} match`;
  }

  function fromRows() {
    ed.text = filterText(ed.rows.filter(complete));
    changed();
  }

  function rowNode(r, i) {
    const sel = (opts, value, label, onChange) => {
      const s = el('select', 'pb-select');
      for (const [v, t] of opts) {
        const o = el('option', null, t);
        o.value = v;
        s.append(o);
      }
      s.value = value;
      s.setAttribute('aria-label', label);
      s.addEventListener('change', () => onChange(s.value));
      return el('span', 'pb-select-wrap', s);
    };
    const input = (value, label, onInput, numeric) => {
      const f = el('input', 'pb-input');
      f.value = value ?? '';
      f.spellcheck = false;
      f.autocomplete = 'off';
      if (numeric) f.inputMode = 'numeric';
      f.setAttribute('aria-label', label);
      f.addEventListener('input', () => {
        onInput(f.value);
        for (const x of f.closest('.sode__filters-rule').querySelectorAll('input')) {
          x.classList.toggle('pb-input--invalid', x.value.trim() !== '' && !complete(r));
        }
        fromRows();
      });
      return f;
    };
    const num = (v) => (/^\s*\d+\s*$/.test(v) ? Number(v) : NaN);
    const n = i + 1;
    const values =
      r.kind === 'range'
        ? [
            input(
              r.a,
              `Rule ${n} from`,
              (v) => {
                r.a = num(v);
              },
              true,
            ),
            el('span', 'sode__hint', 'to'),
            input(
              r.b,
              `Rule ${n} to`,
              (v) => {
                r.b = num(v);
              },
              true,
            ),
          ]
        : r.kind === 'num'
          ? [
              input(
                r.a,
                `Rule ${n} number`,
                (v) => {
                  r.a = num(v);
                },
                true,
              ),
            ]
          : [
              input(r.text, `Rule ${n} ${r.kind === 're' ? 'regex' : 'name'}`, (v) => {
                r.text = v;
                delete r.quoted;
              }),
            ];
    const remove = el('button', 'pb-btn pb-btn--ghost', 'Remove');
    remove.type = 'button';
    remove.setAttribute('aria-label', `Remove rule ${n}`);
    remove.addEventListener('click', () => {
      ed.rows.splice(i, 1);
      fromRows();
      render();
      box.querySelector('.sode__filters-add')?.focus();
    });
    return el(
      'div',
      'sode__filters-rule',
      sel(
        [
          ['in', 'Include'],
          ['out', 'Exclude'],
        ],
        r.not ? 'out' : 'in',
        `Rule ${n}: include or exclude`,
        (v) => {
          r.not = v === 'out';
          fromRows();
        },
      ),
      sel(KINDS, r.kind, `Rule ${n}: what to match`, (v) => {
        if ((v === 'num' || v === 'range') !== (r.kind === 'num' || r.kind === 'range')) Object.assign(r, { a: undefined, b: undefined, text: '' });
        r.kind = v;
        fromRows();
        render();
        box.querySelectorAll('.sode__filters-rule')[i]?.querySelector('input')?.focus();
      }),
      el('span', 'sode__filters-vals', ...values),
      remove,
    );
  }

  function render() {
    const head = el('div', 'sode__filters-ed-head', el('span', 'sode__label', key === 'src' ? 'Sources' : 'Destinations'));
    let body;
    if (mode() === 'code') {
      const f = el('input', 'pb-input sode__code sode__filters-code');
      f.value = ed.text;
      f.placeholder = key === 'src' ? 'CAM, 1-12, !Black' : 'DSM, /^MV\\d+ PIP/, !65';
      f.spellcheck = false;
      f.autocomplete = 'off';
      f.setAttribute('aria-label', `${key === 'src' ? 'Sources' : 'Destinations'} filter`);
      f.addEventListener('input', () => {
        ed.text = f.value;
        changed();
      });
      body = f;
    } else {
      const add = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode__filters-add', '+ Add rule');
      add.type = 'button';
      add.addEventListener('click', () => {
        ed.rows.push({ not: false, kind: 'name', text: '' });
        render();
        const rules = box.querySelectorAll('.sode__filters-rule');
        rules[rules.length - 1]?.querySelector('input')?.focus();
      });
      body = el('div', 'sode__filters-rules', ...ed.rows.map(rowNode), el('div', null, add));
    }
    const err = el('div', 'pb-note pb-note--error');
    err.setAttribute('role', 'alert');
    err.hidden = true;
    box.replaceChildren(head, body, el('div', 'sode__hint sode__filters-preview'), err);
    preview();
  }

  return {
    load(text) {
      ed.text = text;
      const p = parseFilter(text);
      ed.rows = p.rules ? p.rules.map((r) => ({ ...r })) : [];
      ed.error = null;
      render();
      if (p.error) changed(true);
    },
    toVisual() {
      const p = parseFilter(ed.text);
      if (p.error) {
        changed(true);
        return false;
      }
      ed.rows = p.rules.map((r) => ({ ...r }));
      return true;
    },
    render,
    preview,
    flush() {
      clearTimeout(timer);
      if (!ed.error) write({ ...read(), [key]: ed.text });
      preview();
    },
  };
}
