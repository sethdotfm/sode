// Pure helpers shared by brand.js and settings.js; no imports or DOM at load (Node tests).

export function iconURL(size, color) {
  return `icon.png?s=${size}${color ? `&c=${encodeURIComponent(color)}` : ''}`;
}

// A named color goes through its [data-brand] rule, a custom #rrggbb straight into --brand-color.
export function setBrand(el, color) {
  const own = color?.startsWith('#');
  if (color && !own) el.dataset.brand = color;
  else delete el.dataset.brand;
  if (own) el.style.setProperty('--brand-color', color);
  else el.style.removeProperty('--brand-color');
}
