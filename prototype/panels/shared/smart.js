// PROTOTYPE: "smart cursor" over the edit grid. A floating chooser follows the selected square, a grip at the
// corner of a selection extends a run in the direction you drag, and a gesture pad offers Merge, Unmerge,
// Remove and Type for a multi-square selection.
import { KINDS, POOL, labelOf } from './ed.js'
import { PRESETS } from './model.js'
import { picker, icon } from './edview.js'

const val = (c) => (c.kind === 'pre' ? PRESETS.findIndex((p) => p.id === c.pid) : c.n)
const limit = (kind) => (kind === 'pre' ? PRESETS.length : POOL[kind]?.length)
const hits = (x, r, c, rs, cs) => !(x.r + x.rs - 1 < r || x.r > r + rs - 1 || x.c + x.cs - 1 < c || x.c > c + cs - 1)

// Continue the selection's numbering band by band in the dragged direction (to the squares under the pointer).
export function seriesPlan(E, tr, tc) {
  const q = E.rect(), cells = E.selCells()
  if (!q || !cells.length) return null
  const h = q.r2 - q.r1 + 1, w = q.c2 - q.c1 + 1
  const dv = tr > q.r2 ? tr - q.r2 : tr < q.r1 ? q.r1 - tr : 0, dh = tc > q.c2 ? tc - q.c2 : tc < q.c1 ? q.c1 - tc : 0
  if (!dv && !dh) return null
  const vertical = dv / h >= dh / w
  const sign = vertical ? (tr > q.r2 ? 1 : -1) : tc > q.c2 ? 1 : -1
  const bands = Math.ceil(vertical ? dv / h : dh / w)
  const stride = {}
  for (const kind of ['src', 'dst', 'pre']) {
    const arr = cells.filter((c) => c.kind === kind).sort((a, b) => a.r - b.r || a.c - b.c)
    if (!arr.length) continue
    const step = arr.length > 1 ? Math.round((val(arr.at(-1)) - val(arr[0])) / (arr.length - 1)) || 1 : 1
    stride[kind] = arr.length * step
  }
  const slots = []; let dropped = 0
  for (let k = 1; k <= bands; k++) {
    const dr = vertical ? sign * k * h : 0, dc = vertical ? 0 : sign * k * w
    if (cells.some((c) => c.r + dr < 1 || c.c + dc < 1 || c.r + dr + c.rs - 1 > E.rows || c.c + dc + c.cs - 1 > E.cols)) break
    for (const c of cells) {
      const s = { r: c.r + dr, c: c.c + dc, rs: c.rs, cs: c.cs, kind: c.kind, from: c }
      if (stride[c.kind]) {
        const v = val(c) + sign * k * stride[c.kind]
        if (v < 0 || v >= limit(c.kind)) { dropped++; continue }
        s.key = c.kind === 'pre' ? PRESETS[v].id : v
      }
      s.replace = E.cells.some((x) => !cells.includes(x) && hits(x, s.r, s.c, s.rs, s.cs))
      slots.push(s)
    }
  }
  return { slots, dropped }
}

export function smartCursor(host, E, opts = {}) {
  const slim = !!opts.slim
  host.style.position = 'relative'
  const layer = document.createElement('div'); layer.className = 'sc-layer'; host.append(layer)
  const tip = document.createElement('div'); tip.className = 'sc-tip'; tip.hidden = true; document.body.append(tip)
  let down = false, drag = null, popKind = 'src', lastSel = '', typeOpen = false
  const el = (cls, html = '', tag = 'div') => Object.assign(document.createElement(tag), { className: cls, innerHTML: html })

  const geo = () => {
    const g = host.querySelector('.ed-grid'), gr = g.getBoundingClientRect(), hr = host.getBoundingClientRect(), gap = parseFloat(getComputedStyle(g).columnGap) || 0
    const sw = (gr.width - gap * (E.cols - 1)) / E.cols, sh = (gr.height - gap * (E.rows - 1)) / E.rows, ox = gr.left - hr.left, oy = gr.top - hr.top
    return {
      rect: (r1, c1, r2, c2) => ({ x: ox + (c1 - 1) * (sw + gap), y: oy + (r1 - 1) * (sh + gap), w: (c2 - c1 + 1) * sw + (c2 - c1) * gap, h: (r2 - r1 + 1) * sh + (r2 - r1) * gap }),
      at: (x, y) => ({ c: Math.min(E.cols, Math.max(1, Math.floor((x - gr.left) / (sw + gap)) + 1)), r: Math.min(E.rows, Math.max(1, Math.floor((y - gr.top) / (sh + gap)) + 1)) }),
    }
  }

  const apply = (plan) => {
    for (const s of plan.slots) {
      E.dropCells(E.cells.filter((x) => hits(x, s.r, s.c, s.rs, s.cs)))
      E.addCell({ r: s.r, c: s.c, rs: s.rs, cs: s.cs, kind: s.kind, ...(s.kind === 'pre' ? { pid: s.key } : POOL[s.kind] ? { n: s.key } : {}), fs: s.from.fs, art: s.from.art, lv: s.from.lv })
    }
    E.emit('layout')
  }

  const grip = (R) => {
    const g = el('sc-grip'); g.style.left = R.x + R.w + 'px'; g.style.top = R.y + R.h + 'px'; g.title = 'Drag to extend the run'
    g.onpointerdown = (e) => { e.preventDefault(); g.setPointerCapture(e.pointerId); drag = { plan: null, cancel: false }; tip.hidden = false }
    g.onpointermove = (e) => {
      if (!drag) return
      const s = geo().at(e.clientX, e.clientY)
      drag.plan = seriesPlan(E, s.r, s.c); E.ghost = drag.plan; E.emit('ghost', false)
      const p = drag.plan, first = p?.slots[0], lastS = p?.slots.at(-1)
      tip.textContent = p?.slots.length ? `${labelOf({ kind: first.kind, n: first.key, pid: first.key })} to ${labelOf({ kind: lastS.kind, n: lastS.key, pid: lastS.key })}, ${p.slots.length} cell${p.slots.length > 1 ? 's' : ''}${p.dropped ? `, ${p.dropped} past the end` : ''}` : 'Drag across the grid to extend'
      tip.style.left = e.clientX + 16 + 'px'; tip.style.top = e.clientY + 16 + 'px'
    }
    g.onpointerup = () => { const d = drag; drag = null; tip.hidden = true; E.ghost = null; E.emit('ghost', false); d && !d.cancel && d.plan?.slots.length ? apply(d.plan) : render() }
    return g
  }

  const popover = (R, hostRect) => {
    const x = E.single(), q = E.rect(), key = `${q.r1},${q.c1}`
    if (key !== lastSel) { lastSel = key; if (x && (POOL[x.kind] || x.kind === 'pre')) popKind = x.kind }
    const pop = el('sc-pop pb-panel' + (slim ? ' sc-slim' : ''))
    const kindBtn = (k) => slim ? `<button class="pb-btn pb-btn--sm pb-btn--icon${(x ? x.kind : popKind) === k.k && (POOL[k.k] || k.k === 'pre' ? popKind === k.k : true) ? ' pb-btn--selected' : ''}" data-kind="${k.k}" data-pb-tip="${k.name}" aria-label="${k.name}">${icon(k.icon)}</button>` : ''
    if (slim) pop.innerHTML = `<div class="ed-kinds">${KINDS.map(kindBtn).join('')}</div><div data-pick></div>`
    else pop.innerHTML = `<div class="pb-row"><span class="pb-label pb-grow">Square ${q.r1},${q.c1}${x ? ': ' + labelOf(x) : ''}</span><button class="pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm" data-x aria-label="Close">${icon('x')}</button></div>
      <div class="ed-kinds">${KINDS.map((k) => `<button class="pb-btn pb-btn--sm${(x ? x.kind : popKind) === k.k && (POOL[k.k] || k.k === 'pre' ? popKind === k.k : true) ? ' pb-btn--selected' : ''}" data-kind="${k.k}">${icon(k.icon)} ${k.name}</button>`).join('')}</div><div data-pick></div>
      <p class="pb-hint">Enter places it and moves to the next square. Esc closes.</p>`
    pop.querySelector('[data-x]')?.addEventListener('click', () => E.deselect())
    pop.querySelector('.ed-kinds').onclick = (e) => {
      const b = e.target.closest('[data-kind]'); if (!b) return
      if (POOL[b.dataset.kind] || b.dataset.kind === 'pre') { popKind = b.dataset.kind; render() } else { E.chooseAt(b.dataset.kind); E.nextUnbound() || E.advance() }
    }
    const pk = pop.querySelector('[data-pick]')
    if (POOL[popKind] || popKind === 'pre') picker(pk, E, popKind, x && x.kind === popKind ? (popKind === 'pre' ? x.pid : x.n) : null, (v) => { E.chooseAt(popKind, v); E.nextUnbound() || E.advance() })
    layer.append(pop)
    let left = Math.min(R.x, hostRect.width - pop.offsetWidth - 8), top = R.y + R.h + 12
    if (top + pop.offsetHeight > hostRect.height + 120) top = Math.max(0, R.y - pop.offsetHeight - 12)
    pop.style.left = Math.max(0, left) + 'px'; pop.style.top = top + 'px'
    setTimeout(() => pk.querySelector('input')?.focus(), 0)
  }

  // Slim: three icon buttons beside a multi-square selection, no pad and no gesture.
  const bar = (R, hostRect) => {
    const b = el('sc-bar pb-panel')
    b.innerHTML = [['merge', 'Merge', 'merge', true], ['unmerge', 'Unmerge', 'split', E.canUnmerge()], ['remove', 'Remove', 'trash-2', E.selCells().length > 0]].map(([k, t, i, ok]) => `<button class="pb-btn pb-btn--sm pb-btn--icon" data-a="${k}" data-pb-tip="${t}" aria-label="${t}" ${ok ? '' : 'disabled'}>${icon(i)}</button>`).join('')
    b.onclick = (e) => { const a = e.target.closest('[data-a]')?.dataset.a; if (a === 'merge') E.merge(); if (a === 'unmerge') E.unmerge(); if (a === 'remove') E.remove() }
    layer.append(b)
    b.style.left = Math.max(0, Math.min(R.x + R.w - b.offsetWidth, hostRect.width - b.offsetWidth)) + 'px'; b.style.top = R.y + R.h + 12 + 'px'
  }

  const pad = (R, hostRect) => {
    const p = el('sc-pad pb-panel')
    const arms = [['up', 'Merge', 'merge', true], ['left', 'Unmerge', 'split', E.canUnmerge()], ['right', 'Type', 'sliders-horizontal', true], ['down', 'Remove', 'trash-2', E.selCells().length > 0]]
    p.innerHTML = arms.map(([d, t, i, ok]) => `<button class="pb-btn pb-btn--sm sc-arm sc-${d}" data-arm="${d}" ${ok ? '' : 'disabled'}>${icon(i)}<span>${t}</span></button>`).join('') + `<div class="sc-core" title="Press and move toward an action">${icon('move')}</div>` + (typeOpen ? `<div class="ed-kinds sc-types">${KINDS.map((k) => `<button class="pb-btn pb-btn--sm" data-kind="${k.k}">${icon(k.icon)} ${k.name}</button>`).join('')}</div>` : '')
    const run = { up: () => E.merge(), left: () => E.unmerge(), down: () => E.remove(), right: () => { typeOpen = !typeOpen; render() } }
    p.onclick = (e) => { const b = e.target.closest('button'); if (!b) return; if (b.dataset.kind) { typeOpen = false; E.setKind(b.dataset.kind) } else if (b.dataset.arm) run[b.dataset.arm]() }
    const core = p.querySelector('.sc-core'); let from = null, hot = null
    core.onpointerdown = (e) => { core.setPointerCapture(e.pointerId); from = { x: e.clientX, y: e.clientY }; tip.hidden = false }
    core.onpointermove = (e) => {
      if (!from) return
      const dx = e.clientX - from.x, dy = e.clientY - from.y, far = Math.hypot(dx, dy) > 36
      hot = !far ? null : Math.abs(dx) > Math.abs(dy) ? (dx > 0 ? 'right' : 'left') : dy > 0 ? 'down' : 'up'
      p.querySelectorAll('.sc-arm').forEach((b) => b.classList.toggle('is-hot', b.dataset.arm === hot))
      tip.textContent = hot ? arms.find((a) => a[0] === hot)[1] : 'Move toward an action'
      tip.style.left = e.clientX + 16 + 'px'; tip.style.top = e.clientY + 16 + 'px'
    }
    core.onpointerup = () => { from = null; tip.hidden = true; const a = arms.find((x) => x[0] === hot); hot && a[3] ? run[hot]() : p.querySelectorAll('.is-hot').forEach((b) => b.classList.remove('is-hot')) }
    layer.append(p)
    const left = R.x + R.w + 14 + p.offsetWidth > hostRect.width ? R.x - p.offsetWidth - 14 : R.x + R.w + 14
    p.style.left = Math.max(0, left) + 'px'; p.style.top = Math.max(0, R.y + R.h / 2 - 70) + 'px'
  }

  const render = () => {
    layer.replaceChildren()
    const q = E.rect()
    if (!q) lastSel = ''
    if (E.test || down || drag || !q || E.readonly || !host.querySelector('.ed-grid')) return
    const g = geo(), R = g.rect(q.r1, q.c1, q.r2, q.c2), hr = host.getBoundingClientRect()
    if (E.selCells().length) layer.append(grip(R))
    E.selSquares() === 1 ? popover(R, hr) : slim ? bar(R, hr) : pad(R, hr)
  }

  host.addEventListener('pointerdown', (e) => { if (!e.target.closest('.sc-layer')) down = true }, true)
  document.addEventListener('pointerup', () => { if (down) { down = false; setTimeout(render, 0) } })
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') { if (drag) drag.cancel = true; else if (E.rect()) E.deselect() }
    const t = e.target
    if (/^(INPUT|SELECT|TEXTAREA)$/.test(t.tagName) || !E.rect() || E.selSquares() !== 1 && !E.single()) return
    const q = E.rect(), d = { ArrowLeft: [0, -1], ArrowRight: [0, 1], ArrowUp: [-1, 0], ArrowDown: [1, 0] }[e.key]
    if (d) { e.preventDefault(); E.select(Math.min(E.rows, Math.max(1, q.r1 + d[0])), Math.min(E.cols, Math.max(1, q.c1 + d[1]))) }
  })
  window.addEventListener('resize', render)
  E.on((t) => (t === 'sel' || t === 'layout' || t === 'test' || t === 'init' || t === 'rights' || t === 'panel') && render())
}
