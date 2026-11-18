// /api/v1 answers for Grid panel rules tests. Pass what a test cares about; the rest is a small router.

// GET /names: sources and destinations as [n, name] pairs, router numbers from 0.
export function namesReply({
  sources = [
    [0, 'CAM 1'],
    [1, 'CAM 2'],
    [2, 'VT 1'],
  ],
  destinations = [
    [0, 'MON A'],
    [1, 'MON B'],
  ],
  numberOffset = 1,
} = {}) {
  const list = (pairs) => pairs.map(([n, name]) => ({ n, name }));
  return { numberOffset, sources: list(sources), destinations: list(destinations) };
}

// GET /tally: routes as [dst, level, src] triples, levels as UI numbers from 1.
export function tallyReply({ levels = [1, 2, 3], routes = [], levelNumbering = '1', defaultLevels = '1-3' } = {}) {
  return {
    levels: levels.map((l) => ({ level: l, label: String(l), swatch: `l${l}` })),
    levelNumbering,
    defaultLevels,
    routes: routes.map(([dst, level, src]) => ({ dst, level, src })),
  };
}

// A Grid panel as GET /panels/{id} reads it.
export const gridPanel = (over = {}) => ({
  id: 'p-00000001',
  name: 'Grid',
  type: 'grid',
  sources: { members: 'all' },
  destinations: { members: 'all' },
  pages: [],
  settings: {},
  ...over,
});
