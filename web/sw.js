// The service worker: it keeps the app shell, so a main page or panel page that this device opened also opens while
// Sode is down. The page then shows its own Offline state and connects when Sode answers (docs/agents/web-layout.md,
// Service worker). A classic script, not a module, so every browser with service workers runs it. Node tests run it
// in a vm (test/sw.test.mjs).
//
// Sode fills in BUILD and SHELL when it serves this file (internal/app/sw.go). BUILD changes with every change to the
// web files. So a new build is a new worker with its own cache, which the browser installs on the next online load.
//
// Never kept: /api (so /api/v1 and every PIN, token and session key in it), the WebSocket (the hello rides it),
// healthz and this file. Every other request goes to Sode first. The cache answers only when Sode cannot.

const BUILD = '__SODE_BUILD__';
const SHELL = ['__SODE_SHELL__'];
const CACHE = `sode-${BUILD}`;

// path: the request's path below the scope, with no leading slash. 'page' is a page a device opens, 'file' is an app
// shell file, and '' is a request the worker leaves to the browser.
function shellKind(path, method) {
  if (method !== 'GET') return '';
  if (path === '' || /^panels\/?$/.test(path) || /^panels\/[^/]+$/.test(path)) return 'page';
  if (/^(api\/|ws$|healthz$|sw\.js$)/.test(path)) return '';
  if (/^vendor\/[\w./-]+$/.test(path) || /^[\w-]+\.(js|css|svg|png|woff2)$/.test(path)) return 'file';
  return '';
}

// A page is the same HTML for every query (?instance=), so it is kept by its path.
function cacheKey(url, kind) {
  const u = new URL(url);
  return kind === 'page' ? u.origin + u.pathname : u.href;
}

// The kind of a request, or '' when it is outside the scope.
function kindOf(url, method, scope) {
  if (!url.startsWith(scope)) return '';
  return shellKind(new URL(url).pathname.slice(new URL(scope).pathname.length), method);
}

// Sode first. Its answer is kept when it is new (another ETag). With no answer, or a proxy's 5xx while Sode is down,
// the kept copy answers. keep(promise) holds the worker open for a write that the answer does not wait for.
async function networkFirst(req, key, keep = () => {}) {
  const cache = await caches.open(CACHE);
  let res;
  try {
    res = await fetch(req);
  } catch (err) {
    const hit = await cache.match(key, { ignoreVary: true });
    if (hit) return hit;
    throw err;
  }
  if (res.status >= 500) {
    const hit = await cache.match(key, { ignoreVary: true });
    if (hit) return hit;
  }
  if (res.ok && res.type === 'basic') {
    const copy = res.clone();
    keep(
      cache.match(key, { ignoreVary: true }).then((had) => {
        const tag = res.headers.get('ETag');
        if (!had || !tag || had.headers.get('ETag') !== tag) return cache.put(key, copy);
      }),
    );
  }
  return res;
}

const scope = () => self.registration.scope;

self.addEventListener('install', (e) => {
  // A new build takes over at once. A page that started while Sode was down then reloads (swreg.js). A page that
  // started online has its files from Sode already. The hello's build check reloads it when Sode changes.
  e.waitUntil(
    caches
      .open(CACHE)
      .then((c) => c.addAll(SHELL.map((f) => new Request(new URL(f, scope()), { cache: 'no-cache' }))))
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    (async () => {
      for (const name of await caches.keys()) if (name.startsWith('sode-') && name !== CACHE) await caches.delete(name);
      // The page that installed this worker loaded before it, so its own address is kept now.
      const cache = await caches.open(CACHE);
      for (const c of await self.clients.matchAll({ type: 'window', includeUncontrolled: true })) {
        if (kindOf(c.url, 'GET', scope()) === 'page') await cache.add(new Request(cacheKey(c.url, 'page'), { cache: 'no-cache' })).catch(() => {});
      }
      await self.clients.claim();
    })(),
  );
});

self.addEventListener('fetch', (e) => {
  const kind = kindOf(e.request.url, e.request.method, scope());
  if (!kind) return;
  e.respondWith(networkFirst(e.request, cacheKey(e.request.url, kind), (p) => e.waitUntil(p)));
});
