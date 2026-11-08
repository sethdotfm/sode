// PROTOTYPE: stub router and panel engine, in memory only.
export const SOURCES = ['CAM 1','CAM 2','CAM 3','CAM 4','PGM','PVW','CLEAN','GFX 1','GFX 2','REPLAY 1','REPLAY 2','VT 1','VT 2','SAT 1','SAT 2','BARS'].map((name, n) => ({ n, name }))
export const DESTS = ['MON A','MON B','MON C','MON D','REC 1','REC 2','AIR','MV 1','MV 2','STREAM'].map((name, n) => ({ n, name }))
export const LEVELS = [1, 2, 3]
export const PRESETS = [
  { id: 'p1', name: 'Show open', routes: [[0, 0], [1, 1], [4, 6]] },
  { id: 'p2', name: 'Replay', routes: [[9, 0], [10, 1]] },
  { id: 'p3', name: 'Bars all', routes: [[15, 0], [15, 1], [15, 2], [15, 3]] },
]
// Level sets: a take sends the intersection of the Source's and Destination's sets.
const destLevels = { 5: [1, 2], 9: [3] }
const srcLevels = { 12: [3], 13: [1] }
export const levelsOf = (kind, n) => (kind === 'dst' ? destLevels : srcLevels)[n] || LEVELS
export const common = (s, d) => levelsOf('src', s).filter((l) => levelsOf('dst', d).includes(l))
export const PROTECTED = new Set([6])

export function createPanel(opts = {}) {
  const st = {
    mode: opts.mode || 'take', // 'take' | 'instant'
    multi: opts.multi ?? true,
    guard: opts.guard || 'none', // 'none' | 'hold' | 'double'
    idleClear: true,
    selDsts: [],
    selSrc: null,
    plan: null, // preset plan, {name, routes}
    tally: Object.fromEntries(DESTS.map((d, i) => [d.n, (i * 3) % SOURCES.length])),
    flash: {}, // dst -> {result, at}
    error: null,
    last: null,
    log: [],
    idleAt: 0,
  }
  const subs = new Set()
  const emit = () => subs.forEach((f) => f(st))
  const touch = () => (st.idleAt = Date.now())
  const err = (m) => {
    st.error = m
    setTimeout(() => { if (st.error === m) { st.error = null; emit() } }, 3000)
  }
  const routesNow = () => {
    if (st.plan) return st.plan.routes.map(([s, d]) => ({ src: s, dst: d }))
    if (st.selSrc == null) return []
    return st.selDsts.map((d) => ({ src: st.selSrc, dst: d }))
  }
  const api = {
    st,
    subscribe(f) { subs.add(f); f(st) },
    armed() {
      const r = routesNow()
      return st.mode === 'take' && r.length > 0 && r.every((x) => !PROTECTED.has(x.dst) && common(x.src, x.dst).length)
    },
    pickDst(n) {
      touch()
      if (PROTECTED.has(n)) return err(DESTS[n].name + ' is protected')
      st.plan = null
      const i = st.selDsts.indexOf(n)
      if (i >= 0) st.selDsts.splice(i, 1)
      else if (st.multi) st.selDsts.push(n)
      else st.selDsts = [n]
      emit()
    },
    pickSrc(n) {
      touch()
      if (!st.selDsts.length) return err('Pick a destination first')
      const bad = st.selDsts.find((d) => !common(n, d).length)
      if (bad != null) return err(`No common levels: ${SOURCES[n].name} to ${DESTS[bad].name}`)
      st.selSrc = n
      st.plan = null
      if (st.mode === 'instant') return api.fire()
      emit()
    },
    pickPreset(id) {
      touch()
      const p = PRESETS.find((x) => x.id === id)
      st.plan = p
      st.selSrc = null
      st.selDsts = []
      const bad = p.routes.find(([s, d]) => PROTECTED.has(d) || !common(s, d).length)
      if (bad) { st.plan = null; return err('Preset has a refused route') }
      if (st.mode === 'instant') return api.fire()
      emit()
    },
    clear() { touch(); st.selDsts = []; st.selSrc = null; st.plan = null; st.error = null; emit() },
    // Take cell press; a guard is handled by the view (hold ring, double press).
    take() { if (api.armed()) api.fire() },
    fire() {
      const routes = routesNow()
      if (!routes.length) return
      const now = Date.now()
      for (const r of routes) {
        const already = st.tally[r.dst] === r.src
        st.tally[r.dst] = r.src
        st.flash[r.dst] = { result: already ? 'already' : 'connected', at: now }
      }
      st.last = { routes, at: now }
      st.log.unshift(routes.map((r) => `${SOURCES[r.src].name} > ${DESTS[r.dst].name} [${common(r.src, r.dst).join(',')}]`).join(', '))
      st.selSrc = null
      st.plan = null
      if (!st.multi) st.selDsts = []
      emit()
      setTimeout(emit, 1600)
    },
    set(k, v) { st[k] = v; api.clear() },
  }
  setInterval(() => {
    if (st.idleClear && st.idleAt && Date.now() - st.idleAt > 30000 && (st.selDsts.length || st.selSrc != null || st.plan)) api.clear()
  }, 1000)
  return api
}
export const flashOf = (st, dst) => {
  const f = st.flash[dst]
  return f && Date.now() - f.at < 1500 ? f.result : null
}
export const ICON = { connected: '✓', already: '＝', failed: '✕' }
