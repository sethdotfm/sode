// PROTOTYPE: templates dialog and the "unassigned" chip, added to the editor toolbar.
import { icon } from './edview.js'
const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c])

export function templatesUI(E) {
  const bar = document.querySelector('.pb-toolbar')
  const anchor = bar.querySelector('#tb-test').closest('label')
  const chip = document.createElement('span'); chip.className = 'pb-row pb-row--tight'
  const btn = document.createElement('button'); btn.className = 'pb-btn pb-btn--sm'; btn.innerHTML = icon('layout-grid') + ' Templates'
  anchor.before(chip, btn)
  const dlg = document.createElement('dialog'); dlg.className = 'ed-dlg'; document.body.append(dlg)
  const draw = () => {
    const unb = E.unbound().length
    chip.innerHTML = unb ? `<span class="pb-badge pb-badge--warn">${unb} unassigned</span><button class="pb-btn pb-btn--sm pb-btn--ghost" data-next>Next</button>` : ''
    dlg.innerHTML = `<div class="pb-row"><h3 class="ed-h pb-grow">Templates</h3><button class="pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm" data-close aria-label="Close">${icon('x')}</button></div>
      <p class="pb-hint">A template keeps the layout and button types. Cells come back unassigned, ready for this show's sources and destinations.</p>
      ${E.templates.map((t) => `<div class="ed-tpl"><div><strong>${esc(t.name)}</strong> <span class="pb-badge">${t.scope}</span><br><span class="pb-text-muted">${t.cols}&times;${t.rows}, ${t.cells.length} cells, by ${esc(t.by)}</span></div><div class="ed-tpl__acts"><button class="pb-btn pb-btn--sm pb-btn--ok" data-new="${t.id}">New panel</button><button class="pb-btn pb-btn--sm" data-use="${t.id}">Use here</button>${t.by === 'You' ? `<button class="pb-btn pb-btn--sm pb-btn--ghost pb-btn--icon" data-del="${t.id}" aria-label="Delete">${icon('trash-2')}</button>` : ''}</div></div>`).join('')}
      <div class="ed-sec" style="margin-top:var(--pb-space-4)"><h3 class="ed-h">Save this layout as a template</h3>
        <div class="pb-field"><label class="pb-label" for="tp-name">Name</label><input id="tp-name" class="pb-input" value="${esc(E.cfg.name)} template"></div>
        <div class="pb-field"><label class="pb-label" for="tp-scope">Who can use it</label><span class="pb-select-wrap"><select id="tp-scope" class="pb-select"><option>Me</option><option>Everyone</option></select></span><span class="pb-hint">Everyone is shared. Saving one needs a Trusted user or the Advanced Settings PIN.</span></div>
        <label class="pb-check"><input type="checkbox" class="pb-check__input" id="tp-set" checked><span class="pb-check__label">Include panel settings (mode, guard, colours, font size, title bar)</span></label>
        <label class="pb-check"><input type="checkbox" class="pb-check__input" id="tp-bind"><span class="pb-check__label">Include the sources, destinations and presets too</span></label>
        <button class="pb-btn pb-btn--sm pb-btn--commit" data-save>Save template</button></div>`
  }
  dlg.onclick = (e) => {
    const b = e.target.closest('button'); if (!b) return
    const t = E.templates.find((x) => x.id === (b.dataset.new || b.dataset.use))
    if ('close' in b.dataset) dlg.close()
    else if (t) { E.useTemplate(t, !!b.dataset.new); dlg.close() }
    else if (b.dataset.del) E.deleteTemplate(b.dataset.del)
    else if ('save' in b.dataset) E.saveTemplate({ name: dlg.querySelector('#tp-name').value, scope: dlg.querySelector('#tp-scope').value, withSettings: dlg.querySelector('#tp-set').checked, withBindings: dlg.querySelector('#tp-bind').checked })
  }
  btn.onclick = () => { draw(); dlg.showModal() }
  chip.onclick = (e) => { if (e.target.closest('[data-next]')) E.nextUnbound() }
  E.on((t) => { if (['layout', 'init', 'templates'].includes(t)) draw() })
}
