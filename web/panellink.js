// The one place that builds a panel's link text, so a short link can replace the id URL here and nowhere else.
// base is Sode's root (the pages set <base href>), passed only by tests; kiosk asks for the link that hides the
// header (kioskOf in panelrules.js).
export function linkFor(panel, { kiosk = false, base = globalThis.document?.baseURI } = {}) {
  const url = new URL(`panels/${panel.id}`, base);
  if (kiosk) url.hash = 'kiosk=1';
  return url.href;
}
