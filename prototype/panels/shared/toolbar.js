// PROTOTYPE: the settings the panel settings page will own, as a patchbay toolbar.
export function toolbar(panel, label, extra = '') {
  const el = document.createElement('div')
  el.className = 'pb-toolbar'
  el.innerHTML = `<span class="pb-toolbar__title">Prototype ${label}</span>
    <div class="pb-toolbar__group"><label class="pb-label" for="t-mode">Mode</label>
      <span class="pb-select-wrap"><select id="t-mode" class="pb-select pb-w-md"><option value="take">Take mode</option><option value="instant">Instant mode</option></select></span></div>
    <label class="pb-check"><input type="checkbox" class="pb-check__input" id="t-multi" checked><span class="pb-check__label">Multi-destination</span></label>
    <label class="pb-check"><input type="checkbox" class="pb-check__input" id="t-text"><span class="pb-check__label">Result text</span></label>${extra}
    <span class="pb-toolbar__spacer"></span><span id="t-log" class="pb-text-muted pb-truncate"></span>`
  document.body.prepend(el)
  el.querySelector('#t-mode').onchange = (e) => panel.set('mode', e.target.value)
  el.querySelector('#t-multi').onchange = (e) => panel.set('multi', e.target.checked)
  el.querySelector('#t-text').onchange = (e) => document.body.classList.toggle('text-on', e.target.checked)
  panel.subscribe((st) => (el.querySelector('#t-log').textContent = st.log[0] ? 'Last take: ' + st.log[0] : ''))
  const stack = document.createElement('div')
  stack.className = 'pb-toast-stack'
  stack.setAttribute('aria-live', 'polite')
  document.body.append(stack)
  let shown = null
  panel.subscribe((st) => {
    if (st.error === shown) return
    shown = st.error
    stack.replaceChildren()
    if (st.error) stack.insertAdjacentHTML('beforeend', `<div class="pb-toast pb-toast--error" role="alert"><div class="pb-toast__body"><div class="pb-toast__msg"></div></div></div>`)
    if (st.error) stack.querySelector('.pb-toast__msg').textContent = st.error
  })
  // One clock for every blinking light. Reduced motion holds it steady; the static rings still mark state.
  if (!matchMedia('(prefers-reduced-motion: reduce)').matches) setInterval(() => (document.documentElement.dataset.beat = document.documentElement.dataset.beat === 'on' ? 'off' : 'on'), 1000)
}
