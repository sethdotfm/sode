// Wraps a fetcher so repeated GETs send If-None-Match. A 304 comes back as a 200 with the body kept from the last 200,
// so callers (jsonCall) never see it. Tags are per path: one page uses one key and session, so a path has one answer.
// For a path where shared(path) is true, json() gives the same parsed object until the tag moves, so callers must not
// change it. A Grid panel's 2 s tally poll on a 2500 by 750 router parsed 400 KB again on every 304.
export function withEtag(fetcher, shared = () => false) {
  const kept = new Map();
  const parsedOnce = (res, rec) => {
    res.json = async () => (rec.parsed ??= JSON.parse(rec.body));
    return res;
  };
  return async (path, init = {}) => {
    const plain = (init.method || 'GET') === 'GET' && !init.headers?.['If-None-Match'];
    if (!plain) return fetcher(path, init);
    const had = kept.get(path);
    const res = await fetcher(path, had ? { ...init, headers: { ...init.headers, 'If-None-Match': had.tag } } : init);
    if (res.status === 304 && had) {
      const again = new Response(had.body, { status: 200, headers: { 'Content-Type': 'application/json', ETag: had.tag } });
      return shared(path) ? parsedOnce(again, had) : again;
    }
    const tag = res.headers.get('ETag');
    if (!res.ok || !tag) {
      kept.delete(path);
      return res;
    }
    const rec = { tag, body: await res.clone().text() };
    kept.set(path, rec);
    return shared(path) ? parsedOnce(res, rec) : res;
  };
}
