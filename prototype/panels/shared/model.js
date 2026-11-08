// PROTOTYPE: stub router and panel engine, in memory only.
export const SOURCES = ['CAM 1','CAM 2','CAM 3','CAM 4','PGM','PVW','CLEAN','GFX 1','GFX 2','REPLAY 1','REPLAY 2','VT 1','VT 2','SAT 1','SAT 2','BARS'].map((name, n) => ({ n, name }))
export const DESTS = ['MON A','MON B','MON C','MON D','REC 1','REC 2','AIR','MV 1','MV 2','STREAM'].map((name, n) => ({ n, name }))
export const LEVELS = [1, 2, 3]
export const PRESETS = [
  { id: 'p1', name: 'Show open', routes: [[5, 0], [6, 1], [7, 4]] },
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
    multi: opts.multi ?? true, // false: one staged change at a time
    guard: opts.guard || 'none',
    idleClear: true,
    view: null, // destination whose current/pending source the grid shows
    staged: new Map(), // dst -> src, applied together by Take
    tally: Object.fromEntries(DESTS.map((d, i) => [d.n, (i * 3) % SOURCES.length])),
    flash: {},
    error: null,
    log: [],
    idleAt: 0,
    presetName: null,
  }
  const subs = new Set()
  const emit = () => subs.forEach((f) => f(st))
  const touch = () => (st.idleAt = Date.now())
  const err = (m) => {
    st.error = m
    emit()
    setTimeout(() => { if (st.error === m) { st.error = null; emit() } }, 3000)
  }
  const api = {
    st,
    subscribe(f) { subs.add(f); f(st) },
    armed() {
      return st.mode === 'take' && st.staged.size > 0 && [...st.staged].every(([d, s]) => !PROTECTED.has(d) && common(s, d).length)
    },
    pickDst(n) {
      touch()
      if (PROTECTED.has(n)) return err(DESTS[n].name + ' is protected')
      st.presetName = null
      if (st.view === n && st.staged.has(n)) st.staged.delete(n)
      if (!st.multi) st.staged.clear()
      st.view = n
      emit()
    },
    pickSrc(n) {
      touch()
      if (st.view == null) return err('Pick a destination first')
      if (!common(n, st.view).length) return err(`No common levels: ${SOURCES[n].name} to ${DESTS[st.view].name}`)
      st.presetName = null
      if (!st.multi) st.staged.clear()
      if (n === st.tally[st.view]) st.staged.delete(st.view)
      else st.staged.set(st.view, n)
      if (st.mode === 'instant') return api.fire()
      emit()
    },
    pickPreset(id) {
      touch()
      const p = PRESETS.find((x) => x.id === id)
      const bad = p.routes.find(([s, d]) => PROTECTED.has(d) || !common(s, d).length)
      if (bad) return err('Preset has a refused route')
      st.staged = new Map(p.routes.map(([s, d]) => [d, s]))
      st.presetName = p.name
      if (!st.staged.has(st.view)) st.view = p.routes[0][1]
      if (st.mode === 'instant') return api.fire()
      emit()
    },
    clear() { touch(); st.staged.clear(); st.view = null; st.presetName = null; st.error = null; emit() },
    take() { if (api.armed()) api.fire() },
    fire() {
      if (!st.staged.size) return
      const now = Date.now()
      const done = []
      for (const [d, s] of st.staged) {
        const already = st.tally[d] === s
        st.tally[d] = s
        st.flash[d] = { result: already ? 'already' : 'connected', at: now }
        done.push(`${SOURCES[s].name} > ${DESTS[d].name} [${common(s, d).join(',')}]`)
      }
      st.log.unshift(done.join(', '))
      st.staged.clear()
      st.presetName = null
      emit()
      setTimeout(emit, 1600)
    },
    set(k, v) { st[k] = v; api.clear() },
  }
  setInterval(() => {
    if (st.idleClear && st.idleAt && Date.now() - st.idleAt > 30000 && (st.view != null || st.staged.size)) api.clear()
  }, 1000)
  return api
}
export const flashOf = (st, dst) => {
  const f = st.flash[dst]
  return f && Date.now() - f.at < 1500 ? f.result : null
}
export const ICON = { connected: '✓', already: '＝', failed: '✕' }
// Source cell state for the viewed destination: solid green is what it has now, flashing green is staged.
export const srcState = (st, n) => (st.view == null ? 'idle' : st.tally[st.view] === n ? 'current' : st.staged.get(st.view) === n ? 'pending' : 'idle')
// Destination cell state: orange is the one being viewed; staged ones keep an orange outline.
export const dstState = (st, d) => (d === st.view ? 'view' : st.staged.has(d) ? 'staged' : 'idle')
