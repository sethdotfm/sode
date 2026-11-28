// The accent from the page border color: the rule in docs/color-roles.md ("Accent"). Pure, no imports or DOM, so Node
// tests run it (web/test/accent.test.mjs). brand.js applies the tokens and keeps them in sode-accent for prepaint.js.

// The surfaces accent text can sit on, and the text on an accent fill (--pb-text-on-accent), from patchbay 2.2.1's
// tokens. Accent text must pass 4.5:1 on the worst surface of each theme; text on a fill must pass too.
export const THEMES = {
  light: { surfaces: ['#ffffff', '#f5f5f5', '#ebebeb', '#e4e4e4', '#dcdcdc'], onAccent: '#ffffff', hover: -0.07 },
  dark: { surfaces: ['#000000', '#080808', '#0f0f0f', '#121212', '#141414', '#1e1e1e'], onAccent: '#14131f', hover: 0.07 },
};
export const MIN_CONTRAST = 4.5;

// OKLCH hues of the state colors (patchbay 2.2.1, both themes measured). A hue within RESERVED_MARGIN of a range is in
// the reserved zone: an accent there can look like a state. The free band is about 189 to 317. Step in
// (--pb-signal-10, about 320) is inside protected's zone.
export const RESERVED = [
  { state: 'failure', from: 24, to: 26 },
  { state: 'caution', from: 59, to: 81 },
  { state: 'tie line', from: 127, to: 129 },
  { state: 'routed', from: 155, to: 159 },
  { state: 'protected', from: 347, to: 350 },
];
export const RESERVED_MARGIN = 30;
// Below this chroma a color is grey: it has no hue to clash with.
const GREY = 0.02;

const lin = (c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
const gam = (c) => (c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055);

function rgbOf(hex) {
  return [1, 3, 5].map((i) => lin(parseInt(hex.slice(i, i + 2), 16) / 255));
}

export function luminance(hex) {
  const [r, g, b] = rgbOf(hex);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

export function contrast(a, b) {
  const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
}

// [L, C, h] in OKLCH.
export function oklch(hex) {
  const [r, g, b] = rgbOf(hex);
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  const L = 0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s;
  const A = 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s;
  const B = 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s;
  return [L, Math.hypot(A, B), ((Math.atan2(B, A) * 180) / Math.PI + 360) % 360];
}

// The OKLab distance of two colors (deltaE OK): about 0.02 is the least step people see. Offered theme colors are at
// least MIN_APART from each other in both themes (web/test/accent.test.mjs), so no two look alike.
export const MIN_APART = 0.1;
export function distance(a, b) {
  const [La, Ca, ha] = oklch(a);
  const [Lb, Cb, hb] = oklch(b);
  const r = Math.PI / 180;
  return Math.hypot(La - Lb, Ca * Math.cos(ha * r) - Cb * Math.cos(hb * r), Ca * Math.sin(ha * r) - Cb * Math.sin(hb * r));
}

// Linear sRGB of an OKLCH color, maybe out of gamut.
function linear(L, C, h) {
  const A = C * Math.cos((h * Math.PI) / 180);
  const B = C * Math.sin((h * Math.PI) / 180);
  const l = (L + 0.3963377774 * A + 0.2158037573 * B) ** 3;
  const m = (L - 0.1055613458 * A - 0.0638541728 * B) ** 3;
  const s = (L - 0.0894841775 * A - 1.291485548 * B) ** 3;
  return [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  ];
}

const inGamut = (rgb) => rgb.every((c) => c >= -1e-6 && c <= 1 + 1e-6);

// The hex of L and h, with the chroma lowered as far as the sRGB gamut needs.
export function toHex(L, C, h) {
  L = Math.min(1, Math.max(0, L));
  let lo = 0;
  let hi = C;
  if (!inGamut(linear(L, hi, h))) {
    for (let i = 0; i < 24; i++) {
      const mid = (lo + hi) / 2;
      if (inGamut(linear(L, mid, h))) lo = mid;
      else hi = mid;
    }
    C = lo;
  }
  return (
    '#' +
    linear(L, C, h)
      .map((c) => Math.round(Math.min(1, Math.max(0, gam(Math.min(1, Math.max(0, c))))) * 255))
      .map((n) => n.toString(16).padStart(2, '0'))
      .join('')
  );
}

export function passes(hex, theme) {
  const t = THEMES[theme];
  return t.surfaces.every((s) => contrast(hex, s) >= MIN_CONTRAST) && contrast(hex, t.onAccent) >= MIN_CONTRAST;
}

// Keeps the hue and chroma, and moves L only as far as the theme needs: darker for light, lighter for dark.
function clamp(L0, C, h, theme) {
  const fit = (L) => toHex(L, C, h);
  if (passes(fit(L0), theme)) return fit(L0);
  const up = theme === 'dark';
  let bad = L0;
  let good = up ? 1 : 0;
  for (let i = 0; i < 30; i++) {
    const mid = (bad + good) / 2;
    if (passes(fit(mid), theme)) good = mid;
    else bad = mid;
  }
  return fit(good);
}

// Hue distance on the circle.
const apart = (a, b) => Math.min(Math.abs(a - b), 360 - Math.abs(a - b));

// The states whose zone the hue is in; empty for a free hue or a grey.
export function reservedBy(hex) {
  const [, C, h] = oklch(hex);
  if (C < GREY) return [];
  return RESERVED.filter((r) => (h >= r.from && h <= r.to) || Math.min(apart(h, r.from), apart(h, r.to)) <= RESERVED_MARGIN).map((r) => r.state);
}

// The light and dark accent of a page border hex, with their hover colors, and whether the hue is reserved.
export function accentFor(hex) {
  const [L, C, h] = oklch(hex);
  const out = { reserved: reservedBy(hex).length > 0 };
  for (const theme of ['light', 'dark']) {
    const base = clamp(L, C, h, theme);
    const [bl] = oklch(base);
    out[theme] = base;
    out[`${theme}Hover`] = clamp(bl + THEMES[theme].hover, C, h, theme);
  }
  return out;
}

const rgba = (hex, a) => `rgba(${[1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16)).join(', ')}, ${a})`;

// The --pb-accent* tokens to set on the root. The others (muted, tab, subtab) mix --pb-accent in patchbay itself.
// prepaint.js sets the same names from sode-accent: keep ACCENT_TOKENS in step with it.
export const ACCENT_TOKENS = ['--pb-accent', '--pb-accent-hover', '--pb-accent-wash'];
export function accentTokens(a) {
  return {
    '--pb-accent': `light-dark(${a.light}, ${a.dark})`,
    '--pb-accent-hover': `light-dark(${a.lightHover}, ${a.darkHover})`,
    // patchbay's wash alphas.
    '--pb-accent-wash': `light-dark(${rgba(a.light, 0.12)}, ${rgba(a.dark, 0.14)})`,
  };
}

// The hex a page border value draws with: a named color's hex from the hello's list, or a custom #rrggbb.
export function brandHex(value, colors) {
  if (/^#[0-9a-f]{6}$/i.test(value || '')) return value.toLowerCase();
  return (colors || []).find((c) => c.name === value)?.hex || '';
}

// Settings and the setup wizard show it under the picker. Sode never refuses the color.
export const CAUTION_NOTE = 'This color is near a state color (failure, caution, routed or protected). A selected item can look like a state.';
export function brandCaution(value, colors) {
  const hex = brandHex(value, colors);
  return !!hex && reservedBy(hex).length > 0;
}

// The picker's colors: the offered ones, and a retired one only while it is the saved value.
export function pickerColors(colors, value) {
  return (colors || []).filter((c) => !c.retired || c.name === value);
}
