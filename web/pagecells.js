// Page Name and Info cells (GLOSSARY.md): what they show. Pure. A cell's `show` is the list of parts it shows, in any
// order; with no list it shows SHOW_DEFAULT. Keep SHOW_PARTS in step with cellShowParts in internal/app/panelpagelist.go.
import { pageLabel } from './panelpages.js';
import { pageIndex } from './panelnav.js';

export const PAGE_KINDS = ['pageName', 'info'];
export const SHOW_PARTS = { pageName: ['number', 'name'], info: ['panel', 'number', 'page', 'instance'] };
const SHOW_DEFAULT = { pageName: ['name'], info: ['panel', 'page', 'instance'] };
export const PART_WORDS = { number: 'Page number', name: 'Page name', panel: 'Panel name', page: 'Page name', instance: 'Instance name' };

export function showOf(cell) {
  const all = SHOW_PARTS[cell.kind] || [];
  const want = Array.isArray(cell.show) ? cell.show : SHOW_DEFAULT[cell.kind] || [];
  return all.filter((p) => want.includes(p));
}

// page is the id of the Page the cell sits on, so a copy on another Page names that Page. A page with no instance yet
// (still naming itself, or Test it live) shows the kind's words, as nameCellText does.
export function pageCellText(cell, { panelName = '', instance = '', pages = [], page = '' } = {}) {
  const i = pageIndex(pages, page);
  const parts = showOf(cell);
  const number = `Page ${i + 1}`;
  const name = pageLabel(pages[i], i);
  let lines;
  if (cell.kind === 'pageName') {
    const named = !!pages[i]?.name?.trim();
    const v = parts.length === 2 ? (named ? `${number}, ${name}` : number) : parts[0] === 'number' ? number : parts[0] === 'name' ? name : '';
    lines = v ? [v] : [];
  } else {
    lines = [];
    for (const p of parts) {
      if (p === 'panel') lines.push(panelName || 'Panel Name');
      if (p === 'instance') lines.push(instance || 'Instance Name');
      if (p === 'number' && !(parts.includes('page') && !pages[i]?.name?.trim())) lines.push(number);
      if (p === 'page') lines.push(name);
    }
  }
  return { caption: cell.caption || '', lines, value: lines.join(' · ') };
}
