// The clipboard block's text in and out of the system clipboard; the block itself is gridclip.js. navigator.clipboard
// is missing on a plain HTTP address other than localhost (TLS is the user's proxy), so each way has a fallback.
import { typing } from './gridchords.js';

// True when the text reached the clipboard. execCommand still works inside the key's user activation, and its copy
// event lets the page set the text with nothing selected.
export async function writeClip(text) {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {}
  const put = (e) => {
    e.clipboardData.setData('text/plain', text);
    e.preventDefault();
  };
  document.addEventListener('copy', put);
  try {
    return document.execCommand('copy');
  } catch {
    return false;
  } finally {
    document.removeEventListener('copy', put);
  }
}

// Paste reads the browser's own paste event, which asks no permission and works over plain HTTP, so the paste chord
// leaves its key to the browser and calls key(). When no paste event follows in that task (Safari on an element that
// takes no typing, a synthetic key), readText asks instead. onText gets the text, or null when the browser refused.
export function createPasteReader(root, onText) {
  let waiting = 0;
  let seq = 0;
  document.addEventListener('paste', (e) => {
    const at = document.activeElement;
    if (!root.contains(at) || typing(at)) return;
    waiting = 0;
    e.preventDefault();
    onText(e.clipboardData?.getData('text/plain') ?? '');
  });
  return {
    key() {
      const id = ++seq;
      waiting = id;
      setTimeout(async () => {
        if (waiting !== id) return;
        waiting = 0;
        let text = null;
        try {
          text = await navigator.clipboard.readText();
        } catch {}
        onText(text);
      });
    },
  };
}
