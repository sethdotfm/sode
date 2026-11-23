// Settings > Routers > Router levels: the level settings of each router after the first, as the one Save keys
// router/<uid>/<attribute> (docs/agents/settings-saves.md). The first router's are the plain keys of its own block.
// Pure, no page state (Node-tested).

import { parseLevelsAs, levelInputOf } from './levels.js';

export const routerKey = (uid, attr) => `router/${uid}/${attr}`;

// Default levels are saved in 1 numbering, as the first router's are. A text that does not parse stays as it is.
const canonical = (text, count) => {
  const list = parseLevelsAs(text || '', '1', count);
  return list ? levelInputOf(list, [], '1') : text || '';
};

// The saved level settings of router r of the hello's routers list ({uid, levels, levelNumbering, defaultLevels}).
// The level offset is the wire level of UI level 1, less 1.
export function routerLevelsOf(r) {
  const lv = r.levels || [];
  const count = lv.length;
  return {
    levels: count,
    levelWireOffset: count ? lv[0].wire - lv[0].ui : 0,
    levelNumbering: r.levelNumbering || '1',
    defaultLevels: canonical(r.defaultLevels, count),
  };
}

// The box text of the default levels, in the router's numbering.
export function defaultLevelsText(v) {
  const list = parseLevelsAs(v.defaultLevels || '', '1', v.levels);
  return list ? levelInputOf(list, [], v.levelNumbering) : v.defaultLevels || '';
}

// The values the inputs hold: levels and offset as typed, numbering, and the default levels text in that numbering.
// Default levels that do not parse are null; blank is '' (the server then uses every level).
export function readRouterLevels(input) {
  const int = (s) => (String(s).trim() === '' ? NaN : Number(s));
  const levels = int(input.levels);
  const text = String(input.defaultText || '').trim();
  let defaultLevels = '';
  if (text) {
    const list = Number.isInteger(levels) ? parseLevelsAs(text, input.levelNumbering, levels) : null;
    defaultLevels = list ? levelInputOf(list, [], '1') : null;
  }
  return { levels, levelWireOffset: int(input.levelWireOffset), levelNumbering: input.levelNumbering, defaultLevels };
}

// The one Save fields of router uid: {key: value}.
export const routerFields = (uid, v) => Object.fromEntries(Object.entries(v).map(([attr, x]) => [routerKey(uid, attr), x]));

// The first text that stops a save of router prefix's values v, or ''.
export function routerLevelsProblem(prefix, v) {
  const name = `Router ${prefix}`;
  if (!Number.isInteger(v.levels) || v.levels < 1 || v.levels > 256) return `${name}: Levels must be a whole number from 1 to 256.`;
  if (!Number.isInteger(v.levelWireOffset)) return `${name}: Level offset must be a whole number.`;
  if (v.defaultLevels === null) return `${name}: Default levels is not a level or range of this router.`;
  return '';
}

// The Level numbering choices, in the order the select shows them.
export const NUMBERING = Object.freeze([
  ['1', 'Start at 1'],
  ['0', 'Start at 0'],
  ['va', 'V, then 1'],
]);
export const numberingText = (v) => NUMBERING.find(([x]) => x === v)?.[1] || String(v);
