// Pure helpers shared by brand.js and settings.js; no imports or DOM at load (Node tests).

export function iconURL(size, color) {
  return `icon.png?s=${size}${color ? `&c=${encodeURIComponent(color)}` : ''}`;
}

export function setBrand(el, color) {
  if (color) el.dataset.brand = color;
  else delete el.dataset.brand;
}
