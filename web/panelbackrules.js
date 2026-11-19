// A Public panel page that comes back by itself, and Reload from the Open now list (internal/app/publicback.go,
// internal/app/panelreload.go). Pure, so node tests read it.
import { renameState } from './panelassignrules.js';

// The longest random wait before a page asks after a public line. It spreads a wall of tablets behind one address
// under publicAskMax.
export const BACK_MAX_MS = 5000;

export const backWaitMs = (rand = Math.random) => Math.round(rand() * BACK_MAX_MS);

export function lineType(line) {
  try {
    const t = JSON.parse(line)?.t;
    return t === 'public' || t === 'reload' ? t : '';
  } catch {
    return '';
  }
}

// back marks the ask that follows a public line, so Sode writes its Log row.
export const publicAskPath = (id, back) => `api/v1/panels/${id}/public${back ? '?back=1' : ''}`;

export const reloadState = renameState;

export function reloadAllState(p) {
  const rows = [...(p.instances || []), ...(p.keyless || [])].filter((r) => reloadState(r));
  if (!rows.length) return '';
  return rows.some((r) => r.stream) ? 'ready' : 'offline';
}
