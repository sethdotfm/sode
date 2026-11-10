// Assign (GLOSSARY.md): the panels page's Assign to list and the panel page's switch to the target. Pure, so Node
// tests it. Server side: POST /api/v1/panels/{id}/assign and the instance stream's last line (internal/app/panelassign.go).
import { hasClient } from './panelsrules.js';
import { shortSecret } from './panellink.js';

// Only a panel that acts as an API client has a key to give an instance.
export const canAssignTo = (panel) => hasClient(panel);

// The open Sode panel pages (they hold the instance stream) of the panels the page can read with instances, sorted by
// their current panel's name, then the instance name. here marks the ones already on the target.
export function assignRows(panels, targetId) {
  const rows = [];
  for (const p of panels) {
    for (const i of p.instances || []) {
      if (i.stream && i.online) rows.push({ panel: p.id, panelName: p.name, device: i.device, here: p.id === targetId });
    }
  }
  const by = (a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' });
  return rows.sort((a, b) => by(a.panelName, b.panelName) || by(a.device, b.device));
}

// The stream answers JSON lines that may arrive split across chunks; rest waits for the next one.
export function splitLines(rest, chunk) {
  const parts = (rest + chunk).split('\n');
  return { lines: parts.slice(0, -1).filter((l) => l.trim()), rest: parts.at(-1) };
}

// An assign line, or null for the keepalive lines and anything malformed.
export function assignOf(line) {
  let m;
  try {
    m = JSON.parse(line);
  } catch {
    return null;
  }
  if (m?.t !== 'assign' || !m.panel || !m.key || !m.instance) return null;
  return { panel: m.panel, slug: m.slug || '', instance: m.instance, key: m.key };
}

// The target's address, built as linkFor builds one: the short name when there is one, the page's own ?instance=, and
// the fragment with its kiosk part and without an old key or setup token. seg is what the page stores the key under,
// since it reads the stored key by its address. stored false puts the key in the fragment instead.
export function assignAddress(msg, { search, hash }, stored) {
  const seg = msg.slug || msg.panel;
  const keep = hash
    .replace(/^#/, '')
    .split('&')
    .filter((p) => p && !p.startsWith('key=') && !p.startsWith('setup='));
  const frag = [...(stored ? [] : [`key=${shortSecret(msg.key)}`]), ...keep].join('&');
  return { seg, href: `panels/${seg}${search || ''}${frag ? `#${frag}` : ''}` };
}

// The seam for the editor's Authentication mode Confirm: an Assign never edits a panel, but the instance may land on a
// panel whose mode differs from the one it shows now. Where that Confirm should also cover an Assign, it hooks in here.
export const assignChangesMode = (from, to) => (from?.acts || '') !== (to?.acts || '');
