// PROTOTYPE: renders a 4x8 deck from a cell list; validates that every cell sits inside the grid.
import { SOURCES, DESTS, PRESETS, flashOf, ICON, PROTECTED, srcState, dstState } from './model.js'
export const ROWS = 4, COLS = 8

function check(cells) {
  const used = new Map()
  for (const c of cells) for (let r = c.r; r < c.r + (c.rs || 1); r++) for (let k = c.c; k < c.c + (c.cs || 1); k++) {
    if (r > ROWS || k > COLS) throw new Error(`cell ${c.kind} outside ${ROWS}x${COLS}`)
    const key = r + ',' + k
    if (used.has(key)) throw new Error(`overlap at ${key}`)
    used.set(key, c)
  }
  if (used.size !== ROWS * COLS) console.warn(`grid has ${ROWS * COLS - used.size} empty squares`)
}

export function buildDeck(root, p, cells, colours = {}) {
  check(cells)
  root.className = 'deck'
  const bg = { src: 'var(--c-idle)', dst: 'var(--c-idle)', pre: 'var(--c-pre)', take: 'var(--c-take)', clr: 'var(--c-clr)', cur: 'var(--c-cur)', ...colours }
  let lastTap = 0, raf = 0
  const els = cells.map((c) => {
    const b = document.createElement('button')
    b.className = 'cell ' + c.kind
    b.style.gridArea = `${c.r} / ${c.c} / span ${c.rs || 1} / span ${c.cs || 1}`
    b.style.setProperty('--bg', bg[c.kind])
    if (c.kind === 'take') {
      b.innerHTML = '<span class="lbl"></span><div class="ring"></div>'
      const ring = b.querySelector('.ring')
      const stop = () => { cancelAnimationFrame(raf); ring.style.width = '0' }
      b.onpointerdown = () => {
        if (!p.armed()) return
        if (p.st.guard === 'hold') {
          const t0 = performance.now()
          const tick = () => { const k = Math.min(1, (performance.now() - t0) / 600); ring.style.width = k * 100 + '%'; if (k >= 1) { stop(); p.take() } else raf = requestAnimationFrame(tick) }
          tick()
        } else if (p.st.guard === 'double') { const n = Date.now(); if (n - lastTap < 500) { lastTap = 0; p.take() } else lastTap = n }
        else p.take()
      }
      b.onpointerup = b.onpointerleave = stop
    } else {
      b.onclick = () => ({ dst: () => p.pickDst(c.n), src: () => p.pickSrc(c.n), pre: () => p.pickPreset(c.id), clr: () => p.clear(), cur: () => {} })[c.kind]()
    }
    root.append(b)
    return b
  })
  const name = (n) => SOURCES[n].name
  p.subscribe((st) => cells.forEach((c, i) => {
    const b = els[i]
    if (c.kind === 'dst') {
      const f = flashOf(st, c.n)
      b.className = 'cell dst' + (f ? ' flash-' + f : '')
      b.dataset.s = dstState(st, c.n)
      b.dataset.v = c.n === st.view ? '1' : ''
      b.disabled = PROTECTED.has(c.n)
      b.innerHTML = `${DESTS[c.n].name}${f ? `<span class="ico">${ICON[f]}<span class="txt"> ${f}</span></span>` : ''}<small>${b.disabled ? 'PROTECTED' : name(st.tally[c.n])}</small>${st.staged.has(c.n) ? `<span class="stg">&rarr; ${name(st.staged.get(c.n))}</span>` : ''}`
    } else if (c.kind === 'src') {
      b.textContent = name(c.n)
      b.dataset.s = srcState(st, c.n)
    } else if (c.kind === 'pre') {
      const x = PRESETS.find((q) => q.id === c.id)
      b.textContent = x.name
      b.classList.toggle('sel', st.presetName === x.name)
    } else if (c.kind === 'clr') b.textContent = 'CLEAR'
    else if (c.kind === 'cur') {
      b.innerHTML = st.view != null
        ? `${DESTS[st.view].name}<small>now ${name(st.tally[st.view])}${st.staged.has(st.view) ? ' &rarr; ' + name(st.staged.get(st.view)) : ''}</small><small>${st.staged.size} staged</small>`
        : 'NO DESTINATION'
    } else if (c.kind === 'take') {
      b.querySelector('.lbl').textContent = st.mode === 'instant' ? 'INSTANT' : 'TAKE'
      b.disabled = st.mode === 'instant'
      b.classList.toggle('armed', p.armed())
    }
  }))
}
