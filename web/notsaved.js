// The Not saved banner for env-only mode (the hello's notSaved). Each layout draws its own copy.

import { S, on } from './core.js';

// Set here, not in index.html: TestPagesLoadNothingFromOutside reads every outside href there as a load.
// Keep in step with the Not saved heading in docs/reference.md.
const FIX = 'https://github.com/sethdotfm/sode/blob/main/docs/reference.md#not-saved';

on('hello', () => {
  for (const b of document.querySelectorAll('[data-notsaved]')) b.hidden = !S.notSaved;
  for (const a of document.querySelectorAll('[data-notsaved-fix]')) a.href = FIX;
});
