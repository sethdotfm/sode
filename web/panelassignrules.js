// Assign (GLOSSARY.md): the panels page's Assign to list and the panel page's switch to the target. Pure, so Node
// tests it. Server side: POST /api/v1/panels/{id}/assign and the instance stream's last line (internal/app/panelassign.go).
import { hasClient } from './panelsrules.js';
import { shortSecret } from './panellink.js';
import { durationText } from './panelwatchrules.js';

// Only a panel that acts as an API client has a key to give an instance.
export const canAssignTo = (panel) => hasClient(panel);

// signIn is the server's word for who opened the last stream; "ended" and "none" are a page with no working key.
export function signInText(i) {
  if (i.signIn === 'client') return "Signed in as the panel's client";
  if (i.signIn === 'user' && i.user) return `Signed in as ${i.user}`;
  if (i.signIn === 'ended') return 'Key ended';
  return 'Not signed in';
}

// Whole minutes: the list redraws only when a row's text changes, and it is read every MON_POLL_MS.
export const uptimeText = (secs) => (secs < 60 ? 'under a minute' : durationText(Math.floor(secs / 60) * 60));

// The Sode panel pages (page: they held the instance stream) of the panels the page can read with instances, sorted
// by their current panel's name, then the instance name. here marks the ones already on the target; timedOut the ones
// with no stream now, which have nothing to send on. The server keeps an instance 24 hours after it was last heard.
export function assignRows(panels, targetId, now) {
  const rows = [];
  for (const p of panels) {
    for (const i of p.instances || []) {
      if (!i.page) continue;
      const here = p.id === targetId;
      const timedOut = !i.stream;
      const secs = (t) => (now - Date.parse(t)) / 1000;
      rows.push({
        panel: p.id,
        panelName: p.name,
        device: i.device,
        addr: i.addr || '',
        state: signInText(i),
        uptime: timedOut ? `last seen ${uptimeText(secs(i.seen))} ago` : `up ${uptimeText(secs(i.streamSince))}`,
        here,
        timedOut,
        assignable: !here && !timedOut,
      });
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
