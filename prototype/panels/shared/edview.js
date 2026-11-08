// PROTOTYPE: shared editor views. Variants differ in how they arrange these, not in what they edit.
import { SOURCES, DESTS, PRESETS, createPanel } from './model.js'
import { buildDeck, decorate } from './deck.js'
import { KINDS, kindName, labelOf, POOL, FS, FS_NAMES, ART_ICONS, fuzzy } from './ed.js'

export const icon = (id, cls = 'pb-icon--sm') => `<svg class="pb-icon ${cls}" viewBox="0 0 24 24" aria-hidden="true"><use href="/vendor/patchbay/icons/lucide.svg#${id}"/></svg>`
const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c])
const kindIcon = (k) => KINDS.find((x) => x.k === k).icon
const opts = (list, cur) => list.map(([v, t]) => `<option value="${v}"${v === cur ? ' selected' : ''}>${esc(t)}</option>`).join('')
const DEFAULT_COL = { colView: '#e8883a', colCur: '#2fb560', colTake: '#3b82f6' }

// Colours and flags the live panel reads. --panel-src/dst are recomputed on the scope so a custom active colour mutes the same way.
export function applyScope(el, E) {
  const c = E.cfg
  el.classList.add('ed-scope')
  const set = (k, v) => (v ? el.style.setProperty(k, v) : el.style.removeProperty(k))
  set('--panel-view', c.colView); set('--panel-current', c.colCur); set('--pb-commit', c.colTake)
  el.style.setProperty('--fs', FS[c.fontSize])
  const bd = E.uploads.find((u) => u.id === c.backdrop)
  el.style.setProperty('--backdrop', bd ? `url("${bd.url}")` : 'none')
  el.classList.toggle('has-backdrop', !!bd)
  document.body.classList.toggle('text-on', c.text)
}

// Edit grid: the real panel's look (same classes) with a kind badge, a selection outline and drag select.
export function gridView(host, E, { paint = false } = {}) {
  let down = false
  const draw = () => {
    host.className = 'deck ed-grid'
    host.style.gridTemplate = `repeat(${E.rows}, 1fr) / repeat(${E.cols}, 1fr)`
    host.style.aspectRatio = `${E.cols} / ${E.rows}`
    host.replaceChildren()
    for (const x of E.cells) {
      const d = document.createElement(x.kind === 'cur' ? 'div' : 'button')
      d.className = `cell ${x.kind} ` + ({ src: 'pb-btn pb-xpt', dst: 'pb-btn pb-xpt', pre: 'pb-btn', clr: 'pb-btn pb-btn--outline', take: 'pb-btn pb-btn--commit', cur: 'pb-panel now' })[x.kind]
      d.style.gridArea = `${x.r} / ${x.c} / span ${x.rs} / span ${x.cs}`
      d.dataset.r = x.r; d.dataset.c = x.c; d.dataset.s = 'idle'
      if (E.isSel(x.r, x.c)) d.classList.add('ed-sel')
      const lbl = esc(labelOf(x))
      d.innerHTML = (x.kind === 'cur' ? `<span class="pb-label">Currently</span><span class="name">${lbl}</span>` : `<span>${lbl}</span>`) +
        `<span class="ed-badge">${icon(kindIcon(x.kind))}</span>` + (POOL[x.kind] || x.kind === 'pre' ? (E.allowed(x.kind, x.kind === 'pre' ? x.pid : x.n) ? '' : `<span class="ed-out" title="Outside your Rights">${icon('lock')}</span>`) : '') + (x.lv ? `<span class="ed-lv">L${x.lv.join(',')}</span>` : '') + (x.rs * x.cs > 1 ? `<span class="ed-span">${x.cs}&times;${x.rs}</span>` : '')
      host.append(d)
      decorate(d, { fs: x.fs ? FS[x.fs] : null, art: E.artOf(x) })
    }
    for (let r = 1; r <= E.rows; r++) for (let c = 1; c <= E.cols; c++) {
      if (E.cellAt(r, c)) continue
      const d = document.createElement('button')
      d.className = 'ed-empty' + (E.isSel(r, c) ? ' ed-sel' : '')
      d.style.gridArea = `${r} / ${c}`; d.dataset.r = r; d.dataset.c = c
      d.innerHTML = icon('plus')
      host.append(d)
    }
  }
  const sq = (e) => { const t = e.target.closest('[data-r]'); return t && { r: +t.dataset.r, c: +t.dataset.c } }
  host.onpointerdown = (e) => {
    const s = sq(e); if (!s) return
    down = true
    if (paint && E.tool !== 'select') E.paint(s.r, s.c, E.tool); else E.select(s.r, s.c, e.shiftKey)
  }
  host.onpointerover = (e) => {
    const s = sq(e); if (!s) return
    if (!down) return
    if (paint && E.tool !== 'select') E.paint(s.r, s.c, E.tool); else E.extend(s.r, s.c)
  }
  document.addEventListener('pointerup', () => (down = false))
  const ghosts = () => {
    host.querySelectorAll('.ed-ghost').forEach((n) => n.remove())
    if (!E.ghost) return
    for (const g of E.ghost.slots) {
      const d = document.createElement('div')
      d.className = 'ed-ghost' + (g.replace ? ' ed-ghost--bad' : '')
      d.style.gridArea = `${g.r} / ${g.c}`
      d.textContent = (E.ghost.kind === 'pre' ? labelOf({ kind: 'pre', pid: g.key }) : labelOf({ kind: E.ghost.kind, n: g.key }))
      host.append(d)
    }
  }
  E.on((t) => { if (t === 'ghost') ghosts(); else if (!['cfg', 'notice', 'save', 'seq', 'panel', 'trusted', 'uploads'].includes(t)) { draw(); ghosts() } })
}

// Test mode swaps the edit grid for the live stub panel at the same size, with the panel's settings applied.
export function canvas(host, E, opt = {}) {
  host.classList.add('ed-canvas')
  const bar = document.createElement('div')
  bar.className = 'ed-title'
  const inner = document.createElement('div')
  host.append(bar, inner)
  let panel = null, built = ''
  const sync = () => {
    applyScope(host, E)
    bar.hidden = !E.cfg.title
    bar.innerHTML = `<strong>${esc(E.cfg.name)}</strong><span class="pb-text-muted">${E.cfg.mode === 'take' ? 'Take mode' : 'Instant mode'}</span>`
    if (panel) { for (const k of ['mode', 'multi', 'guard']) if (panel.st[k] !== E.cfg[k]) panel.set(k, E.cfg[k]) }
  }
  const render = (t) => {
    if (t === 'cfg' || t === 'panel' || t === 'uploads') return sync()
    if (['notice', 'save', 'seq', 'ghost', 'trusted'].includes(t)) return
    const sig = E.test + JSON.stringify([E.rows, E.cols, E.deckCells()])
    if (E.test) {
      if (sig === built && t !== 'init') return
      built = sig
      panel = createPanel({ mode: E.cfg.mode, multi: E.cfg.multi, guard: E.cfg.guard })
      inner.replaceChildren(); const d = document.createElement('div'); inner.append(d)
      try { buildDeck(d, panel, E.deckCells(), E.rows, E.cols) } catch (err) { d.textContent = err.message }
    } else if (built !== 'edit' || t === 'init') {
      built = 'edit'; panel = null
      inner.replaceChildren(); const g = document.createElement('div'); inner.append(g)
      gridView(g, E, opt)
    }
    sync()
  }
  E.on(render)
  E.on((t) => { if (t === 'test') { built = ''; render('init') } })
}

// A fuzzy-findable list: type a number or part of a name. Items outside the viewer's Rights are greyed (Read-only visibility) or absent (Hidden).
export function picker(host, E, kind, cur, onPick) {
  host.className = 'ed-pick'
  host.innerHTML = '<input class="pb-input" placeholder="Find by number or name" aria-label="Find"><div class="ed-opts" role="listbox"></div>'
  const inp = host.querySelector('input'), list = host.querySelector('.ed-opts')
  let active = 0, shown = []
  const draw = () => {
    shown = E.items(kind).map((i) => ({ i, s: fuzzy(inp.value, `${i.num ?? ''} ${i.name}`) })).filter((x) => x.s >= 0).sort((a, b) => b.s - a.s)
    list.innerHTML = shown.map(({ i }, k) => `<button class="ed-opt${i.key === cur ? ' is-on' : ''}${k === active && inp.value ? ' is-active' : ''}${i.ok ? '' : ' is-out'}" data-k="${i.key}" role="option">${i.num != null ? `<span class="pb-num">${i.num}</span>` : ''}<span>${esc(i.name)}</span>${i.ok ? '' : '<span class="pb-text-muted">outside Rights</span>'}</button>`).join('') || '<p class="pb-text-muted">No match.</p>'
  }
  inp.oninput = () => { active = 0; draw() }
  inp.onkeydown = (e) => {
    if (e.key === 'ArrowDown') { active = Math.min(shown.length - 1, active + 1); draw(); e.preventDefault() }
    if (e.key === 'ArrowUp') { active = Math.max(0, active - 1); draw(); e.preventDefault() }
    if (e.key === 'Enter' && shown[active]) onPick(shown[active].i.key)
  }
  list.onclick = (e) => { const b = e.target.closest('[data-k]'); if (b) onPick(kind === 'pre' ? b.dataset.k : +b.dataset.k) }
  draw()
  list.querySelector('.is-on')?.scrollIntoView({ block: 'nearest' })
}

// Icon grid with a filter, plus uploaded images; choosing either calls onPick({kind, ref}).
export function artPicker(host, E, art, onPick) {
  const mode = art?.kind || 'none'
  host.innerHTML = `<div class="pb-label">Artwork</div><div class="pb-row pb-row--tight">${[['none', 'None'], ['icon', 'Icon'], ['image', 'Image']].map(([k, t]) => `<button class="pb-btn pb-btn--sm${mode === k ? ' pb-btn--selected' : ''}" data-m="${k}">${t}</button>`).join('')}</div><div data-body></div>`
  const body = host.querySelector('[data-body]')
  host.querySelector('.pb-row').onclick = (e) => { const b = e.target.closest('[data-m]'); if (!b) return; onPick(b.dataset.m === 'none' ? null : b.dataset.m === 'icon' ? { kind: 'icon', ref: art?.kind === 'icon' ? art.ref : 'video' } : { kind: 'image', ref: art?.kind === 'image' ? art.ref : E.uploads[0]?.id || null }) }
  if (mode === 'icon') {
    body.innerHTML = '<input class="pb-input" placeholder="Find an icon" aria-label="Find an icon"><div class="ed-icons"></div>'
    const grid = body.querySelector('.ed-icons'), inp = body.querySelector('input')
    const draw = () => (grid.innerHTML = ART_ICONS.filter((i) => fuzzy(inp.value, i.replace(/-/g, ' ')) >= 0).map((i) => `<button class="pb-btn pb-btn--ghost pb-btn--icon${art.ref === i ? ' pb-btn--selected' : ''}" data-i="${i}" data-pb-tip="${i.replace(/-/g, ' ')}" aria-label="${i}">${icon(i, '')}</button>`).join('') || '<p class="pb-text-muted">No icon matches.</p>')
    inp.oninput = draw; draw()
    grid.onclick = (e) => { const b = e.target.closest('[data-i]'); if (b) onPick({ kind: 'icon', ref: b.dataset.i }) }
  } else if (mode === 'image') uploads(body, E, art.ref, (ref) => onPick({ kind: 'image', ref }))
}

// Uploaded images for this panel: thumbnails to choose from and a button to add JPEG, PNG or SVG.
export function uploads(host, E, cur, onPick) {
  host.innerHTML = `<div class="ed-thumbs">${E.uploads.map((u) => `<button class="ed-thumb${cur === u.id ? ' is-on' : ''}" data-u="${u.id}" title="${esc(u.name)}"><img src="${u.url}" alt="${esc(u.name)}"></button>`).join('')}</div><label class="pb-btn pb-btn--sm">${icon('plus')} Upload JPEG, PNG, GIF or SVG<input type="file" accept="image/jpeg,image/png,image/gif,image/svg+xml" hidden></label><span class="pb-hint">Up to 1 MB each, GIFs may animate. Shown dimmed behind the label so the state colour still reads.</span>`
  host.querySelector('input').onchange = (e) => { const f = e.target.files[0]; const u = f && E.addUpload(f); if (u) onPick(u.id) }
  host.querySelector('.ed-thumbs').onclick = (e) => { const b = e.target.closest('[data-u]'); if (b) onPick(b.dataset.u) }
}

export function inspector(host, E) {
  const draw = () => {
    const sel = E.selCells(), n = E.selSquares(), one = E.single(), q = E.rect()
    const kindBtns = (cur) => KINDS.map((k) => `<button class="pb-btn pb-btn--sm${cur === k.k ? ' pb-btn--selected' : ''}" data-kind="${k.k}">${icon(k.icon)} ${k.name}</button>`).join('')
    const fsSel = (cur) => `<div class="pb-field"><label class="pb-label" for="i-fs">Font size</label><span class="pb-select-wrap"><select id="i-fs" class="pb-select" data-fs>${opts(FS_NAMES, cur || '')}</select></span></div>`
    let h = ''
    if (E.readonly) h += '<p class="pb-hint">This panel is shared with you for use only. Save a copy to edit it.</p>'
    if (!q) h += `<p class="pb-text-muted">Select a square, or drag across several to merge them.</p>`
    else if (one) {
      const x = one, pool = POOL[x.kind], cur = x.kind === 'pre' ? PRESETS.find((p) => p.id === x.pid)?.name : pool?.[x.n]?.name
      h += `<div class="pb-label">Button type</div><div class="ed-kinds">${kindBtns(x.kind)}</div>`
      if (pool || x.kind === 'pre') h += `<div class="pb-label">${x.kind === 'src' ? 'Source' : x.kind === 'dst' ? 'Destination' : 'Preset'}: <strong>${esc(cur || '')}</strong></div>${E.allowed(x.kind, x.kind === 'pre' ? x.pid : x.n) ? '' : '<p class="pb-hint">Outside your Rights. A press on this button would be refused.</p>'}<div data-pick></div>`
      if (pool) h += `<div class="pb-label">Levels for this cell</div><div class="ed-lvls">${E.cfg.levels.map((l) => `<label class="pb-check"><input type="checkbox" class="pb-check__input" data-lv="${l}" ${!x.lv || x.lv.includes(l) ? 'checked' : ''}><span class="pb-check__label">${l}</span></label>`).join('')}<span class="pb-hint">${x.lv ? '' : 'All'}</span></div>`
      if (x.kind === 'take' || x.kind === 'clr' || x.kind === 'cur') h += `<p class="pb-hint">${{ take: 'Confirms the plan. Lights when armed.', clr: 'Drops every staged change and the viewed destination.', cur: 'Shows the source now on the viewed destination. Not pressable.' }[x.kind]}</p>`
      h += fsSel(x.fs) + '<div data-art></div>'
      h += `<div class="pb-row pb-row--tight">${x.rs * x.cs > 1 ? `<button class="pb-btn pb-btn--sm" data-act="unmerge">${icon('split')} Unmerge</button>` : ''}<button class="pb-btn pb-btn--sm pb-btn--danger" data-act="remove">${icon('trash-2')} Remove</button></div>`
    } else {
      h += `<p><strong>${n}</strong> squares selected${sel.length ? `, ${sel.length} with cells` : ', all empty'}.</p>`
      h += `<div class="pb-label">Button type</div><div class="ed-kinds">${kindBtns(sel.length && sel.every((c) => c.kind === sel[0].kind) ? sel[0].kind : '')}</div>`
      if (sel.length) h += fsSel(sel.every((c) => c.fs === sel[0].fs) ? sel[0].fs : '')
      h += `<div class="pb-row pb-row--tight"><button class="pb-btn pb-btn--sm pb-btn--commit" data-act="merge">${icon('merge')} Merge into one</button>${E.canUnmerge() ? `<button class="pb-btn pb-btn--sm" data-act="unmerge">${icon('split')} Unmerge</button>` : ''}${sel.length ? `<button class="pb-btn pb-btn--sm pb-btn--danger" data-act="remove">${icon('trash-2')} Remove</button>` : ''}</div>`
    }
    host.innerHTML = h
    const x = one
    const pk = host.querySelector('[data-pick]')
    if (pk && x) picker(pk, E, x.kind, x.kind === 'pre' ? x.pid : x.n, (v) => E.bind(x, x.kind === 'pre' ? 'pid' : 'n', v))
    const ar = host.querySelector('[data-art]')
    if (ar && x) artPicker(ar, E, x.art, (a) => E.setLook([x], { art: a }))
  }
  host.onclick = (e) => {
    const b = e.target.closest('button'); if (!b || E.readonly) return
    if (b.dataset.kind) E.setKind(b.dataset.kind)
    else if (b.dataset.act === 'merge') E.merge()
    else if (b.dataset.act === 'unmerge') E.unmerge()
    else if (b.dataset.act === 'remove') E.remove()
  }
  host.onchange = (e) => {
    const t = e.target, x = E.single()
    if (t.dataset.lv && x) E.levelsOf(x, +t.dataset.lv, t.checked)
    if ('fs' in t.dataset) E.setLook(E.selCells(), { fs: t.value || null })
  }
  E.on((t) => (t === 'sel' || t === 'layout' || t === 'init' || t === 'rights' || t === 'uploads') && draw())
}

// Fill selection: drag across squares in the grid, type the numbers to put in them, and a preview shows in those squares before Apply.
export function sequencer(host, E) {
  host.innerHTML = `<div class="pb-label">Fill selection</div><p class="pb-hint" data-sel></p>
    <div class="ed-kinds">${[['src', 'Sources'], ['dst', 'Destinations'], ['pre', 'Presets']].map(([k, t]) => `<button class="pb-btn pb-btn--sm" data-sk="${k}">${t}</button>`).join('')}</div>
    <div class="pb-field"><label class="pb-label" for="q-text">Numbers</label><input id="q-text" class="pb-input" data-text placeholder="1-8, 12, 15-16" value="${E.fill.text}"><span class="pb-hint">As the router numbers them. Ranges and commas.</span></div>
    <div class="pb-field"><label class="pb-label" for="q-dir">Fill</label><span class="pb-select-wrap"><select id="q-dir" class="pb-select" data-o="dir"><option value="across">Across, then down</option><option value="down">Down, then across</option></select></span></div>
    <label class="pb-check"><input type="checkbox" class="pb-check__input" data-o="keep"><span class="pb-check__label">Keep cells already there</span></label>
    <p class="pb-hint" data-names></p>
    <button class="pb-btn pb-btn--sm pb-btn--commit" data-apply>Fill</button>`
  const $ = (x) => host.querySelector(x)
  const draw = () => {
    const F = E.fill, g = E.fillPlan(), n = E.selSquares()
    host.querySelectorAll('[data-sk]').forEach((b) => b.classList.toggle('pb-btn--selected', b.dataset.sk === F.kind))
    $('[data-sel]').textContent = n ? `${n} square${n > 1 ? 's' : ''} selected. The preview shows in the grid.` : 'Drag across the squares to fill, or shift-click the two corners.'
    const names = g ? g.slots.slice(0, 3).map((s) => labelOf(F.kind === 'pre' ? { kind: 'pre', pid: s.key } : { kind: F.kind, n: s.key })) : []
    $('[data-names]').textContent = !g ? '' : `${g.slots.length} to place${names.length ? ': ' + names.join(', ') + (g.slots.length > 3 ? ', ...' : '') : ''}.${g.dropped ? ` ${g.dropped} will not fit.` : ''}${g.bad.length ? ` Not found: ${g.bad.slice(0, 4).join(', ')}.` : ''}${g.slots.some((s) => s.replace) ? ' Squares that replace a cell are outlined red.' : ''}`
    const btn = $('[data-apply]'); btn.disabled = !g || !g.slots.length; btn.textContent = g && g.slots.length ? `Fill ${g.slots.length} square${g.slots.length > 1 ? 's' : ''}` : 'Fill'
    E.previewFill()
  }
  host.onclick = (e) => {
    const b = e.target.closest('button'); if (!b) return
    if (b.dataset.sk) E.setFill({ kind: b.dataset.sk, text: b.dataset.sk === 'pre' ? '1-3' : E.fill.text })
    if ('apply' in b.dataset) { E.applyFill(); $('[data-text]').focus() }
  }
  host.oninput = host.onchange = (e) => { const t = e.target; if (t.dataset.text !== undefined) E.setFill({ text: t.value }); if (t.dataset.o) E.setFill({ [t.dataset.o]: t.type === 'checkbox' ? t.checked : t.value }) }
  E.on((t) => { if (t === 'seq' || t === 'sel' || t === 'layout' || t === 'rights' || t === 'init') { if (t === 'seq' && document.activeElement === $('[data-text]')) { /* keep focus */ } draw() } })
}

const f = (id, label, ctl, hint = '') => `<div class="pb-field"><label class="pb-label" for="${id}">${label}</label>${ctl}${hint ? `<span class="pb-hint">${hint}</span>` : ''}</div>`
const sel = (id, key, list, cur) => `<span class="pb-select-wrap"><select id="${id}" class="pb-select" data-cfg="${key}">${opts(list, cur)}</select></span>`
const chk = (id, key, text, on) => `<label class="pb-check"><input id="${id}" type="checkbox" class="pb-check__input" data-cfg="${key}" ${on ? 'checked' : ''}><span class="pb-check__label">${text}</span></label>`

export const SECTIONS = {
  identity: (E) => { const c = E.cfg; return f('s-name', 'Display name', `<input id="s-name" class="pb-input" data-cfg="name" value="${esc(c.name)}">`) +
    f('s-id', 'Panel id', `<code class="pb-code">${c.id}</code>`, 'Set when the panel is made. It is the file name under panels/ and the URL.') +
    `<div class="pb-kv pb-kv--inline"><span class="pb-kv__key">Panel link</span><span class="pb-kv__val"><code class="pb-code">/panels/${c.id}</code></span></div>` },
  behaviour: (E) => { const c = E.cfg; return f('s-mode', 'Mode', sel('s-mode', 'mode', [['take', 'Take mode, press Take to confirm'], ['instant', 'Instant mode, pressing a Source confirms']], c.mode)) +
    f('s-guard', 'Guard on the confirm', sel('s-guard', 'guard', [['none', 'None'], ['hold', 'Hold to confirm'], ['double', 'Double press']], c.guard)) +
    chk('s-multi', 'multi', 'Multi-destination: stage several destinations, then one Take', c.multi) +
    chk('s-text', 'text', 'Show text beside the Result icon', c.text) +
    f('s-idle', 'Clear a pending selection after (seconds)', `<span class="pb-number-wrap"><input id="s-idle" type="number" min="0" max="600" class="pb-input pb-w-sm" data-cfg="idle" data-num value="${c.idle}"></span>`, '0 keeps it until Clear.') +
    `<div class="pb-label">Panel levels</div><div class="ed-lvls">${[1, 2, 3].map((l) => `<label class="pb-check"><input type="checkbox" class="pb-check__input" data-plv="${l}" ${c.levels.includes(l) ? 'checked' : ''}><span class="pb-check__label">${l}</span></label>`).join('')}</div><span class="pb-hint">A take sends the levels the Source, the Destination and this set share.</span>` },
  colours: (E) => { const c = E.cfg; const v = (k) => c[k] || DEFAULT_COL[k]
    const row = (k, label) => `<div class="ed-col"><label class="pb-label" for="c-${k}">${label}</label><input id="c-${k}" type="color" data-cfg="${k}" value="${v(k)}"><button class="pb-btn pb-btn--ghost pb-btn--sm" data-reset="${k}">Reset</button></div>`
    return row('colView', 'Viewed and staged destination') + row('colCur', 'Current and pending source') + row('colTake', 'Armed Take') +
      `<p class="pb-hint">Idle cells use each colour, muted. Match the colours on your hardware panel.</p>` +
      `<div class="ed-sw" data-scheme><span class="ed-sw__a" style="background:var(--panel-dst)">Idle destination</span><span class="ed-sw__a" style="background:var(--panel-src)">Idle source</span></div>` },
  access: (E) => { const c = E.cfg, api = c.acts === 'api'
    return f('s-acts', 'Acts as', `<div class="pb-row pb-row--tight"><button class="pb-btn pb-btn--sm${!api ? ' pb-btn--selected' : ''}" data-acts="user">${icon('users')} Signed-in user</button><button class="pb-btn pb-btn--sm${api ? ' pb-btn--selected' : ''}" data-acts="api" ${E.trusted ? '' : 'disabled'}>${icon('key-round')} Its own API client</button></div>`, E.trusted ? (api ? 'The panel keeps its own API key.' : 'Every press is the signed-in user, with their Rights.') : 'An API client panel needs a Trusted user.') +
      (api ? f('s-rights', 'Rights of the API client', sel('s-rights', 'rightsCap', [['mine', 'Same as mine (the most it may have)'], ['cells', 'Only what this panel\'s cells use']], c.rightsCap), 'Never wider than yours. Only an admin can widen it later.') +
        f('s-auth', 'Kiosk sign-in', sel('s-auth', 'auth', [['open', 'None, the link is the credential'], ['secure', 'Secure mode, one-time setup link']], c.auth)) : '') +
      chk('s-kiosk', 'kiosk', 'Kiosk: hide Sode branding and settings', c.kiosk) },
  sharing: (E) => { const c = E.cfg
    return f('s-share', 'Who can use this panel', sel('s-share', 'share', [['private', 'Only me'], ['named', 'Named users'], ['everyone', 'Everyone']], c.share), 'Use only. Others can save a copy to edit.') +
      (c.share === 'named' ? `<div class="pb-label">Named users</div><div class="pb-row pb-row--wrap">${c.sharedWith.map((u) => `<span class="pb-chip pb-chip--static">${u}<button class="pb-chip__remove" data-unshare="${u}" aria-label="Remove ${u}">${icon('x')}</button></span>`).join('')}<span class="pb-select-wrap"><select class="pb-select pb-w-md" data-addshare><option value="">Add a user</option>${['Dana', 'Eli', 'Priya', 'Sam'].filter((u) => !c.sharedWith.includes(u)).map((u) => `<option>${u}</option>`).join('')}</select></span></div>` : '') },
  appearance: (E) => { const c = E.cfg; const bd = E.uploads.find((u) => u.id === c.backdrop)
    return chk('s-title', 'title', 'Show a title bar with the panel name', c.title) +
      f('s-fs', 'Font size for the whole panel', sel('s-fs', 'fontSize', FS_NAMES.slice(1), c.fontSize), 'A cell can override it.') +
      `<div class="pb-label">Backdrop image${bd ? `: <strong>${esc(bd.name)}</strong>` : ''}</div><div data-backdrop></div>` },
  layout: (E) => `<div class="ed-steps">${[['rows', 'Rows', E.rows], ['cols', 'Columns', E.cols]].map(([k, l, v]) => `<div class="ed-step"><span class="pb-label">${l}</span><button class="pb-btn pb-btn--icon pb-btn--sm" data-step="${k}" data-d="-1" aria-label="Fewer ${l}">${icon('minus')}</button><span class="pb-number-wrap"><input type="number" min="1" max="${k === 'rows' ? 8 : 16}" class="pb-input pb-w-sm" data-dim="${k}" value="${v}" aria-label="${l}"></span><button class="pb-btn pb-btn--icon pb-btn--sm" data-step="${k}" data-d="1" aria-label="More ${l}">${icon('plus')}</button></div>`).join('')}</div><button class="pb-btn pb-btn--sm" data-act="fill">Fill empty squares with Sources</button>`,
}

// Renders the named sections into host and wires every [data-cfg] control. Re-renders on structural cfg changes only, so typing keeps focus.
export function settings(host, E, names) {
  const mountBackdrop = () => { const n = host.querySelector('[data-backdrop]'); if (n) { uploads(n, E, E.cfg.backdrop, (id) => { E.setCfg('backdrop', id); mountBackdrop() }); n.insertAdjacentHTML('afterbegin', `<button class="pb-btn pb-btn--ghost pb-btn--sm" data-nobd>No backdrop</button>`); n.querySelector('[data-nobd]').onclick = () => { E.setCfg('backdrop', null); mountBackdrop() } } }
  const draw = () => { host.innerHTML = names.map((n) => `<div class="ed-sec" data-sec="${n}">${SECTIONS[n](E)}</div>`).join(''); mountBackdrop() }
  const redraw = (...secs) => { for (const s of secs) { const n = host.querySelector(`[data-sec="${s}"]`); if (n) n.innerHTML = SECTIONS[s](E) } mountBackdrop() }
  host.oninput = host.onchange = (e) => {
    const t = e.target
    if (E.readonly) return
    if (t.dataset.dim) { if (e.type === 'change') { E.resize(t.dataset.dim === 'rows' ? +t.value : E.rows, t.dataset.dim === 'cols' ? +t.value : E.cols); E.emit('layout', false) } return }
    if (t.dataset.cfg) E.setCfg(t.dataset.cfg, t.type === 'checkbox' ? t.checked : t.dataset.num !== undefined ? +t.value : t.value)
    if (t.dataset.plv) { const s = new Set(E.cfg.levels); t.checked ? s.add(+t.dataset.plv) : s.delete(+t.dataset.plv); E.setCfg('levels', [...s].sort()) }
    if (t.dataset.addshare !== undefined && t.value) { E.setCfg('sharedWith', [...E.cfg.sharedWith, t.value]); redraw('sharing') }
    if (t.dataset.cfg === 'share') redraw('sharing')
    if (t.dataset.cfg === 'colView' || t.dataset.cfg === 'colCur') setTimeout(() => redraw('colours'), 0)
  }
  host.onclick = (e) => {
    const b = e.target.closest('button'); if (!b || E.readonly) return
    if (b.dataset.acts) { E.setCfg('acts', b.dataset.acts); redraw('access') }
    if (b.dataset.reset) { E.setCfg(b.dataset.reset, null); redraw('colours') }
    if (b.dataset.unshare) { E.setCfg('sharedWith', E.cfg.sharedWith.filter((u) => u !== b.dataset.unshare)); redraw('sharing') }
    if (b.dataset.step) E.resize(E.rows + (b.dataset.step === 'rows' ? +b.dataset.d : 0), E.cols + (b.dataset.step === 'cols' ? +b.dataset.d : 0))
    if (b.dataset.act === 'fill') E.fillEmpty()
  }
  E.on((t) => { if (t === 'init') draw(); if (t === 'layout') redraw('layout'); if (t === 'trusted') redraw('access') })
}

export function tabs(host, items, start = 0) {
  host.innerHTML = `<div class="pb-tabs" role="tablist">${items.map((t, i) => `<button class="pb-tab" role="tab" data-i="${i}">${t.text}</button>`).join('')}</div>`
  const panes = items.map((t) => { const d = document.createElement('div'); d.className = 'pb-tabpanel ed-tabpane'; host.append(d); return d })
  items.forEach((t, i) => t.render(panes[i]))
  const show = (i) => { host.querySelectorAll('.pb-tab').forEach((b, j) => b.setAttribute('aria-selected', String(i === j))); panes.forEach((p, j) => (p.hidden = i !== j)) }
  host.querySelector('.pb-tabs').onclick = (e) => { const b = e.target.closest('.pb-tab'); if (b) show(+b.dataset.i) }
  show(start)
  return { show }
}

// Toolbar: patchbay toolbar with the prototype-only switches (theme, viewer) plus Test, Save and Discard.
// panels: true puts the panel picker in the middle, the way the editor page names which panel is open.
export function toolbar(E, label, { steppers = false, panels = false } = {}) {
  const el = document.createElement('div')
  el.className = 'pb-toolbar'
  el.innerHTML = `<span class="pb-toolbar__title">${panels ? label : 'Editor ' + label}</span>
    ${panels ? '' : '<span class="pb-text" id="tb-name"></span>'}<span id="tb-dirty"></span>
    ${steppers ? '<span class="pb-toolbar__sep"></span><div class="pb-toolbar__group" id="tb-steps"></div>' : ''}
    <span class="pb-toolbar__spacer"></span>
    ${panels ? '<span class="pb-select-wrap ed-panelpick"><select id="tb-panel" aria-label="Panel to edit"></select></span><span class="pb-toolbar__spacer"></span>' : ''}
    <label class="pb-check"><input type="checkbox" class="pb-check__input" id="tb-test"><span class="pb-check__label">Test it live</span></label>
    <span class="pb-select-wrap"><select id="tb-rights" class="pb-select" aria-label="Viewer Rights"><option value="all">Full Rights</option><option value="ro">Limited, Read-only visibility</option><option value="hidden">Limited, Hidden visibility</option></select></span>
    <span class="pb-select-wrap"><select id="tb-theme" class="pb-select pb-w-sm" aria-label="Theme"><option value="dark">Dark</option><option value="light">Light</option></select></span>
    <button class="pb-btn pb-btn--sm pb-btn--ghost" id="tb-discard">Discard</button><button class="pb-btn pb-btn--sm pb-btn--ok" id="tb-save">${icon('save')} Save</button>`
  document.body.prepend(el)
  const stack = document.createElement('div'); stack.className = 'pb-toast-stack'; stack.setAttribute('aria-live', 'polite'); document.body.append(stack)
  document.documentElement.dataset.theme = 'dark'
  const $ = (id) => el.querySelector(id)
  $('#tb-theme').onchange = (e) => (document.documentElement.dataset.theme = e.target.value)
  $('#tb-test').onchange = (e) => { E.test = e.target.checked; E.emit('test', false) }
  $('#tb-rights').onchange = (e) => E.setRights(e.target.value)
  $('#tb-save').onclick = () => E.save()
  $('#tb-discard').onclick = () => location.reload()
  if (panels) $('#tb-panel').onchange = (e) => (e.target.value === 'new' ? E.newPanel() : E.openPanel(+e.target.value))
  E.on((t) => {
    if (!panels) $('#tb-name').textContent = E.cfg.name
    $('#tb-dirty').innerHTML = E.dirty ? '<span class="pb-badge pb-badge--warn">Unsaved</span>' : ''
    $('#tb-save').disabled = !E.dirty
    $('#tb-discard').disabled = !E.dirty
    if (panels && (t === 'init' || t === 'panel')) {
      const o = (i) => `<option value="${i}"${i === E.curPanel ? ' selected' : ''}>${esc(E.panels[i].name)}${E.panels[i].own ? '' : ' (' + E.panels[i].who + ')'}</option>`
      const idx = E.panels.map((_, i) => i)
      $('#tb-panel').innerHTML = `<optgroup label="My panels">${idx.filter((i) => E.panels[i].own).map(o).join('')}</optgroup><optgroup label="Shared with me, use only">${idx.filter((i) => !E.panels[i].own).map(o).join('')}</optgroup><option value="new">+ New panel</option>`
    }
    if (steppers && (t === 'init' || t === 'layout')) {
      const s = $('#tb-steps'); s.innerHTML = SECTIONS.layout(E).split('<button class="pb-btn pb-btn--sm" data-act="fill">')[0]
      s.onclick = (e) => { const b = e.target.closest('[data-step]'); if (b) E.resize(E.rows + (b.dataset.step === 'rows' ? +b.dataset.d : 0), E.cols + (b.dataset.step === 'cols' ? +b.dataset.d : 0)) }
      s.onchange = (e) => { const d = e.target.dataset.dim; if (d) { E.resize(d === 'rows' ? +e.target.value : E.rows, d === 'cols' ? +e.target.value : E.cols); E.emit('layout', false) } }
    }
    if (t === 'notice' && E.notice) { stack.innerHTML = `<div class="pb-toast pb-toast--info" role="status"><div class="pb-toast__body"><div class="pb-toast__msg"></div></div></div>`; stack.querySelector('.pb-toast__msg').textContent = E.notice; setTimeout(() => stack.replaceChildren(), 3500) }
  })
}
