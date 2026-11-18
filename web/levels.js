// Typed levels, labels and colors. Pure, no imports, no page state.

function levelTok(tok, mode) {
  const t = tok.toLowerCase();
  if (mode === 'va') {
    if (t === 'v') return 1;
    const a = t.match(/^a(\d+)$/);
    if (a) return +a[1] >= 1 ? +a[1] + 1 : NaN;
  }
  return /^\d+$/.test(t) ? +t + (mode === '1' ? 0 : 1) : NaN;
}

// "1-3,5" in mode -> sorted UI levels within 1..max, or null. ! leaves out ("1-8 !3"); + is a comma; t/thru/through is -; n/not is !.
// Phone pad: . is a comma and .. is -.
export function parseLevelsAs(text, mode, max) {
  const out = new Set(),
    not = new Set();
  const norm = text
    .replace(/and/gi, ',')
    .replace(/not|n/gi, '!')
    .replace(/through|thru|t|\.{2,}/gi, '-')
    .replace(/\./g, ',')
    .replace(/\s*-\s*/g, '-')
    .replace(/!\s*/g, ' !');
  for (const part of norm.split(/[\s,+]+/).filter(Boolean)) {
    const m = part.match(/^(!?)([^-!]+)(?:-([^-!]+))?$/);
    if (!m) return null;
    let a = levelTok(m[2], mode),
      b = m[3] ? levelTok(m[3], mode) : a;
    if (!Number.isInteger(a) || !Number.isInteger(b)) return null;
    if (a > b) [a, b] = [b, a];
    for (let i = a; i <= b; i++) {
      if (i < 1 || i > max) return null;
      (m[1] ? not : out).add(i);
    }
  }
  if (!out.size && not.size) for (let i = 1; i <= max; i++) out.add(i);
  for (const i of not) out.delete(i);
  return out.size ? [...out].sort((x, y) => x - y) : null;
}

// These take levels ({ui} from the hello, {level} from /api/v1) and the Level numbering as arguments.
// A page with no socket can use them.
const uiOf = (l) => l.ui ?? l.level;

// No mode: the level's own label (the server's Level.Label); a mode previews an unsaved numbering.
export function levelLabelOf(levels, numbering, ui, mode) {
  if (mode === undefined) {
    const l = levels.find((x) => uiOf(x) === ui);
    if (l) return l.label;
    mode = numbering;
  }
  if (mode === '0') return String(ui - 1);
  if (mode === 'va') return ui === 1 ? 'V' : String(ui - 1);
  return String(ui);
}

// none: picker cleared to no levels, so blank means none. Blank otherwise means the default levels, typed in 1 numbering.
export function parseLevelsIn(text, { levels, numbering, defaultLevels }, none = false) {
  if (none && !text.trim()) return null;
  if (!text.trim()) return parseLevelsAs(defaultLevels, '1', levels.length);
  return parseLevelsAs(text, numbering, levels.length);
}

// Rewrites a box's text after numbering changed so it keeps meaning the same levels. prev is { mode, max } or falsy.
export function relabelLevelsFor(text, levels, numbering, prev) {
  if (!prev || !text.trim() || prev.mode === numbering) return text;
  const list = parseLevelsAs(text, prev.mode, prev.max);
  return list ? levelInputOf(list, levels, numbering) : text;
}

function levelRuns(list) {
  const out = [];
  for (let i = 0; i < list.length; ) {
    let j = i;
    while (j + 1 < list.length && list[j + 1] === list[j] + 1) j++;
    out.push([list[i], list[j]]);
    i = j + 1;
  }
  return out;
}
const runsText = (list, levels, numbering, dash, sep, mode) =>
  levelRuns(list)
    .map(([a, b]) => {
      const la = levelLabelOf(levels, numbering, a, mode);
      return a === b ? la : `${la}${dash}${levelLabelOf(levels, numbering, b, mode)}`;
    })
    .join(sep);
export const levelTextOf = (list, levels, numbering) => runsText(list, levels, numbering, '–', ', ');
export const levelInputOf = (list, levels, numbering, mode) => runsText(list, levels, numbering, '-', ',', mode);

// A configured #rrggbb is one color for both themes: its number gets black or white by WCAG luminance.
function inkFor(hex) {
  const lin = (i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  };
  const y = 0.2126 * lin(1) + 0.7152 * lin(3) + 0.0722 * lin(5);
  return (y + 0.05) / 0.05 > 1.05 / (y + 0.05) ? '#000' : '#fff';
}

// ink null: the swatch token carries its own ink.
export function levelStyle(l) {
  if (l.color) return { color: l.color, ink: inkFor(l.color) };
  return { color: `var(--pb-signal-${l.swatch})`, ink: null };
}
