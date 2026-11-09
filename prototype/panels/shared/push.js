// PROTOTYPE: copy this panel to other panels, and checkpoint sets that give each panel its own saved layout.
// Everything is mock state; remote panels would pull the saved config and show the refresh banner.
import { icon } from './edview.js'
const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c])

export function pushUI(E) {
  for (const n of ['EVS 1', 'EVS 2', 'EVS 3', 'EVS 4']) if (!E.panels.some((p) => p.name === n)) E.panels.push({ name: n, who: 'You', own: true, acts: 'api' })
  E.emit('panel', false)
  const GROUPS = [['All panels', () => E.panels.filter((p) => p.own).map((p) => p.name)], ['EVS bank', () => ['EVS 1', 'EVS 2', 'EVS 3', 'EVS 4']], ['Studio', () => ['Studio A', 'Master control']]]
  let tab = 0, to = new Set(), what = { settings: true, bind: true, look: true }, confirm = null, results = null, mode = 'own'
  let sets = [
    { id: 1, name: 'Studio default', when: 'Fri 6 Nov, 22:40', map: [['EVS 1', 'EVS 1 default'], ['EVS 2', 'EVS 2 default'], ['EVS 3', 'EVS 3 default'], ['EVS 4', 'EVS 4 default']] },
    { id: 2, name: 'Friday show', when: 'Fri 6 Nov, 18:05', map: [['EVS 1', 'Cams 1-8'], ['EVS 2', 'Cams 9-16'], ['EVS 3', 'Replays'], ['EVS 4', 'Graphics']] },
  ]
  const bar = document.querySelector('.pb-toolbar'), anchor = bar.querySelector('#tb-test').closest('label')
  const btn = Object.assign(document.createElement('button'), { className: 'pb-btn pb-btn--sm', innerHTML: icon('copy') + ' Push' })
  anchor.before(btn)
  const dlg = Object.assign(document.createElement('dialog'), { className: 'ed-dlg ed-dlg--wide' }); document.body.append(dlg)

  const status = (names, note) => { results = names.map((n, i) => ({ n, s: n === 'EVS 4' && i ? 'Offline, pulls when it reconnects' : note })); confirm = null; draw() }
  const rows = () => results ? `<div class="ed-sec"><h3 class="ed-h">Result</h3>${results.map((r) => `<div class="pb-row"><span>${icon(/Offline/.test(r.s) ? 'clock' : 'check')}</span><strong>${esc(r.n)}</strong><span class="pb-text-muted">${r.s}</span></div>`).join('')}</div>` : ''
  const copyTab = () => {
    const others = E.panels.filter((p) => p.own && p.name !== E.cfg.name)
    return `<p class="pb-hint">Copies <strong>${esc(E.cfg.name)}</strong> as saved. Each target keeps its own name, id and sharing, and refreshes the way an edit does, with the countdown banner.</p>
      <div class="ed-sec"><h3 class="ed-h">What to copy</h3><label class="pb-check"><input type="checkbox" class="pb-check__input" checked disabled><span class="pb-check__label">Layout and button types</span></label>
      ${[['settings', 'Panel settings (mode, guard, colours, title bar)'], ['bind', 'Sources, destinations and presets in the cells'], ['look', 'Labels, icons and images']].map(([k, t]) => `<label class="pb-check"><input type="checkbox" class="pb-check__input" data-what="${k}" ${what[k] ? 'checked' : ''}><span class="pb-check__label">${t}</span></label>`).join('')}</div>
      <div class="ed-sec"><h3 class="ed-h">Copy to</h3><div class="pb-row pb-row--wrap">${GROUPS.map(([g]) => `<button class="pb-btn pb-btn--sm" data-group="${g}">${icon('users')} ${g}</button>`).join('')}</div>
      <div class="ed-picklist">${others.map((p) => `<label class="pb-check"><input type="checkbox" class="pb-check__input" data-to="${esc(p.name)}" ${to.has(p.name) ? 'checked' : ''}><span class="pb-check__label">${esc(p.name)} <span class="pb-text-muted">${p.acts === 'api' ? 'API client' : 'signed-in user'}</span></span></label>`).join('')}</div>
      ${confirm === 'copy' ? `<div class="pb-banner pb-banner--warn"><span>This replaces the layout of ${to.size} panel${to.size > 1 ? 's' : ''}. A checkpoint of each is kept first.</span> <button class="pb-btn pb-btn--sm pb-btn--commit" data-go="copy">Yes, copy</button> <button class="pb-btn pb-btn--sm pb-btn--ghost" data-go="cancel">Cancel</button></div>` : `<button class="pb-btn pb-btn--sm pb-btn--ok" data-ask="copy" ${to.size ? '' : 'disabled'}>Copy to ${to.size} panel${to.size === 1 ? '' : 's'}</button>`}</div>${rows()}`
  }
  const setsTab = () => `<p class="pb-hint">A checkpoint set saves a layout for each panel in a group. Push the set and every panel gets its own: EVS 1 gets its checkpoint, EVS 2 gets its own.</p>
    ${sets.map((s) => `<div class="ed-tpl"><div><strong>${esc(s.name)}</strong> <span class="pb-text-muted">${s.when}</span><div class="ed-map">${s.map.map(([p, c]) => `<span class="pb-chip pb-chip--static">${esc(p)} ${icon('arrow-right')} ${esc(c)}</span>`).join('')}</div></div><div class="ed-tpl__acts">${confirm === 'set' + s.id ? `<button class="pb-btn pb-btn--sm pb-btn--commit" data-pushset="${s.id}">Yes, push</button><button class="pb-btn pb-btn--sm pb-btn--ghost" data-go="cancel">Cancel</button>` : `<button class="pb-btn pb-btn--sm pb-btn--ok" data-askset="${s.id}">Push set</button><button class="pb-btn pb-btn--sm pb-btn--ghost pb-btn--icon" data-delset="${s.id}" aria-label="Delete">${icon('trash-2')}</button>`}</div></div>`).join('')}
    <div class="ed-sec" style="margin-top:var(--pb-space-4)"><h3 class="ed-h">Save a checkpoint set</h3>
      <div class="pb-field"><label class="pb-label" for="cp-name">Name</label><input id="cp-name" class="pb-input" value="End of show"></div>
      <div class="pb-label">Panels in the set</div><div class="ed-picklist">${E.panels.filter((p) => p.own).map((p) => `<label class="pb-check"><input type="checkbox" class="pb-check__input" data-cp="${esc(p.name)}" ${/^EVS/.test(p.name) ? 'checked' : ''}><span class="pb-check__label">${esc(p.name)}</span></label>`).join('')}</div>
      <div class="pb-field"><span class="pb-label">Each panel keeps</span><span class="pb-select-wrap"><select class="pb-select" data-mode><option value="own" ${mode === 'own' ? 'selected' : ''}>Its own layout right now</option><option value="this" ${mode === 'this' ? 'selected' : ''}>This panel's layout for all of them</option></select></span></div>
      <button class="pb-btn pb-btn--sm pb-btn--commit" data-saveset>Save checkpoint set</button></div>${rows()}`
  const draw = () => {
    dlg.innerHTML = `<div class="pb-row"><h3 class="ed-h pb-grow">Push to other panels</h3><button class="pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm" data-close aria-label="Close">${icon('x')}</button></div>
      <div class="pb-tabs" role="tablist">${['Copy this panel', 'Checkpoint sets'].map((t, i) => `<button class="pb-tab" role="tab" aria-selected="${tab === i}" data-tab="${i}">${t}</button>`).join('')}</div>
      <div class="pb-tabpanel active ed-tabpane">${tab ? setsTab() : copyTab()}</div><p class="pb-hint">A panel's owner can push to their own panels. A Trusted user can push to anybody's.</p>`
  }
  dlg.onclick = (e) => {
    const b = e.target.closest('button'); if (!b) return
    if ('close' in b.dataset) return dlg.close()
    if (b.dataset.tab) { tab = +b.dataset.tab; results = null; confirm = null }
    else if (b.dataset.group) { const names = GROUPS.find((g) => g[0] === b.dataset.group)[1]().filter((n) => n !== E.cfg.name); const all = names.every((n) => to.has(n)); names.forEach((n) => (all ? to.delete(n) : to.add(n))) }
    else if (b.dataset.ask) confirm = 'copy'
    else if (b.dataset.go === 'cancel') confirm = null
    else if (b.dataset.go === 'copy') return status([...to], 'Saved. Open panels refresh in 10 s.')
    else if (b.dataset.askset) confirm = 'set' + b.dataset.askset
    else if (b.dataset.pushset) return status(sets.find((s) => s.id === +b.dataset.pushset).map.map((m) => m[0]), 'Got its own checkpoint. Refreshes in 10 s.')
    else if (b.dataset.delset) sets = sets.filter((s) => s.id !== +b.dataset.delset)
    else if ('saveset' in b.dataset) {
      const names = [...dlg.querySelectorAll('[data-cp]:checked')].map((i) => i.dataset.cp), name = dlg.querySelector('#cp-name').value || 'Checkpoint set'
      sets.unshift({ id: Date.now(), name, when: 'Just now', map: names.map((n) => [n, mode === 'this' ? E.cfg.name : n + ' now']) }); E.emit('notice', false)
    }
    draw()
  }
  dlg.onchange = (e) => {
    const t = e.target
    if (t.dataset.to) t.checked ? to.add(t.dataset.to) : to.delete(t.dataset.to)
    if (t.dataset.what) what[t.dataset.what] = t.checked
    if (t.dataset.mode !== undefined) mode = t.value
    confirm = null; if (t.dataset.to || t.dataset.what) draw()
  }
  btn.onclick = () => { to = new Set(); confirm = null; results = null; draw(); dlg.showModal() }
}
