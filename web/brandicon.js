// Pure helpers shared by brand.js and settings.js; no imports or DOM at load (Node tests).

export function iconURL(size, color) {
  return `icon.png?s=${size}${color ? `&c=${encodeURIComponent(color)}` : ''}`;
}

// A named color comes from base.css's [data-brand] rules; a custom #rrggbb sets --brand-color itself.
export function setBrand(el, color) {
  const custom = /^#[0-9a-f]{6}$/i.test(color || '');
  if (custom) el.style.setProperty('--brand-color', color);
  else el.style.removeProperty('--brand-color');
  if (color) el.dataset.brand = custom ? 'custom' : color;
  else delete el.dataset.brand;
}
