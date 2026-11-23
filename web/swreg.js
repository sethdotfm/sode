// Registers the service worker (sw.js) on the main page and the panel pages. A service worker needs a secure origin:
// localhost, or HTTPS through the user's TLS proxy. Elsewhere this does nothing and the pages work as before.
//
// The reload rule (docs/agents/web-layout.md, Service worker): a new build's worker takes over at once (skipWaiting).
// A page that started while Sode was down has the old build's files. At its first hello it asks for the worker again.
// When a new one takes over, the page reloads, so it never runs with a newer Sode. A page that started online has its
// files from Sode already. The hello's build check (core.js) reloads it when Sode changes.
import { on, once } from './core.js';

export function registerShell() {
  const sw = navigator.serviceWorker;
  if (!sw || !window.isSecureContext) return;
  const reg = sw.register(new URL('sw.js', document.baseURI), { scope: new URL('./', document.baseURI).href, updateViaCache: 'none' }).catch(() => null);
  let hello = false;
  let offlineStart = false;
  const had = !!sw.controller;
  // A socket that closed before the first hello: the page loaded while Sode was down.
  on('wsclose', () => (offlineStart ||= !hello));
  once('hello', () => {
    hello = true;
    if (offlineStart) reg.then((r) => r?.update()).catch(() => {});
  });
  sw.addEventListener('controllerchange', () => {
    if (offlineStart && had) location.reload();
  });
}
