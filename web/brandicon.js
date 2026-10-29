// Pure helpers shared by brand.js and settings.js (no imports, no DOM
// access at load, so Node can test them).

// The icon the server draws (icon.go): size is 180, 192 or 512.
export function iconURL(size, color) {
  return `icon.png?s=${size}${color ? `&c=${encodeURIComponent(color)}` : ''}`;
}

// A brand colour on an element ([data-brand] picks the token in base.css).
export function setBrand(el, color) {
  if (color) el.dataset.brand = color;
  else delete el.dataset.brand;
}
