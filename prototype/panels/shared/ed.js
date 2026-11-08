// PROTOTYPE: panel editor state and operations. No persistence; "Save" only clears the dirty flag.
import { SOURCES, DESTS, PRESETS } from './model.js'

export const KINDS = [
  { k: 'src', name: 'Source', icon: 'log-in' },
  { k: 'dst', name: 'Destination', icon: 'log-out' },
  { k: 'pre', name: 'Preset', icon: 'bookmark' },
  { k: 'take', name: 'Take', icon: 'zap' },
  { k: 'clr', name: 'Clear', icon: 'eraser' },
  { k: 'cur', name: 'Current Destination', icon: 'eye' },
]
export const kindName = (k) => KINDS.find((x) => x.k === k).name
export const labelOf = (c) => ({ src: () => SOURCES[c.n]?.name, dst: () => DESTS[c.n]?.name, pre: () => PRESETS.find((p) => p.id === c.pid)?.name, take: () => 'Take', clr: () => 'Clear', cur: () => 'Currently' })[c.kind]() || '?'
export const FS = { sm: 0.85, md: 1, lg: 1.3, xl: 1.7 }
export const FS_NAMES = [['', 'Panel default'], ['sm', 'Small'], ['md', 'Normal'], ['lg', 'Large'], ['xl', 'Huge']]
// Vendored Lucide icons that suit sources, destinations and presets.
export const ART_ICONS = 'video camera clapperboard film tv tv-minimal monitor monitor-play projector webcam mic mic-vocal headphones speaker volume-2 audio-lines music podcast radio radio-tower satellite satellite-dish antenna wifi signal cast airplay globe cloud server hard-drive disc router cable plug play pause circle-play square-play circle-dot rewind fast-forward list-video image images layers layout-grid presentation captions type megaphone newspaper user user-round users briefcase mountain sun moon star flag flame trophy house building warehouse car plane map-pin clock timer zap eye scan-line'.split(' ')
export const POOL = { src: SOURCES, dst: DESTS }

// Fuzzy match: query letters in order, with a bonus for runs and word starts. -1 is no match, 0 is an empty query.
export function fuzzy(q, text) {
  q = q.trim().toLowerCase(); if (!q) return 0
  const t = text.toLowerCase(); let s = 0, i = 0, prev = -2
  for (const ch of q) { if (ch === ' ') continue; const j = t.indexOf(ch, i); if (j < 0) return -1; s += (j === prev + 1 ? 3 : 1) + (j === 0 || t[j - 1] === ' ' ? 2 : 0); prev = j; i = j + 1 }
  return s
}

const DEFAULT_CFG = () => ({
  name: 'Studio A', id: 'studio-a', fontSize: 'md', title: true, backdrop: null, mode: 'take', guard: 'none', multi: true, text: false, idle: 30, levels: [1, 2, 3],
  colView: null, colCur: null, colTake: null,
  acts: 'user', rightsCap: 'mine', kiosk: false, auth: 'open', share: 'private', sharedWith: ['Dana'],
})

export function createEditor() {
  const E = { rows: 4, cols: 8, cells: [], anchor: null, head: null, tool: 'select', test: false, uploads: [], trusted: true, rights: 'all', dirty: false, ghost: null, curPanel: 0,
    fill: { kind: 'src', text: '1-8', dir: 'across', keep: false },
    panels: [{ name: 'Studio A', who: 'You', own: true, acts: 'user' }, { name: 'Replay desk', who: 'You', own: true, acts: 'api' }, { name: 'Master control', who: 'Dana', own: false }, { name: 'Edit bay 2', who: 'Priya', own: false }], notice: '', cfg: DEFAULT_CFG(), readonly: false, subs: new Set(), uid: 1 }
  E.on = (f) => (E.subs.add(f), f('init'))
  const emit = (topic, dirty = true) => { if (dirty && topic !== 'sel') E.dirty = true; E.subs.forEach((f) => f(topic)) }
  E.emit = emit
  const say = (m) => { E.notice = m; emit('notice', false) }
  const add = (cell) => (E.cells.push({ id: E.uid++, rs: 1, cs: 1, lv: null, ...cell }), cell)

  E.cellAt = (r, c) => E.cells.find((x) => r >= x.r && r < x.r + x.rs && c >= x.c && c < x.c + x.cs)
  const nextFree = (kind) => {
    const used = new Set(E.cells.filter((x) => x.kind === kind).map((x) => x.n))
    const pool = POOL[kind]
    return (pool.find((p) => !used.has(p.n)) || pool[0]).n
  }
  const defaults = (kind) => (kind === 'src' || kind === 'dst' ? { n: nextFree(kind) } : kind === 'pre' ? { pid: PRESETS[0].id } : {})

  // Selection is always a rectangle grown to include every cell it touches, so a merged cell is never half selected.
  E.rect = () => {
    if (!E.anchor) return null
    let r1 = Math.min(E.anchor.r, E.head.r), r2 = Math.max(E.anchor.r, E.head.r), c1 = Math.min(E.anchor.c, E.head.c), c2 = Math.max(E.anchor.c, E.head.c)
    for (let grew = true; grew; ) {
      grew = false
      for (const x of E.cells) {
        if (x.r + x.rs - 1 < r1 || x.r > r2 || x.c + x.cs - 1 < c1 || x.c > c2) continue
        if (x.r < r1 || x.c < c1 || x.r + x.rs - 1 > r2 || x.c + x.cs - 1 > c2) { r1 = Math.min(r1, x.r); c1 = Math.min(c1, x.c); r2 = Math.max(r2, x.r + x.rs - 1); c2 = Math.max(c2, x.c + x.cs - 1); grew = true }
      }
    }
    return { r1, c1, r2, c2 }
  }
  E.selCells = () => { const q = E.rect(); return q ? E.cells.filter((x) => !(x.r + x.rs - 1 < q.r1 || x.r > q.r2 || x.c + x.cs - 1 < q.c1 || x.c > q.c2)) : [] }
  E.selSquares = () => { const q = E.rect(); return q ? (q.r2 - q.r1 + 1) * (q.c2 - q.c1 + 1) : 0 }
  E.isSel = (r, c) => { const q = E.rect(); return !!q && r >= q.r1 && r <= q.r2 && c >= q.c1 && c <= q.c2 }
  E.select = (r, c, extend) => { if (extend && E.anchor) E.head = { r, c }; else { E.anchor = { r, c }; E.head = { r, c } } emit('sel') }
  E.extend = (r, c) => { if (E.anchor && (E.head.r !== r || E.head.c !== c)) { E.head = { r, c }; emit('sel') } }
  E.deselect = () => { E.anchor = E.head = null; emit('sel') }
  E.single = () => { const s = E.selCells(); return s.length === 1 && E.selSquares() === s[0].rs * s[0].cs ? s[0] : null }

  E.setKind = (kind) => {
    const q = E.rect(); if (!q) return
    const cells = E.selCells()
    if (cells.length) for (const c of cells) { if (c.kind !== kind) Object.assign(c, { kind, n: undefined, pid: undefined, ...defaults(kind) }) }
    else for (let r = q.r1; r <= q.r2; r++) for (let k = q.c1; k <= q.c2; k++) add({ r, c: k, kind, ...defaults(kind) })
    emit('layout')
  }
  E.paint = (r, c, tool) => {
    const hit = E.cellAt(r, c)
    if (tool === 'erase') { if (hit) E.cells = E.cells.filter((x) => x !== hit) }
    else if (!hit) add({ r, c, kind: tool, ...defaults(tool) })
    else if (hit.kind !== tool) Object.assign(hit, { kind: tool, n: undefined, pid: undefined, ...defaults(tool) })
    emit('layout')
  }
  E.bind = (cell, key, val) => { cell[key] = val; emit('layout') }
  E.levelsOf = (cell, l, on) => { const cur = new Set(cell.lv || E.cfg.levels); on ? cur.add(l) : cur.delete(l); cell.lv = cur.size === E.cfg.levels.length ? null : [...cur].sort(); emit('layout') }
  E.merge = () => {
    const q = E.rect(); const hit = E.selCells()
    if (!q || E.selSquares() < 2) return say('Select two or more squares to merge.')
    const keep = hit[0] || { kind: 'take' }
    E.cells = E.cells.filter((x) => !hit.includes(x))
    add({ r: q.r1, c: q.c1, rs: q.r2 - q.r1 + 1, cs: q.c2 - q.c1 + 1, kind: keep.kind, n: keep.n, pid: keep.pid, lv: keep.lv || null, ...(hit.length ? {} : defaults('take')) })
    if (hit.length > 1) say(`Merged ${hit.length} cells. The first one's settings were kept.`)
    E.anchor = { r: q.r1, c: q.c1 }; E.head = { r: q.r2, c: q.c2 }
    emit('layout')
  }
  E.split = (cell) => {
    if (cell.rs * cell.cs < 2) return
    Object.assign(cell, { rs: 1, cs: 1 })
    say('Split. The extra squares are empty.')
    E.anchor = E.head = { r: cell.r, c: cell.c }
    emit('layout')
  }
  E.remove = () => { const h = E.selCells(); E.cells = E.cells.filter((x) => !h.includes(x)); emit('layout') }
  E.fillEmpty = () => {
    for (let r = 1; r <= E.rows; r++) for (let c = 1; c <= E.cols; c++) if (!E.cellAt(r, c)) add({ r, c, kind: 'src', n: nextFree('src') })
    emit('layout')
  }
  E.resize = (rows, cols) => {
    rows = Math.max(1, Math.min(8, rows)); cols = Math.max(1, Math.min(16, cols))
    if (rows === E.rows && cols === E.cols) return
    let cut = 0
    E.cells = E.cells.filter((x) => (x.r > rows || x.c > cols ? (cut++, false) : true))
    for (const x of E.cells) { x.rs = Math.min(x.rs, rows - x.r + 1); x.cs = Math.min(x.cs, cols - x.c + 1) }
    E.rows = rows; E.cols = cols
    if (E.anchor && (E.anchor.r > rows || E.anchor.c > cols || E.head.r > rows || E.head.c > cols)) E.anchor = E.head = null
    if (cut) say(`Removed ${cut} cell${cut > 1 ? 's' : ''} that no longer fit.`)
    emit('layout')
  }

  // Rights stub: 'all' is full Rights; 'ro' and 'hidden' are limited, with Visibility Read-only (everything listed, outside greyed) or Hidden (outside not listed).
  E.allowed = (kind, key) => E.rights === 'all' || (kind === 'src' ? key < 10 : kind === 'dst' ? key < 8 : ['p1', 'p2'].includes(key))
  E.items = (kind) => (kind === 'pre' ? PRESETS.map((p) => ({ key: p.id, name: p.name, num: null })) : POOL[kind].map((p) => ({ key: p.n, name: p.name, num: p.n + 1 })))
    .map((i) => ({ ...i, ok: E.allowed(kind, i.key) })).filter((i) => E.rights !== 'hidden' || i.ok)
  E.setRights = (v) => { E.rights = v; emit('rights', false) }

  E.canUnmerge = () => E.selCells().some((x) => x.rs * x.cs > 1)
  E.unmerge = () => {
    const m = E.selCells().filter((x) => x.rs * x.cs > 1)
    if (!m.length) return say('Select a merged cell to unmerge.')
    for (const x of m) Object.assign(x, { rs: 1, cs: 1 })
    say('Unmerged. The first square keeps the cell and the others are empty.')
    emit('layout')
  }

  // Fill selection: the selected squares are the target; typed numbers ("1-8, 12") pick what goes in, in reading order.
  const order = (kind) => (kind === 'pre' ? PRESETS.map((p) => p.id) : POOL[kind].map((p) => p.n))
  const parseRuns = (text, max) => {
    const out = [], bad = []
    for (const tok of text.split(/[,\s]+/).filter(Boolean)) {
      const m = tok.match(/^(\d+)(?:-(\d+))?$/)
      if (!m) { bad.push(tok); continue }
      const a = +m[1], z = m[2] ? +m[2] : a
      for (let k = Math.min(a, z); k <= Math.max(a, z); k++) k >= 1 && k <= max ? out.push(k - 1) : bad.push(String(k))
    }
    return { out, bad }
  }
  E.fillPlan = () => {
    const q = E.rect(), F = E.fill
    if (!q) return null
    const ord = order(F.kind), { out, bad } = parseRuns(F.text, ord.length)
    const keys = out.map((i) => ord[i]).filter((k) => E.items(F.kind).some((i) => i.key === k))
    const queue = [...keys], slots = []
    const [o0, o1, i0, i1] = F.dir === 'across' ? [q.r1, q.r2, q.c1, q.c2] : [q.c1, q.c2, q.r1, q.r2]
    for (let o = o0; o <= o1; o++) for (let i = i0; i <= i1; i++) {
      const [r, c] = F.dir === 'across' ? [o, i] : [i, o]
      const hit = E.cellAt(r, c)
      if (hit && (hit.r !== r || hit.c !== c || F.keep)) continue
      if (!queue.length) continue
      slots.push({ r, c, key: queue.shift(), replace: !!hit })
    }
    return { slots, dropped: queue.length, bad, kind: F.kind, picked: keys.length, squares: E.selSquares() }
  }
  E.setFill = (patch) => { Object.assign(E.fill, patch); emit('seq', false) }
  E.previewFill = () => { E.ghost = E.fillPlan(); emit('ghost', false) }
  E.applyFill = () => {
    const pl = E.fillPlan(); if (!pl || !pl.slots.length) return say('Nothing to place. Select squares and type some numbers.')
    for (const s of pl.slots) {
      E.cells = E.cells.filter((x) => x !== E.cellAt(s.r, s.c))
      add({ r: s.r, c: s.c, kind: pl.kind, ...(pl.kind === 'pre' ? { pid: s.key } : { n: s.key }) })
    }
    if (pl.dropped) say(`Filled ${pl.slots.length}. ${pl.dropped} more did not fit in the selection.`)
    E.ghost = null
    emit('layout')
  }

  // Look: per-cell font size and artwork (an icon, or an uploaded image). Uploads live in memory here; the build needs per-panel storage and a size cap.
  E.artOf = (x) => (x.art ? { ...x.art, url: x.art.kind === 'image' ? E.uploads.find((u) => u.id === x.art.ref)?.url : null } : null)
  E.setLook = (cells, patch) => { for (const x of cells) Object.assign(x, patch); emit('layout') }
  E.addUpload = (file) => {
    if (!/^image\/(jpeg|png|gif|svg\+xml)$/.test(file.type)) return say('Use a JPEG, PNG, GIF or SVG file.')
    if (file.size > 1024 * 1024) return say('That file is over 1 MB.')
    const u = { id: 'u' + E.uid++, name: file.name, url: URL.createObjectURL(file) }
    E.uploads.push(u); emit('uploads'); return u
  }

  E.openPanel = (i) => {
    const p = E.panels[i]; E.curPanel = i; E.readonly = !p.own
    E.cfg.name = p.name; E.cfg.acts = p.own ? p.acts || 'user' : 'user'
    E.dirty = false; E.emit('panel', false); E.emit('cfg', false)
  }
  E.newPanel = () => { E.panels.unshift({ name: 'New panel', who: 'You', own: true, acts: 'user' }); E.openPanel(0) }
  E.copyPanel = () => { const p = E.panels[E.curPanel]; E.panels.unshift({ ...p, name: p.name + ' copy', who: 'You', own: true }); E.openPanel(0) }
  E.setCfg = (k, v) => { E.cfg[k] = v; if (k === 'name') { E.panels[E.curPanel].name = v; emit('panel', false) } emit('cfg') }
  E.save = () => { E.dirty = false; say('Saved. Open panels refresh in 10 s.'); emit('save', false) }
  E.deckCells = () => E.cells.map((x) => ({ r: x.r, c: x.c, rs: x.rs, cs: x.cs, kind: x.kind, n: x.n, id: x.pid, fs: x.fs ? FS[x.fs] : null, art: E.artOf(x) }))

  // Starting layout: the approved prototype A grid.
  for (let i = 0; i < 8; i++) add({ r: 1, c: i + 1, kind: 'dst', n: i })
  for (let i = 0; i < 16; i++) add({ r: 2 + (i >> 3), c: (i & 7) + 1, kind: 'src', n: i })
  add({ r: 4, c: 1, cs: 2, kind: 'cur' }); add({ r: 4, c: 3, kind: 'pre', pid: 'p1' }); add({ r: 4, c: 4, kind: 'pre', pid: 'p2' }); add({ r: 4, c: 5, kind: 'clr' }); add({ r: 4, c: 6, cs: 3, kind: 'take' })
  E.dirty = false
  return E
}
