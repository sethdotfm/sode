// The lock screen's site name: its flight from the page's own heading and its fit on a phone (lock.js).
import { layout } from './layout.js';
import { liftFrom, shouldFly, brandFrames, OUT_FACTOR } from './lockrules.js';

const logo = () =>
  [...document.querySelectorAll('.sode__mark, .sode-m__mark, .sode-log__title, .sode-panels__title')].find((e) => e.getBoundingClientRect().width > 0);
const calmMotion = matchMedia('(prefers-reduced-motion: reduce)');
export const motionMs = () => (calmMotion.matches ? 0 : parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--sode-lock-motion')) || 0);

// Every run starts from the brand at rest with its own keyframes, so it restarts reliably. A CSS transition
// measured during the last run does not. Flies only when shouldFly, else the logo fades with the overlay.
export function moveBrand(brand, dir) {
  for (const a of brand.getAnimations()) a.cancel();
  const ms = motionMs();
  if (!ms || !brand.animate) return;
  const from = logo()?.getBoundingClientRect();
  const fly = !logo()?.classList.contains('sode-log__title') && shouldFly(from, window.scrollY, window.innerHeight); // /log's heading is text: it fades, not scales
  const t = fly ? liftFrom(from, brand.getBoundingClientRect()) : { x: 0, y: 0, scale: 1 };
  brand.animate(brandFrames(dir, fly, t), {
    duration: dir === 'in' ? ms : ms * OUT_FACTOR,
    easing: dir === 'in' ? 'cubic-bezier(0.2, 0, 0, 1)' : 'cubic-bezier(0.4, 0, 0.2, 1)',
    fill: dir === 'out' ? 'forwards' : 'none',
  });
}

// A site name wider than the screen scales down to one line (under 1.25rem it wraps instead).
// Phone only.
export function fitBrand(root, brand) {
  brand.style.fontSize = '';
  brand.style.whiteSpace = '';
  if (layout() !== 'phone') return;
  const cs = getComputedStyle(root);
  const room = root.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
  brand.style.whiteSpace = 'nowrap';
  const w = brand.getBoundingClientRect().width;
  if (w <= room) return;
  const size = (parseFloat(getComputedStyle(brand).fontSize) * (room - 1)) / w;
  const min = 1.25 * parseFloat(getComputedStyle(document.documentElement).fontSize);
  if (size >= min) brand.style.fontSize = `${size}px`;
  else {
    brand.style.whiteSpace = '';
    brand.style.fontSize = `${min}px`;
  }
}
