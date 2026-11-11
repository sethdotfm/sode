// Panel artwork. The route needs the Authorization header, which an <img src> cannot send, so each image is fetched
// with the panel's key and shown from a blob URL. SVG is only ever used as an image or a CSS mask, so a script in it
// never runs.
import { artUrlPath } from './panellook.js';

// call is the page's keyed fetch. A fetch that fails gives null and is tried again on the next load of the panel.
// revoke() frees every blob URL made (a blob URL holds its bytes until then): call it when the panel this was made
// for goes away, since each panel load and each live test makes its own.
export function createArt(call, panelId, make = URL.createObjectURL, drop = URL.revokeObjectURL) {
  let seen = new Map(); // artwork id -> Promise<{ url, type } | null>; type is the served Content-Type
  const look = (id) => {
    if (!seen.has(id)) {
      const mine = seen;
      const p = call(artUrlPath(panelId, id))
        .then((res) => (res.ok ? res.blob() : null))
        .then((blob) => {
          if (!blob) return null;
          const u = make(blob);
          // Revoked or forgotten while the fetch was in flight: nothing shows it any more.
          if (mine !== seen || seen.get(id) !== p) {
            drop(u);
            return null;
          }
          return { url: u, type: blob.type || '' };
        })
        .catch(() => null);
      seen.set(id, p);
    }
    return seen.get(id);
  };
  return {
    look,
    url: (id) => look(id).then((l) => l?.url ?? null),
    // One id only, as when a library icon is deleted; the next look fetches it again.
    forget(id) {
      const p = seen.get(id);
      seen.delete(id);
      p?.then((l) => l && drop(l.url));
    },
    revoke() {
      const old = seen;
      seen = new Map();
      for (const p of old.values()) p.then((l) => l && drop(l.url));
    },
  };
}
