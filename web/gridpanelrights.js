// Rights, protection and other sources on a Grid panel. Pure: no DOM, no clock, no network.
// env is envFrom's (panelrules.js); names is namesFrom.
import { rightsAllow } from './panelrules.js';
import { acrossRouters } from './routerids.js';

// The sources dst has now that the grid does not show, in level order, each one time. shown is a Set of router numbers.
export function otherSources(env, dst, shown) {
  const row = env.tally.get(dst);
  if (!row) return [];
  const out = [];
  for (const [, s] of [...row].sort(([a], [b]) => a - b)) {
    if (s >= 0 && !shown.has(s) && !out.includes(s)) out.push(s);
  }
  return out;
}

// Any protected level protects the whole destination, as on a Buttons panel. A source and a destination on two routers
// need a tie line, which Sode does not have yet. null when the press can stage.
export const pressRefusal = ({ src, dst }, env) =>
  env.protectedDsts.has(dst) ? { code: 'protected', dst } : acrossRouters(src, dst) ? { code: 'across-routers', src, dst } : null;

// The pairs a press can stage: a Diagonal skips protected rows. err only when every target is protected.
export function stageable(pairs, env) {
  const kept = pairs.filter((p) => !pressRefusal(p, env));
  return { pairs: kept, err: pairs.length && !kept.length ? pressRefusal(pairs[0], env) : null };
}

// The banner words for the rows a press or a line skipped: protected rows with up to three names, then "+N", and
// routes across routers. '' when none.
export function skippedText(pairs, names) {
  const across = pairs.filter((x) => acrossRouters(x.src, x.dst)).length;
  const dsts = [...new Set(pairs.filter((x) => !acrossRouters(x.src, x.dst)).map((x) => x.dst))];
  const out = [];
  if (dsts.length) {
    const shown = dsts.slice(0, 3).map((d) => names.destination(d));
    const more = dsts.length > 3 ? ` +${dsts.length - 3}` : '';
    out.push(`Skipped ${dsts.length === 1 ? 'a protected destination' : 'protected destinations'}: ${shown.join(', ')}${more}`);
  }
  if (across) out.push(`Skipped ${across === 1 ? 'a route' : `${across} routes`} across routers: Sode has no tie lines yet`);
  return out.join('. ');
}

export const refusalText = (err, names) =>
  err?.code === 'protected' ? `${names.destination(err.dst)} is protected` : err?.code === 'across-routers' ? acrossText(err, names) : '';

// Keep in step with errorText in panelrules.js.
export const acrossText = (err, names) => `${names.source(err.src)} and ${names.destination(err.dst)} are on two routers. Sode has no tie lines yet.`;

// Why a staged entry { dst, level, src } no longer applies, or ''. world is { env, rights, names (GET /names) }.
// A read that has not come yet leaves nothing out, so a slow first poll never drops staged work.
function whyOut(x, { env, rights, names }) {
  if (!rightsAllow(rights, 'dsts', x.dst) || !rightsAllow(rights, 'sources', x.src) || !rightsAllow(rights, 'levels', x.level, env?.router)) return 'rights';
  const has = (list, n) => !Array.isArray(list) || list.some((y) => y.n === n);
  if (!has(names?.sources, x.src) || !has(names?.destinations, x.dst)) return 'gone';
  if (env.levels.length && !env.levels.includes(x.level)) return 'gone';
  if (env.protectedDsts.has(x.dst)) return 'protected';
  return '';
}

// The staged routes the confirm leaves out, as { src, dst, levels, why }, in the order first staged.
export function leftOut(st, world) {
  const by = new Map();
  for (const x of st.staged) {
    const why = whyOut(x, world);
    if (!why) continue;
    const k = `${x.dst}:${x.src}:${why}`;
    if (!by.has(k)) by.set(k, { src: x.src, dst: x.dst, levels: [], why });
    by.get(k).levels.push(x.level);
  }
  return [...by.values()].map((r) => ({ ...r, levels: r.levels.sort((a, b) => a - b) }));
}

// st without the entries leftOut names; the same st when all apply.
export function keepLive(st, world) {
  const staged = st.staged.filter((x) => !whyOut(x, world));
  return staged.length === st.staged.length ? st : { ...st, staged };
}

const WHY = {
  protected: (r, names) => `${names.destination(r.dst)} is protected`,
  rights: () => "it is outside this panel's Rights",
  gone: () => 'the router does not report it',
};

export const leftOutLine = (r, names) => `${names.source(r.src)} to ${names.destination(r.dst)}, since ${WHY[r.why](r, names)}`;

export function leftOutText(list, names) {
  if (!list.length) return '';
  if (list.length > 1) return `Left out of the take: ${list.length} staged routes`;
  return `Left out of the take: ${leftOutLine(list[0], names)}`;
}

// The first other source by name, then "+N" for the rest. '' when there is none.
export const otherText = (srcs, names) => (srcs.length ? names.source(srcs[0]) + (srcs.length > 1 ? ` +${srcs.length - 1}` : '') : '');
