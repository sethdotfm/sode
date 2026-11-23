// The Via nodes of the main page (docs/tielines-design.md, Via). The Via mark goes after a source that tie lines feed.
// A mixed chain also shows its level pie. The words are viarules.js's.
import { el, endSrcLabel, routerOfItem } from './core.js';
import { viaWord, viaText, pieGroups } from './viarules.js';
import { levelBar } from './levelview.js';

// viaMark: "Via TIE 3, TIE 9" in the tie line color, "(mixed)" added for a mixed chain. The tip has the full chain.
export function viaMark(row) {
  const m = el('span', 'sode-via', `${viaWord(row)}${row.mixed ? ' (mixed)' : ''}`);
  m.dataset.pbTip = viaText(row, endSrcLabel);
  return m;
}

// viaWordMark: the Via mark of words that are made already (a History row's "Via TIE 3").
export const viaWordMark = (text) => el('span', 'sode-via', text);

// pieRows: the level pie of a mixed chain, one row for each origin upstream of the nearest tie line. Each row has the
// levels of that router that carry the origin.
export function pieRows(row, srcEnd) {
  if (!row.mixed) return [];
  const up = routerOfItem({ router: row.levelsRouter });
  return pieGroups(row).map((g) =>
    el('div', 'sode__route sode-via__pie', levelBar(g.levels, up), srcEnd(g.origin, g.origin ? endSrcLabel(g.origin) : 'nothing')),
  );
}
