// Settings > Routers > Tie lines: suggestions (pure rules, Node-tested). Sode suggests a tie line where a destination on
// one router has the same name as a source on another router (trimmed, case ignored). A suggestion is only a hint: an
// Administrator accepts it, and nothing but the saved list of tie lines reaches a take (docs/tielines-design.md).

import { MAX_TIE_NAME } from './tielinerules.js';

const fold = (s) =>
  String(s || '')
    .trim()
    .toLowerCase();

export const endsKey = (from, to) => `${from.router}/${from.n}/${to.router}/${to.n}`;

// At most this many suggestions show. Two routers with 50 spare ends of one name give 2500 matches. A list that long
// helps nobody, and an accept or a dismissal shows the next one.
export const SUGGEST_MAX = 20;

const byNumber = (list) => [...list].sort((x, y) => x[0] - y[0]);

// routers: [{uid, dst: [[n, name]], src: [[n, name]]}]. A suggestion is {key, from, to, name}. A pair that a tie line
// already has, or that is in dismissed (a set of keys), gives none. A blank name matches nothing. The order is the
// router order, then the destination and source numbers, so the same names always give the same list. shown has the
// first SUGGEST_MAX; more counts the others.
export function suggest(routers, tieLines, dismissed) {
  const have = new Set(tieLines.map((t) => endsKey(t.from, t.to)));
  const shown = [];
  let more = 0;
  for (const a of routers) {
    for (const b of routers) {
      if (a.uid === b.uid) continue;
      const srcs = new Map();
      for (const [n, name] of byNumber(b.src)) {
        const k = fold(name);
        if (k) (srcs.get(k) || srcs.set(k, []).get(k)).push([n, String(name).trim()]);
      }
      for (const [dn, dname] of byNumber(a.dst)) {
        for (const [sn, sname] of srcs.get(fold(dname)) || []) {
          const from = { router: a.uid, n: dn };
          const to = { router: b.uid, n: sn };
          const key = endsKey(from, to);
          if (have.has(key) || dismissed.has(key)) continue;
          if (shown.length < SUGGEST_MAX) shown.push({ key, from, to, name: sname });
          else more++;
        }
      }
    }
  }
  return { shown, more };
}

// One string for what the list shows. text(s) adds what a row shows beside the suggestion (the end names). The same
// string means that the list need not be built again.
export function suggestSig({ shown, more }, text = () => '') {
  return JSON.stringify([more, shown.map((s) => [s.key, s.name, text(s)])]);
}

// A Tie line name for an accepted suggestion: the source name, with a number added when another tie line has it.
export function acceptName(name, tieLines) {
  const used = new Set(tieLines.map((t) => fold(t.name)));
  const cut = (s) => [...s].slice(0, MAX_TIE_NAME).join('');
  let out = cut(name);
  for (let i = 2; used.has(fold(out)); i++) {
    const tail = ` ${i}`;
    out = cut(name).slice(0, MAX_TIE_NAME - tail.length) + tail;
  }
  return out;
}
