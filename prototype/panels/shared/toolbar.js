// PROTOTYPE: the settings the panel settings page will own, as a top strip.
export function toolbar(panel, label, extra = '') {
  const el = document.createElement('div')
  el.className = 'toolbar'
  el.innerHTML = `<b>PROTOTYPE ${label}</b>
    <label>Mode <select id="t-mode"><option value="take">Take mode</option><option value="instant">Instant mode</option></select></label>
    <label><input type="checkbox" id="t-multi" checked> Multi-destination</label>${extra}
    <label><input type="checkbox" id="t-demo"> Show text</label>
    <span id="t-log" style="opacity:.7;margin-left:auto"></span>`
  document.body.prepend(el)
  el.querySelector('#t-mode').value = panel.st.mode
  el.querySelector('#t-mode').onchange = (e) => panel.set('mode', e.target.value)
  el.querySelector('#t-multi').onchange = (e) => panel.set('multi', e.target.checked)
  el.querySelector('#t-demo').onchange = (e) => { document.body.classList.toggle('text-on', e.target.checked); }
  panel.subscribe((st) => (el.querySelector('#t-log').textContent = st.log[0] ? 'last: ' + st.log[0] : ''))
  const err = document.createElement('div')
  err.className = 'err'
  document.body.append(err)
  panel.subscribe((st) => { err.style.display = st.error ? '' : 'none'; err.textContent = st.error || '' })
}
