// PROTOTYPE: smart menu (variant F). A compact list beside the selection: Merge and Unmerge act on click,
// Type and Find widen on hover or focus. Find searches every category at once and fills the selected group.
import { KINDS, POOL, labelOf, fuzzy } from './ed.js'
import { icon, orderButtons } from './edview.js'
import { seriesPlan } from './smart.js'

const CAT = [['src', 'Sources', 'log-in'], ['dst', 'Destinations', 'log-out'], ['pre', 'Presets', 'bookmark']]
const catIcon = (k) => CAT.find((c) => c[0] === k)[2]
const hits = (x, r, c, rs, cs) => !(x.r + x.rs - 1 < r || x.r > r + rs - 1 || x.c + x.cs - 1 < c || x.c > c + cs - 1)
const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c])

export function smartMenu(host, E) {
  host.style.position = 'relative'
  const layer = document.createElement('div'); layer.className = 'sc-layer'; host.append(layer)
  const tip = document.createElement('div'); tip.className = 'sc-tip'; tip.hidden = true; document.body.append(tip)
  let session = null, pending = null, lastRow = null, down = false, drag = null, open = null, findKind = 'all', q = '', picks = [], active = 0, cur = [], timer = 0, focusNext = null
  const el = (cls, html = '', tag = 'div') => Object.assign(document.createElement(tag), { className: cls, innerHTML: html })
  // Ctrl+J/K (Cmd+J/K on Mac) step a list, as in the rest of Sode; keep in step with keys.js.
  const jk = (e) => (e.ctrlKey !== e.metaKey && !e.altKey && !e.shiftKey && { KeyJ: 1, KeyK: -1 }[e.code]) || 0
  const same = (a, b) => a.kind === b.kind && a.key === b.key

  const geo = () => {
    const g = host.querySelector('.ed-grid'), gr = g.getBoundingClientRect(), hr = host.getBoundingClientRect(), gap = parseFloat(getComputedStyle(g).columnGap) || 0
    const sw = (gr.width - gap * (E.cols - 1)) / E.cols, sh = (gr.height - gap * (E.rows - 1)) / E.rows, ox = gr.left - hr.left, oy = gr.top - hr.top
    return {
      rect: (r1, c1, r2, c2) => ({ x: ox + (c1 - 1) * (sw + gap), y: oy + (r1 - 1) * (sh + gap), w: (c2 - c1 + 1) * sw + (c2 - c1) * gap, h: (r2 - r1 + 1) * sh + (r2 - r1) * gap }),
      at: (x, y) => ({ c: Math.min(E.cols, Math.max(1, Math.floor((x - gr.left) / (sw + gap)) + 1)), r: Math.min(E.rows, Math.max(1, Math.floor((y - gr.top) / (sh + gap)) + 1)) }),
    }
  }

  // Search every category at once; the filter chips narrow it to one.
  const results = () => {
    const all = CAT.flatMap(([k]) => E.items(k).map((i) => ({ ...i, kind: k })))
    const list = all.filter((i) => findKind === 'all' || i.kind === findKind).map((i) => ({ i, s: fuzzy(q, `${i.num ?? ''} ${i.name}`) })).filter((x) => x.s >= 0)
    if (q) list.sort((a, b) => b.s - a.s)
    return list.map((x) => x.i).slice(0, 80)
  }
  const targets = () => {
    const r = E.rect(); if (!r) return []
    return E.squareOrder(r, E.fill.dir).map(([rr, cc]) => ({ r: rr, c: cc, hit: E.cellAt(rr, cc) })).filter((t) => !t.hit || (t.hit.r === t.r && t.hit.c === t.c))
  }
  const cascade = (it) => { const list = E.items(it.kind); return list.slice(list.findIndex((x) => x.key === it.key)).map((x) => ({ kind: it.kind, key: x.key })) }
  const plan = (items) => ({ slots: targets().slice(0, items.length).map((t, i) => ({ r: t.r, c: t.c, rs: t.hit?.rs || 1, cs: t.hit?.cs || 1, kind: items[i].kind, key: items[i].key, replace: false })), dropped: 0 })
  const preview = (items) => { E.ghost = items ? plan(items) : null; E.emit('ghost', false) }
  const fillWith = (items, kbd) => {
    const single = E.selSquares() === 1
    for (const s of plan(items).slots) {
      const hit = E.cellAt(s.r, s.c); if (hit) E.dropCells([hit])
      E.addCell({ r: s.r, c: s.c, rs: s.rs, cs: s.cs, kind: s.kind, ...(s.kind === 'pre' ? { pid: s.key } : { n: s.key }), fs: hit?.fs })
    }
    E.ghost = null; picks = []; active = 0
    // A click on a single square keeps the menu where it is and moves the highlight to the next row, so clicking the same spot fills the next square.
    if (single && !kbd) { session = session || { R: E.rect() }; open = 'find' } else { q = ''; session = null; open = single && kbd ? 'find' : null }
    focusNext = open && kbd ? 'input' : null
    E.emit('layout')
    if (single) E.nextUnbound() || E.advance()
  }

  const drawRows = (fly) => {
    cur = results(); active = Math.max(0, Math.min(active, cur.length - 1))
    fly.querySelector('.sm-results').innerHTML = cur.map((i, k) => {
      const on = picks.some((p) => same(p, i))
      return `<div class="sm-row${k === active ? ' is-active' : ''}${on ? ' is-on' : ''}${i.ok ? '' : ' is-out'}" data-k="${k}"><input type="checkbox" class="pb-check__input" data-pick aria-label="Pick" ${on ? 'checked' : ''}><button class="sm-main" data-main>${icon(catIcon(i.kind))}<span class="pb-num">${i.num ?? ''}</span><span>${esc(i.name)}</span></button></div>`
    }).join('') || '<p class="pb-text-muted">No match.</p>'
    fly.querySelectorAll('[data-fk]').forEach((b) => b.classList.toggle('pb-btn--selected', b.dataset.fk === findKind))
    fly.querySelector('[data-foot]').innerHTML = picks.length ? `<button class="pb-btn pb-btn--sm pb-btn--commit" data-fillpicks>Fill with ${picks.length} picked</button>` : '<span class="pb-hint">Enter or a click fills from the highlighted item onward. Shift-click or Shift+arrows pick your own. Ctrl/Cmd+J/K step the list.</span>'
    const box = fly.querySelector('.sm-results')
    if (pending) { active = Math.min(pending.idx, cur.length - 1); fly.querySelectorAll('.sm-row').forEach((r, k) => r.classList.toggle('is-active', k === active)); const row = fly.querySelector('.sm-row.is-active'); if (row) box.scrollTop = row.offsetTop - pending.off; pending = null }
    else fly.querySelector('.sm-row.is-active')?.scrollIntoView({ block: 'nearest' })
  }

  const drawFly = (wrap, fly, place) => {
    fly.hidden = !open; wrap.classList.toggle('is-open', !!open)
    if (open === 'type') {
      const n = E.selSquares()
      fly.className = 'sm-fly sm-types'
      fly.innerHTML = `<div class="pb-label">Type for ${n} square${n > 1 ? 's' : ''}</div>` + KINDS.map((k) => `<button class="pb-btn pb-btn--sm sm-kind" data-kind="${k.k}">${icon(k.icon)} ${k.name}</button>`).join('') + `<button class="pb-btn pb-btn--sm pb-btn--danger sm-kind" data-kind="empty">${icon('trash-2')} Empty</button>`
    } else if (open === 'find') {
      fly.className = 'sm-fly sm-finder'
      if (!fly.querySelector('.sm-results')) {
        fly.innerHTML = `<div class="sm-filters">${[['all', 'All', 'layout-grid'], ...CAT.map(([k, t, i]) => [k, t, i])].map(([k, t, i]) => `<button class="pb-btn pb-btn--sm" data-fk="${k}">${icon(i)} ${t}</button>`).join('')}</div>
          <input class="pb-input" placeholder="Find sources, destinations, presets" aria-label="Find">
          <div class="sm-results"></div><div class="sm-order"><span class="pb-hint">Fill order</span>${orderButtons(E.fill.dir)}</div><div data-foot></div>`
        fly.querySelector('input').value = q
      }
      drawRows(fly)
    } else { fly.className = 'sm-fly'; fly.innerHTML = '' }
    place()
  }

  const menu = (R, hr) => {
    const rem = parseFloat(getComputedStyle(document.documentElement).fontSize), need = 8 * rem + 22 * rem, n = E.selSquares()
    const mode = hr.width - (R.x + R.w) - 22 >= need ? 'right' : R.x - 22 >= need ? 'left' : 'below'
    const items = [['merge', 'Merge', 'merge', n > 1, false], ['unmerge', 'Unmerge', 'split', E.canUnmerge(), false], ['type', 'Type', 'sliders-horizontal', true, true], ['find', 'Find', 'search', true, true]]
    const wrap = el('sm sm-' + mode)
    wrap.innerHTML = `<div class="sm-bar" role="menu">${items.map(([id, t, i, ok, exp]) => `<button class="pb-btn pb-btn--sm sm-item${open === id ? ' pb-btn--selected' : ''}" role="menuitem" data-id="${id}" data-exp="${exp ? 1 : ''}" ${ok ? '' : 'disabled'}>${icon(i)}<span>${t}</span>${exp ? icon(mode === 'below' ? 'chevron-down' : 'chevron-right') : ''}</button>`).join('')}</div><div class="sm-fly" hidden></div>`
    const fly = wrap.querySelector('.sm-fly')
    layer.append(wrap)
    const place = () => {
      const w = wrap.offsetWidth, h = wrap.offsetHeight
      const x = mode === 'right' ? R.x + R.w + 22 : mode === 'left' ? R.x - 22 - w : Math.max(0, Math.min(R.x, hr.width - w))
      const y = mode === 'below' ? R.y + R.h + 12 : Math.max(0, R.y)
      wrap.style.left = x + 'px'; wrap.style.top = y + 'px'
    }
    const setOpen = (v) => { if (open === v) return; open = v; if (v !== 'find') fly.innerHTML = ''; wrap.querySelectorAll('.sm-item').forEach((b) => b.classList.toggle('pb-btn--selected', b.dataset.id === open)); drawFly(wrap, fly, place) }
    wrap.onmouseenter = () => clearTimeout(timer)
    wrap.onmouseleave = () => { clearTimeout(timer); timer = setTimeout(() => { if (!wrap.contains(document.activeElement)) { setOpen(null); preview(null) } }, 350) }
    wrap.querySelector('.sm-bar').onclick = (e) => {
      const b = e.target.closest('.sm-item'); if (!b || b.disabled) return
      if (b.dataset.id === 'merge') E.merge(); else if (b.dataset.id === 'unmerge') E.unmerge(); else setOpen(open === b.dataset.id ? null : b.dataset.id)
    }
    const hover = (e) => { const b = e.target.closest('.sm-item'); if (!b || b.disabled) return; setOpen(b.dataset.exp ? b.dataset.id : null) }
    wrap.querySelector('.sm-bar').onmouseover = hover
    wrap.querySelector('.sm-bar').onfocusin = hover
    fly.onclick = (e) => {
      const t = e.target.closest('button'); if (!t) return
      if (t.dataset.kind) { open = null; t.dataset.kind === 'empty' ? E.remove() : E.setKind(t.dataset.kind); return }
      if (t.dataset.fk) { findKind = t.dataset.fk; active = 0; drawRows(fly); return }
      if (t.dataset.dir) { E.setFill({ dir: t.dataset.dir }); fly.querySelector('.sm-order').innerHTML = `<span class="pb-hint">Fill order</span>${orderButtons(E.fill.dir)}`; return }
      if ('fillpicks' in t.dataset) return fillWith(picks)
      if ('main' in t.dataset) {
        const row = t.closest('.sm-row'), k = +row.dataset.k
        if (e.shiftKey) {
          const a = lastRow == null ? k : lastRow
          for (let i = Math.min(a, k); i <= Math.max(a, k); i++) if (!picks.some((p) => same(p, cur[i]))) picks.push({ kind: cur[i].kind, key: cur[i].key })
          lastRow = k; drawRows(fly); return preview(picks)
        }
        lastRow = k
        if (E.selSquares() === 1) pending = { idx: k + 1, off: row.offsetTop - row.parentElement.scrollTop }
        fillWith(cascade(cur[k]))
      }
    }
    fly.onchange = (e) => {
      if (!('pick' in e.target.dataset)) return
      const i = cur[+e.target.closest('.sm-row').dataset.k]
      picks = picks.some((p) => same(p, i)) ? picks.filter((p) => !same(p, i)) : [...picks, { kind: i.kind, key: i.key }]
      drawRows(fly); preview(picks.length ? picks : null)
    }
    fly.onmouseover = (e) => { const r = e.target.closest('.sm-row'); if (r && cur[+r.dataset.k]) preview(picks.length ? picks : cascade(cur[+r.dataset.k])) }
    fly.onmouseleave = () => preview(null)
    fly.oninput = (e) => { if (e.target.tagName === 'INPUT' && e.target.type !== 'checkbox') { q = e.target.value; active = 0; drawRows(fly); cur[0] && preview(cascade(cur[0])) } }
    fly.onkeydown = (e) => {
      const t = e.target
      if (t.tagName === 'INPUT' && t.type !== 'checkbox') {
        const step = e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : jk(e)
        if (step) {
          e.preventDefault()
          // Shift+arrows pick each row they pass, like a list multi-select.
          if (e.shiftKey && cur[active] && !picks.some((p) => same(p, cur[active]))) picks.push({ kind: cur[active].kind, key: cur[active].key })
          active = Math.max(0, Math.min(cur.length - 1, active + step))
          if (e.shiftKey && cur[active] && !picks.some((p) => same(p, cur[active]))) picks.push({ kind: cur[active].kind, key: cur[active].key })
          drawRows(fly); preview(picks.length ? picks : cur[active] ? cascade(cur[active]) : null)
        }
        if (e.key === 'Enter' && (cur[active] || picks.length)) { e.preventDefault(); fillWith(picks.length ? picks : cascade(cur[active]), true) }
        if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); if (picks.length) { picks = []; drawRows(fly); preview(null) } else wrap.querySelector('[data-id="find"]').focus() }
      } else if (t.classList.contains('sm-kind')) {
        const b = [...fly.querySelectorAll('.sm-kind')], i = b.indexOf(t)
        if (e.key === 'ArrowDown' || jk(e) === 1) { b[(i + 1) % b.length].focus(); e.preventDefault() }
        if (e.key === 'ArrowUp' || jk(e) === -1) { b[(i + b.length - 1) % b.length].focus(); e.preventDefault() }
        if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); wrap.querySelector('[data-id="type"]').focus() }
      }
    }
    if (open) drawFly(wrap, fly, place); else place()
    return wrap
  }

  const grip = (R) => {
    const g = el('sc-grip'); g.style.left = R.x + R.w + 'px'; g.style.top = R.y + R.h + 'px'; g.title = 'Drag to extend the run'
    g.onpointerdown = (e) => { e.preventDefault(); g.setPointerCapture(e.pointerId); drag = { plan: null, cancel: false }; tip.hidden = false }
    g.onpointermove = (e) => {
      if (!drag) return
      const s = geo().at(e.clientX, e.clientY)
      drag.plan = seriesPlan(E, s.r, s.c); E.ghost = drag.plan; E.emit('ghost', false)
      const p = drag.plan, a = p?.slots[0], z = p?.slots.at(-1)
      tip.textContent = p?.slots.length ? `${labelOf({ kind: a.kind, n: a.key, pid: a.key })} to ${labelOf({ kind: z.kind, n: z.key, pid: z.key })}, ${p.slots.length} cells` : 'Drag across the grid to extend'
      tip.style.left = e.clientX + 16 + 'px'; tip.style.top = e.clientY + 16 + 'px'
    }
    g.onpointerup = () => {
      const d = drag; drag = null; tip.hidden = true; E.ghost = null; E.emit('ghost', false)
      if (d && !d.cancel && d.plan?.slots.length) {
        for (const s of d.plan.slots) {
          E.dropCells(E.cells.filter((x) => hits(x, s.r, s.c, s.rs, s.cs)))
          E.addCell({ r: s.r, c: s.c, rs: s.rs, cs: s.cs, kind: s.kind, ...(s.kind === 'pre' ? { pid: s.key } : POOL[s.kind] ? { n: s.key } : {}), fs: s.from.fs, art: s.from.art, lv: s.from.lv })
        }
        E.emit('layout')
      } else render()
    }
    return g
  }

  const render = () => {
    layer.replaceChildren()
    const sel = E.rect()
    if (!sel) { open = null; picks = []; q = '' }
    if (E.test || down || drag || !sel || E.readonly || !host.querySelector('.ed-grid')) return
    const g = geo(), R = g.rect(sel.r1, sel.c1, sel.r2, sel.c2), P = session ? g.rect(session.R.r1, session.R.c1, session.R.r2, session.R.c2) : R
    if (E.selCells().length) layer.append(grip(R))
    menu(P, host.getBoundingClientRect())
    if (focusNext === 'input') { focusNext = null; setTimeout(() => { const i = layer.querySelector('.sm-finder input'); i && (i.focus(), i.setSelectionRange(i.value.length, i.value.length)) }, 0) }
  }

  host.addEventListener('pointerdown', (e) => { if (!e.target.closest('.sc-layer')) { down = true; session = null } }, true)
  document.addEventListener('pointerup', () => { if (down) { down = false; setTimeout(render, 0) } })
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && drag) { drag.cancel = true; return }
    const wrap = layer.querySelector('.sm'), sel = E.rect(); if (!wrap || !sel) return
    const t = e.target, inWrap = wrap.contains(t), typing = /^(INPUT|SELECT|TEXTAREA)$/.test(t.tagName)
    const printable = e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey
    const seed = (ch) => { open = 'find'; q = ch; active = 0; focusNext = 'input'; render() }
    if (!inWrap) {
      if (typing) return
      session = null
      if (e.key === 'Escape') return E.deselect()
      if (e.key === 'Tab' && !e.shiftKey) { e.preventDefault(); return wrap.querySelector('.sm-item:not([disabled])')?.focus() }
      if (e.key === 'Delete' || e.key === 'Backspace') { e.preventDefault(); return E.remove() }
      if (e.key === 'Enter') { e.preventDefault(); open = 'find'; focusNext = 'input'; return render() }
      if (printable) { e.preventDefault(); return seed(e.key) }
      const d = { ArrowLeft: [0, -1], ArrowRight: [0, 1], ArrowUp: [-1, 0], ArrowDown: [1, 0] }[e.key]
      if (d) {
        e.preventDefault()
        const h = E.head, r = Math.min(E.rows, Math.max(1, h.r + d[0])), c = Math.min(E.cols, Math.max(1, h.c + d[1]))
        E.select(r, c, e.shiftKey)
      }
      return
    }
    // Inside the menu: arrows walk the items, the arrow toward the flyout enters it, Esc returns to the grid.
    if (t.classList.contains('sm-item')) {
      const mode = wrap.classList.contains('sm-right') ? 'right' : wrap.classList.contains('sm-left') ? 'left' : 'below'
      const [prev, next, into] = mode === 'below' ? ['ArrowLeft', 'ArrowRight', 'ArrowDown'] : ['ArrowUp', 'ArrowDown', mode === 'right' ? 'ArrowRight' : 'ArrowLeft']
      const items = [...wrap.querySelectorAll('.sm-item:not([disabled])')], i = items.indexOf(t)
      if (e.key === next) { items[(i + 1) % items.length].focus(); e.preventDefault() }
      else if (e.key === prev) { items[(i + items.length - 1) % items.length].focus(); e.preventDefault() }
      else if (e.key === into && t.dataset.exp) { e.preventDefault(); (wrap.querySelector('.sm-finder input') || wrap.querySelector('.sm-kind'))?.focus() }
      else if (e.key === 'Escape') { t.blur(); open = null; render() }
      else if (printable && t.dataset.id === 'find') { e.preventDefault(); seed(e.key) }
    } else if (e.key === 'Escape' && !typing) { wrap.querySelector('.sm-item.pb-btn--selected')?.focus() }
  })
  window.addEventListener('resize', render)
  E.on((t) => (t === 'sel' || t === 'layout' || t === 'test' || t === 'init' || t === 'rights' || t === 'panel') && render())
}
