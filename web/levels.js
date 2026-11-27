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

// These get levels ({ui} from the hello, {level} from /api/v1) and the Level numbering as arguments.
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

// The most levels a router can have (config levels 1-256).
const MAX_LEVELS = 256;

// none: picker cleared to no levels, so blank means none. Blank otherwise means the default levels, typed in 1 numbering;
// blank default levels are every level.
// The levels are those of one router. A router with one level cannot break away: any typed levels mean its one level,
// the whole signal (docs/tielines-design.md, Levels).
export function parseLevelsIn(text, { levels, numbering, defaultLevels }, none = false) {
  if (none && !text.trim()) return null;
  if (levels.length === 1) return parseLevelsAs(text.trim() ? text : '1', numbering, MAX_LEVELS) && [1];
  if (!text.trim()) return defaultLevels?.trim() ? parseLevelsAs(defaultLevels, '1', levels.length) : levels.map(uiOf);
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
// V never starts a range: V and then 1 to 3 is "V, 1–3", since "V–3" reads as "V minus 3" (LP-21). Keep in step with
// Router.LevelsText in internal/config/config.go.
const runsText = (list, levels, numbering, dash, sep, mode) =>
  levelRuns(list)
    .flatMap(([a, b]) =>
      a < b && levelLabelOf(levels, numbering, a, mode) === 'V'
        ? [
            [a, a],
            [a + 1, b],
          ]
        : [[a, b]],
    )
    .map(([a, b]) => {
      const la = levelLabelOf(levels, numbering, a, mode);
      return a === b ? la : `${la}${dash}${levelLabelOf(levels, numbering, b, mode)}`;
    })
    .join(sep);
export const levelTextOf = (list, levels, numbering) => runsText(list, levels, numbering, '–', ', ');
export const levelInputOf = (list, levels, numbering, mode) => runsText(list, levels, numbering, '-', ',', mode);
// A list of levels as the file and the Settings saves record write it, in numbering mode: "V, 1-2, 4". The Save review
// and the Default levels box show it.
export const levelsTextAs = (list, mode) => runsText(list, [], mode, '-', ', ', mode);

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

// The router of a route's typed levels. That is the router of its first destination, else the router that the
// destination text names with its prefix, else the router of its source. routerOf gives an item's router ('' for the
// first router); routerOfText the router a text names ('' for none).
export function levelRouterOf({ dsts = [], dstText = '', src = null }, routerOf, routerOfText = () => '') {
  if (dsts.length) return routerOf(dsts[0]);
  return routerOfText(dstText) || (src && routerOf(src)) || '';
}

// The desktop main page shows its Levels box only when some router has more than one level. counts: each router's
// levels setting. With no counts yet (before the hello), the box shows, as it did before this rule.
export const levelsBoxShown = (counts) => !counts.length || counts.some((n) => n > 1);
