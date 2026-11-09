// Panel artwork. The route needs the Authorization header, which an <img src> cannot send, so each image is fetched
// with the panel's key and shown from a blob URL. SVG is only ever used as an image, so a script in it never runs.
import { artUrlPath } from './panellook.js';

// call is the page's keyed fetch. A fetch that fails gives null and is tried again on the next load of the panel.
// revoke() frees every blob URL made (a blob URL holds its bytes until then): call it when the panel this was made
// for goes away, since each panel load and each live test makes its own.
export function createArt(call, panelId, make = URL.createObjectURL, drop = URL.revokeObjectURL) {
  let seen = new Map(); // artwork id -> Promise<blob URL | null>
  return {
    url(id) {
      if (!seen.has(id)) {
        const mine = seen;
        seen.set(
          id,
          call(artUrlPath(panelId, id))
            .then((res) => (res.ok ? res.blob() : null))
            .then((blob) => {
              if (!blob) return null;
              const u = make(blob);
              // Revoked while the fetch was in flight: nothing shows it any more.
              if (mine !== seen) {
                drop(u);
                return null;
              }
              return u;
            })
            .catch(() => null),
        );
      }
      return seen.get(id);
    },
    revoke() {
      const old = seen;
      seen = new Map();
      for (const p of old.values()) p.then((u) => u && drop(u));
    },
  };
}
