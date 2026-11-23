// The artwork each Grid panel name shows (view.art). Images load with the panel's key, so a name paints without one
// until its fetch lands, then paints again.
import { nameArt } from './gridpanelart.js';
import { defaultsBy } from './panellook.js';

// panel() is the run's panel; repaint() runs when an image lands, unless stopped(). first() is the first router's UID,
// or '' while it is not known (defaultsBy).
export function gridArtwork(ctx, panel, repaint, stopped, first = () => '') {
  const images = new Map(); // artwork id -> { url, type }, or null while it loads or after it failed
  const imageOf = (id) => {
    if (!images.has(id)) {
      images.set(id, null);
      ctx.art?.look(id).then((l) => {
        if (!l || stopped()) return;
        images.set(id, l);
        repaint();
      });
    }
    return images.get(id);
  };
  let defaultsFor = null; // the artDefaults list that defaults was made from
  let firstFor = '';
  let defaults = new Map();
  return function nameArtOf(kind, n) {
    const p = panel();
    if (defaultsFor !== p.artDefaults || firstFor !== first()) {
      defaultsFor = p.artDefaults;
      firstFor = first();
      defaults = defaultsBy(defaultsFor, firstFor);
    }
    const art = kind === 'srcs' ? nameArt(p.sources, 'source', n, defaults) : nameArt(p.destinations, 'destination', n, defaults);
    if (!art) return null;
    const out = { b: art.b };
    if (art.hide) out.hide = true;
    if (art.a?.icon) out.icon = art.a.icon;
    else if (art.a) {
      const l = imageOf(art.a.id);
      if (l) out[art.a.lib && l.type.startsWith('image/svg+xml') ? 'mask' : 'url'] = l.url;
    }
    return out;
  };
}
