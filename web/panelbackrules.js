// A Public panel page that comes back by itself, and Reload from the Open now list (internal/app/publicback.go,
// internal/app/panelreload.go). Pure, so node tests read it.

// Fixed and free of any reason or name: anyone can read a Public panel's page.
export const PUBLIC_STOPPED = 'This panel is not public now. It comes back by itself when it is public again.';

// The longest random wait before a page asks after a public line. It spreads a wall of tablets behind one address
// under publicAskMax.
export const BACK_MAX_MS = 5000;

export const backWaitMs = (rand = Math.random) => Math.round(rand() * BACK_MAX_MS);

// 'public' or 'reload' for those stream lines, else ''.
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

// The Reload control of an Open now row (an instance or a keyless page): '' for none, since only a Sode panel page
// hears it, 'offline' when its stream is gone and nothing would hear it.
export const reloadState = (r) => (!r?.page && !r?.keyless ? '' : r.stream ? 'ready' : 'offline');

// Reload all: ready while any page of the panel holds a stream.
export function reloadAllState(p) {
  const rows = [...(p.instances || []), ...(p.keyless || [])].filter((r) => reloadState(r));
  if (!rows.length) return '';
  return rows.some((r) => r.stream) ? 'ready' : 'offline';
}
