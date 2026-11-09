// Panel artwork. The route needs the Authorization header, which an <img src> cannot send, so each image is fetched
// with the panel's key and shown from a blob URL. SVG is only ever used as an image, so a script in it never runs.
import { artUrlPath } from './panellook.js';

// call is the page's keyed fetch. A fetch that fails gives null and is tried again on the next load of the panel.
export function createArt(call, panelId, make = URL.createObjectURL) {
  const seen = new Map(); // artwork id -> Promise<blob URL | null>
  return {
    url(id) {
      if (!seen.has(id)) {
        seen.set(
          id,
          call(artUrlPath(panelId, id))
            .then((res) => (res.ok ? res.blob() : null))
            .then((blob) => (blob ? make(blob) : null))
            .catch(() => null),
        );
      }
      return seen.get(id);
    },
  };
}
