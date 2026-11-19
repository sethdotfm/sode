// The smart menu's own items (editormenu.js draws them): Merge or Unmerge and the cell type.
import { selCells, setKind, canMerge, merge, canUnmerge, unmerge } from './gridmodel.js';
import { kindChoices, typeLabel } from './editorrules.js';

// grid() is the editor's grid, read again when an item runs. sections are addMenuSection's (grid) => items.
export function menuItems(grid, fixed, sections, setGrid) {
  const g = grid();
  const cells = selCells(g);
  const items = [
    // One item that toggles, so Ctrl+M merges a group and unmerges a merged square.
    canUnmerge(g)
      ? { chord: 'm', label: 'Unmerge', icon: 'split', run: () => setGrid(unmerge(grid()), { what: 'unmerge' }) }
      : { chord: 'm', label: 'Merge', icon: 'combine', disabled: !canMerge(g), run: () => setGrid(merge(grid()), { what: 'merge' }) },
    {
      label: typeLabel(cells),
      icon: 'type',
      sub: kindChoices({ fixed }).map(({ kind, label, disabled, hint }) => ({
        label,
        disabled,
        hint,
        current: kind === '' ? !cells.length : cells.length > 0 && cells.every((c) => c.kind === kind),
        run: () => setGrid(setKind(grid(), kind), { what: 'type change' }),
      })),
    },
  ];
  for (const section of sections) items.push(...section(g));
  return items;
}
