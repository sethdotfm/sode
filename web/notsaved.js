// The Not saved banner for env-only mode (the hello's notSaved). Each layout draws its own copy.

import { S, on } from './core.js';

// Set here, not in index.html: TestPagesLoadNothingFromOutside reads every outside href there as a load.
// Keep in step with the Not saved heading in docs/reference.md.
const FIX = 'https://github.com/sethdotfm/sode/blob/main/docs/reference.md#not-saved';

// The page's own copy is sticky and in the flow, so --sode-down-h leaves it out. The fixed Settings sidebar
// (base.css .sode__toc) adds --sode-notsaved-h, so the banner never covers Settings search.
const page = document.querySelector('[data-notsaved]');
const publish = () => document.documentElement.style.setProperty('--sode-notsaved-h', `${page && !page.hidden ? page.offsetHeight : 0}px`);
if (page && typeof ResizeObserver !== 'undefined') new ResizeObserver(publish).observe(page);

on('hello', () => {
  for (const b of document.querySelectorAll('[data-notsaved]')) b.hidden = !S.notSaved;
  for (const a of document.querySelectorAll('[data-notsaved-fix]')) a.href = FIX;
  publish();
});
