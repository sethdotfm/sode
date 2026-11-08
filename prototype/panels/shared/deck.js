// PROTOTYPE: renders a 4x8 deck from a cell list; validates that every cell sits inside the grid.
import { SOURCES, DESTS, PRESETS, flashOf, ICON, PROTECTED, srcState, dstState } from './model.js'
export const ROWS = 4, COLS = 8

function check(cells, ROWS, COLS) {
  const used = new Map()
  for (const c of cells) for (let r = c.r; r < c.r + (c.rs || 1); r++) for (let k = c.c; k < c.c + (c.cs || 1); k++) {
    if (r > ROWS || k > COLS) throw new Error(`cell ${c.kind} outside ${ROWS}x${COLS}`)
    const key = r + ',' + k
    if (used.has(key)) throw new Error(`overlap at ${key}`)
    used.set(key, c)
  }
  if (used.size !== ROWS * COLS) console.warn(`grid has ${ROWS * COLS - used.size} empty squares`)
}

const icon = (id) => `<svg class="pb-icon pb-icon--sm" viewBox="0 0 24 24" aria-hidden="true"><use href="/vendor/patchbay/icons/lucide.svg#${id}"/></svg>`
const RESULT_ICON = { connected: 'check', already: 'equal', failed: 'x' }

export function buildDeck(root, p, cells, ROWS = 4, COLS = 8) {
  check(cells, ROWS, COLS)
  root.className = 'deck'
  root.style.gridTemplate = `repeat(${ROWS}, 1fr) / repeat(${COLS}, 1fr)`
  root.style.aspectRatio = `${COLS} / ${ROWS}`
  const kindClass = {
    src: 'pb-btn pb-xpt',
    dst: 'pb-btn pb-xpt',
    pre: 'pb-btn',
    clr: 'pb-btn pb-btn--outline',
    take: 'pb-btn pb-btn--commit',
    cur: 'pb-panel now',
  }
  let lastTap = 0, raf = 0
  const els = cells.map((c) => {
    const b = document.createElement(c.kind === 'cur' ? 'div' : 'button')
    b.className = `cell ${c.kind} ${kindClass[c.kind]}`
    b.style.gridArea = `${c.r} / ${c.c} / span ${c.rs || 1} / span ${c.cs || 1}`
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
    } else if (c.kind !== 'cur') {
      b.onclick = () => ({ dst: () => p.pickDst(c.n), src: () => p.pickSrc(c.n), pre: () => p.pickPreset(c.id), clr: () => p.clear() })[c.kind]()
    }
    root.append(b)
    return b
  })
  const name = (n) => SOURCES[n].name
  p.subscribe((st) => cells.forEach((c, i) => {
    const b = els[i]
    if (c.kind === 'dst') {
      const f = flashOf(st, c.n)
      b.className = `cell dst ${kindClass.dst}` + (PROTECTED.has(c.n) ? ' pb-xpt--protected' : '') + (f ? ' flash-' + f : '')
      b.dataset.s = dstState(st, c.n)
      b.dataset.v = c.n === st.view ? '1' : ''
      b.innerHTML = `<span>${DESTS[c.n].name}${f ? ` ${icon(RESULT_ICON[f])}<span class="txt"> ${f}</span>` : ''}</span><small>${PROTECTED.has(c.n) ? 'Protected' : name(st.tally[c.n])}</small>${st.staged.has(c.n) ? `<span class="stg">&rarr; ${name(st.staged.get(c.n))}</span>` : ''}`
    } else if (c.kind === 'src') {
      const s = srcState(st, c.n)
      b.dataset.s = s
      b.innerHTML = `<span>${name(c.n)}${s === 'current' ? ` ${icon('check')}` : ''}</span>`
    } else if (c.kind === 'pre') {
      const x = PRESETS.find((q) => q.id === c.id)
      b.textContent = x.name
      b.classList.toggle('pb-btn--selected', st.presetName === x.name)
    } else if (c.kind === 'clr') b.textContent = 'Clear'
    else if (c.kind === 'cur') {
      // Only the source now on the viewed destination, so it stays visible when the destination is off screen.
      b.innerHTML = st.view != null ? `<span class="pb-label">Currently</span><span class="name">${name(st.tally[st.view])}</span>` : '<span class="pb-label">Currently</span><span class="pb-text-muted">&mdash;</span>'
      b.title = st.view != null ? `Now on ${DESTS[st.view].name}` : 'No destination selected'
    } else if (c.kind === 'take') {
      b.querySelector('.lbl').textContent = st.mode === 'instant' ? 'Instant' : 'Take'
      b.disabled = st.mode === 'instant' || !p.armed()
      b.classList.toggle('armed', p.armed())
    }
  }))
}

// Two copies of the same live panel, one per colour scheme, so light and dark can be judged side by side.
export function mountDecks(host, p, cells) {
  host.className = 'schemes'
  for (const scheme of ['dark', 'light']) {
    const w = document.createElement('div')
    w.className = 'scheme'
    w.dataset.scheme = scheme
    const deck = document.createElement('div')
    w.append(deck)
    host.append(w)
    buildDeck(deck, p, cells)
  }
  const apply = (mode) => {
    host.dataset.mode = mode
    if (mode === 'both') document.documentElement.removeAttribute('data-theme')
    else document.documentElement.dataset.theme = mode
  }
  document.addEventListener('scheme', (e) => apply(e.detail))
  apply('both')
}
