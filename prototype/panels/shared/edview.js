// PROTOTYPE: shared editor views. Variants differ in how they arrange these, not in what they edit.
import { SOURCES, DESTS, PRESETS, createPanel } from './model.js'
import { buildDeck } from './deck.js'
import { KINDS, kindName, labelOf, POOL } from './ed.js'

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
        `<span class="ed-badge">${icon(kindIcon(x.kind))}</span>` + (x.lv ? `<span class="ed-lv">L${x.lv.join(',')}</span>` : '') + (x.rs * x.cs > 1 ? `<span class="ed-span">${x.cs}&times;${x.rs}</span>` : '')
      host.append(d)
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
    if (!down) return
    const s = sq(e); if (!s) return
    if (paint && E.tool !== 'select') E.paint(s.r, s.c, E.tool); else E.extend(s.r, s.c)
  }
  document.addEventListener('pointerup', () => (down = false))
  E.on((t) => { if (t !== 'cfg' && t !== 'notice' && t !== 'save') draw() })
}

// Test mode swaps the edit grid for the live stub panel at the same size, with the panel's settings applied.
export function canvas(host, E, opt = {}) {
  host.classList.add('ed-canvas')
  const inner = document.createElement('div')
  host.append(inner)
  let panel = null, built = ''
  const sync = () => {
    applyScope(host, E)
    if (panel) { for (const k of ['mode', 'multi', 'guard']) if (panel.st[k] !== E.cfg[k]) panel.set(k, E.cfg[k]) }
  }
  const render = (t) => {
    if (t === 'cfg') return sync()
    if (t === 'notice' || t === 'save') return
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

export function inspector(host, E) {
  const draw = () => {
    const sel = E.selCells(), n = E.selSquares(), one = E.single(), q = E.rect()
    const kindBtns = (cur) => KINDS.map((k) => `<button class="pb-btn pb-btn--sm${cur === k.k ? ' pb-btn--selected' : ''}" data-kind="${k.k}">${icon(k.icon)} ${k.name}</button>`).join('')
    let h = ''
    if (E.readonly) h += '<p class="pb-hint">This panel is shared with you for use only. Save a copy to edit it.</p>'
    if (!q) h += `<p class="pb-text-muted">Select a square, or drag across several to merge them.</p>`
    else if (one) {
      const x = one, pool = POOL[x.kind]
      h += `<div class="pb-label">Cell kind</div><div class="ed-kinds">${kindBtns(x.kind)}</div>`
      if (pool) h += `<div class="pb-field"><label class="pb-label" for="i-n">${x.kind === 'src' ? 'Source' : 'Destination'}</label><span class="pb-select-wrap"><select id="i-n" class="pb-select" data-bind="n">${opts(pool.map((p) => [p.n, `${p.n + 1}  ${p.name}`]), x.n)}</select></span></div>`
      if (x.kind === 'pre') h += `<div class="pb-field"><label class="pb-label" for="i-p">Preset</label><span class="pb-select-wrap"><select id="i-p" class="pb-select" data-bind="pid">${opts(PRESETS.map((p) => [p.id, p.name]), x.pid)}</select></span></div>`
      if (pool) h += `<div class="pb-label">Levels for this cell</div><div class="ed-lvls">${E.cfg.levels.map((l) => `<label class="pb-check"><input type="checkbox" class="pb-check__input" data-lv="${l}" ${!x.lv || x.lv.includes(l) ? 'checked' : ''}><span class="pb-check__label">${l}</span></label>`).join('')}<span class="pb-hint">${x.lv ? '' : 'All'}</span></div>`
      if (x.kind === 'take' || x.kind === 'clr' || x.kind === 'cur') h += `<p class="pb-hint">${{ take: 'Confirms the plan. Lights when armed.', clr: 'Drops every staged change and the viewed destination.', cur: 'Shows the source now on the viewed destination. Not pressable.' }[x.kind]}</p>`
      h += `<div class="pb-row pb-row--tight">${x.rs * x.cs > 1 ? `<button class="pb-btn pb-btn--sm" data-act="split">${icon('split')} Split</button>` : ''}<button class="pb-btn pb-btn--sm pb-btn--danger" data-act="remove">${icon('trash-2')} Remove</button></div>`
    } else {
      h += `<p><strong>${n}</strong> squares selected${sel.length ? `, ${sel.length} with cells` : ', all empty'}.</p>`
      h += `<div class="pb-label">Make them all</div><div class="ed-kinds">${kindBtns(sel.length && sel.every((c) => c.kind === sel[0].kind) ? sel[0].kind : '')}</div>`
      h += `<div class="pb-row pb-row--tight"><button class="pb-btn pb-btn--sm pb-btn--commit" data-act="merge">${icon('merge')} Merge into one</button>${sel.length ? `<button class="pb-btn pb-btn--sm pb-btn--danger" data-act="remove">${icon('trash-2')} Remove</button>` : ''}</div>`
    }
    host.innerHTML = h
  }
  host.onclick = (e) => {
    const b = e.target.closest('button'); if (!b || E.readonly) return
    if (b.dataset.kind) E.setKind(b.dataset.kind)
    else if (b.dataset.act === 'merge') E.merge()
    else if (b.dataset.act === 'split') E.split(E.single())
    else if (b.dataset.act === 'remove') E.remove()
  }
  host.onchange = (e) => {
    const t = e.target, x = E.single(); if (!x) return
    if (t.dataset.bind) E.bind(x, t.dataset.bind, t.dataset.bind === 'pid' ? t.value : +t.value)
    if (t.dataset.lv) E.levelsOf(x, +t.dataset.lv, t.checked)
  }
  E.on((t) => (t === 'sel' || t === 'layout' || t === 'init') && draw())
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
  layout: (E) => `<div class="ed-steps">${[['rows', 'Rows', E.rows], ['cols', 'Columns', E.cols]].map(([k, l, v]) => `<div class="ed-step"><span class="pb-label">${l}</span><button class="pb-btn pb-btn--icon pb-btn--sm" data-step="${k}" data-d="-1" aria-label="Fewer ${l}">${icon('minus')}</button><b class="pb-num">${v}</b><button class="pb-btn pb-btn--icon pb-btn--sm" data-step="${k}" data-d="1" aria-label="More ${l}">${icon('plus')}</button></div>`).join('')}</div><button class="pb-btn pb-btn--sm" data-act="fill">Fill empty squares with Sources</button>`,
}

// Renders the named sections into host and wires every [data-cfg] control. Re-renders on structural cfg changes only, so typing keeps focus.
export function settings(host, E, names) {
  const draw = () => { host.innerHTML = names.map((n) => `<div class="ed-sec" data-sec="${n}">${SECTIONS[n](E)}</div>`).join(''); applyScope(host.closest('.ed-scope') || host, E) }
  const redraw = (...secs) => { for (const s of secs) { const n = host.querySelector(`[data-sec="${s}"]`); if (n) n.innerHTML = SECTIONS[s](E) } }
  host.oninput = host.onchange = (e) => {
    const t = e.target
    if (E.readonly) return
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
export function toolbar(E, label, { steppers = false } = {}) {
  const el = document.createElement('div')
  el.className = 'pb-toolbar'
  el.innerHTML = `<span class="pb-toolbar__title">Editor ${label}</span>
    <span class="pb-text" id="tb-name"></span><span id="tb-dirty"></span>
    ${steppers ? '<span class="pb-toolbar__sep"></span><div class="pb-toolbar__group" id="tb-steps"></div>' : ''}
    <span class="pb-toolbar__spacer"></span>
    <label class="pb-check"><input type="checkbox" class="pb-check__input" id="tb-test"><span class="pb-check__label">Test it live</span></label>
    <label class="pb-check"><input type="checkbox" class="pb-check__input" id="tb-trusted" checked><span class="pb-check__label">Viewer is Trusted</span></label>
    <span class="pb-select-wrap"><select id="tb-theme" class="pb-select pb-w-sm" aria-label="Theme"><option value="dark">Dark</option><option value="light">Light</option></select></span>
    <button class="pb-btn pb-btn--sm pb-btn--ghost" id="tb-discard">Discard</button><button class="pb-btn pb-btn--sm pb-btn--ok" id="tb-save">${icon('save')} Save</button>`
  document.body.prepend(el)
  const stack = document.createElement('div'); stack.className = 'pb-toast-stack'; stack.setAttribute('aria-live', 'polite'); document.body.append(stack)
  document.documentElement.dataset.theme = 'dark'
  el.querySelector('#tb-theme').onchange = (e) => (document.documentElement.dataset.theme = e.target.value)
  el.querySelector('#tb-test').onchange = (e) => { E.test = e.target.checked; E.emit('test', false) }
  el.querySelector('#tb-trusted').onchange = (e) => { E.trusted = e.target.checked; if (!E.trusted && E.cfg.acts === 'api') E.cfg.acts = 'user'; E.emit('trusted', false) }
  el.querySelector('#tb-save').onclick = () => E.save()
  el.querySelector('#tb-discard').onclick = () => location.reload()
  E.on((t) => {
    el.querySelector('#tb-name').textContent = E.cfg.name
    el.querySelector('#tb-dirty').innerHTML = E.dirty ? '<span class="pb-badge pb-badge--warn">Unsaved</span>' : ''
    el.querySelector('#tb-save').disabled = !E.dirty
    el.querySelector('#tb-discard').disabled = !E.dirty
    if (steppers && (t === 'init' || t === 'layout')) { const s = el.querySelector('#tb-steps'); s.innerHTML = SECTIONS.layout(E).split('<button class="pb-btn pb-btn--sm" data-act="fill">')[0]; s.onclick = (e) => { const b = e.target.closest('[data-step]'); if (b) E.resize(E.rows + (b.dataset.step === 'rows' ? +b.dataset.d : 0), E.cols + (b.dataset.step === 'cols' ? +b.dataset.d : 0)) } }
    if (t === 'notice' && E.notice) { stack.innerHTML = `<div class="pb-toast pb-toast--info" role="status"><div class="pb-toast__body"><div class="pb-toast__msg"></div></div></div>`; stack.querySelector('.pb-toast__msg').textContent = E.notice; setTimeout(() => stack.replaceChildren(), 3500) }
  })
}
