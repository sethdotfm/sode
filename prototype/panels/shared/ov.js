// PROTOTYPE: state and the share / link modal for the panels root page (/panels). Mock data, nothing persists.
// Variants only differ in how they render OV.visible(); every action and the modal live here.
import { icon } from './edview.js'
export const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c])
export const USERS = ['Dana', 'Priya', 'Marcus', 'Ops desk', 'Seth']
const mk = (rows, cols, spec) => spec.map(([r, c, rs, cs, k]) => ({ r, c, rs, cs, k }))
const STUDIO = mk(4, 8, [[0, 0, 1, 1, 'd'], [0, 1, 1, 1, 'd'], [0, 2, 1, 1, 'd'], [0, 3, 1, 1, 'd'], [0, 4, 1, 1, 'c'], [0, 5, 1, 1, 'c'], [0, 6, 1, 1, 'p'], [0, 7, 1, 1, 'p'], [1, 0, 1, 1, 's'], [1, 1, 1, 1, 's'], [1, 2, 1, 1, 's'], [1, 3, 1, 1, 's'], [1, 4, 1, 1, 's'], [1, 5, 1, 1, 's'], [1, 6, 1, 1, 's'], [1, 7, 1, 1, 's'], [2, 0, 1, 1, 's'], [2, 1, 1, 1, 's'], [2, 2, 1, 1, 's'], [2, 3, 1, 1, 's'], [2, 4, 1, 1, 's'], [2, 5, 1, 1, 's'], [2, 6, 2, 2, 't'], [3, 0, 1, 1, 's'], [3, 1, 1, 1, 's'], [3, 2, 1, 1, 's'], [3, 3, 1, 1, 's'], [3, 4, 1, 1, 'x']])
const EVS = mk(4, 8, [[0, 0, 1, 2, 'c'], [0, 2, 1, 6, 'c'], [1, 0, 1, 1, 'p'], [1, 1, 1, 1, 'p'], [1, 2, 1, 1, 'p'], [1, 3, 1, 1, 'p'], [2, 0, 2, 4, 's'], [2, 4, 2, 2, 't'], [2, 6, 2, 2, 'x']])
const MC = mk(4, 8, [[0, 0, 1, 1, 'd'], [0, 1, 1, 1, 'd'], [0, 2, 1, 1, 'd'], [1, 0, 1, 1, 's'], [1, 1, 1, 1, 's'], [1, 2, 1, 1, 's'], [2, 0, 2, 2, 't'], [2, 2, 2, 2, 'p']])
export const ME = 'Seth'
const now = () => Date.now()
export function createOverview() {
  const subs = []
  const OV = {
    me: ME, role: 'mine', filter: '', owner: '', sel: new Set(), note: '', trustedUser: true,
    panels: [
      { id: 'studio-a', name: 'Studio A', owner: ME, acts: 'user', size: '4x8', cells: STUDIO, share: 'users', with: ['Dana'], inst: ['Studio A iPad', 'Control room PC'], watch: true, upd: '2 days ago', mode: 'Take mode' },
      { id: 'replay-desk', name: 'Replay desk', owner: ME, acts: 'api', size: '4x8', cells: EVS, share: 'private', with: [], inst: ['Replay Pi (kiosk)'], watch: true, upd: 'Today, 09:12', mode: 'Instant mode', key: { name: 'Panel Replay desk', made: '3 Nov', used: '1 min ago', open: true } },
      ...[1, 2, 3, 4].map((n) => ({ id: 'evs-' + n, name: 'EVS ' + n, owner: ME, acts: 'api', size: '4x8', cells: EVS, share: 'private', with: [], inst: n === 4 ? [] : ['EVS ' + n + ' tablet'], watch: n < 3, upd: n === 4 ? '9 days ago' : '4 days ago', mode: 'Instant mode', key: { name: 'Panel EVS ' + n, made: '28 Oct', used: n === 4 ? '9 days ago' : '3 min ago', open: n !== 4 } })),
      { id: 'master', name: 'Master control', owner: 'Dana', acts: 'user', size: '4x8', cells: MC, share: 'users', with: [ME, 'Priya'], inst: ['MCR wall'], watch: false, upd: 'Yesterday', mode: 'Take mode' },
      { id: 'edit-2', name: 'Edit bay 2', owner: 'Priya', acts: 'user', size: '4x8', cells: MC, share: 'everyone', with: [], inst: [], watch: false, upd: '5 days ago', mode: 'Take mode' },
      { id: 'ops-wall', name: 'Ops wall', owner: 'Ops desk', acts: 'api', size: '4x8', cells: STUDIO, share: 'private', with: [], inst: ['Ops wall display', 'Ops iPad'], watch: false, upd: '12 days ago', mode: 'Instant mode', key: { name: 'Panel Ops wall', made: '1 Oct', used: '2 min ago', open: true } },
      { id: 'marcus-test', name: 'Marcus scratch', owner: 'Marcus', acts: 'user', size: '4x8', cells: MC, share: 'private', with: [], inst: [], watch: false, upd: '20 days ago', mode: 'Take mode' },
    ],
    get(id) { return OV.panels.find((p) => p.id === id) },
    // A panel is online while at least one instance is.
    online(p) { return p.inst.length > 0 },
    mine(p) { return p.owner === ME },
    sharedWithMe(p) { return !OV.mine(p) && (p.share === 'everyone' || p.with.includes(ME)) },
    canEdit(p) { return OV.mine(p) || (OV.role === 'trusted' && OV.trustedUser) },
    canShare(p) { return OV.canEdit(p) },
    canWatch(p) { return OV.mine(p) || OV.role === 'trusted' },
    visible() {
      const q = OV.filter.trim().toLowerCase()
      return OV.panels.filter((p) => (OV.role === 'trusted' || OV.mine(p) || OV.sharedWithMe(p)) && (!q || (p.name + ' ' + p.owner).toLowerCase().includes(q)) && (!OV.owner || p.owner === OV.owner))
    },
    owners() { return [...new Set(OV.panels.map((p) => p.owner))] },
    on(fn) { subs.push(fn); fn('init') },
    emit(t = 'data') { subs.forEach((f) => f(t)) },
    say(m) { OV.note = m; OV.emit('note'); clearTimeout(OV._t); OV._t = setTimeout(() => { OV.note = ''; OV.emit('note') }, 3500) },
    setRole(r) { OV.role = r; OV.sel.clear(); OV.owner = ''; OV.emit('role') },
    toggleWatch(id) { const p = OV.get(id); p.watch = !p.watch; OV.say(p.watch ? `Watching ${p.name}. You are told when it goes offline.` : `Stopped watching ${p.name}.`); OV.emit() },
    launch(id, kiosk) { const p = OV.get(id); OV.say(`Opens /panels/${p.id}${kiosk ? '?kiosk' : ''}${p.acts === 'user' ? ' and asks you to sign in' : ''}.`) },
    edit(id) { OV.say(`Opens /panels/editor?panel=${id}.`) },
    copy(id) { const p = OV.get(id); OV.panels.unshift({ ...p, id: p.id + '-copy', name: p.name + ' copy', owner: ME, share: 'private', with: [], inst: [], watch: false, upd: 'Just now', key: undefined, acts: 'user' }); OV.say(`Saved a copy of ${p.name}. It is yours to edit.`); OV.emit() },
    remove(id) { const p = OV.get(id); OV.panels = OV.panels.filter((x) => x.id !== id); OV.sel.delete(id); OV.say(`Deleted ${p.name}.`); OV.emit() },
    newPanel() { OV.say('Opens /panels/editor with a new panel.') },
    setShare(id, mode) { const p = OV.get(id); p.share = mode; OV.emit('share') },
    toggleWith(id, u) { const p = OV.get(id); p.with = p.with.includes(u) ? p.with.filter((x) => x !== u) : [...p.with, u]; p.share = p.with.length ? 'users' : 'private'; OV.emit('share') },
    // The key lives in the URL fragment so it is never sent to the server in a request line or a proxy log.
    keyUrl(p) { return `https://sode.studio.lan/panels/${p.id}#key=${p.key && p.key.open ? 'pk_live_8fQ2x9Zr4VnL' : '…'}` },
    makeKey(id) { const p = OV.get(id); p.key = { name: 'Panel ' + p.name, made: 'Just now', used: 'never', open: true }; p.acts = 'api'; OV.emit('share') },
    rotateKey(id) { const p = OV.get(id); p.key.made = 'Just now'; p.key.used = 'never'; OV.say('Rotated. Links with the old key stop working now, and open instances sign out.'); OV.emit('share') },
    revokeKey(id) { const p = OV.get(id); p.key = undefined; p.inst = []; OV.say('Revoked. The panel has no key and every link to it is dead.'); OV.emit('share') },
  }
  return OV
}

// One modal for sharing and the key link. Tabs: People (who can use it) and Link (the key in a URL, API client panels only).
export function shareModal(OV) {
  const dlg = Object.assign(document.createElement('dialog'), { className: 'ed-dlg ov-dlg' })
  document.body.append(dlg)
  let id = null, tab = 'people', confirm = '', shown = false
  const p = () => OV.get(id)
  const draw = () => {
    const x = p(); if (!x) return dlg.close()
    const who = OV.mine(x) ? 'You own this panel.' : `${esc(x.owner)} owns this panel. You can change it because you are a Trusted user.`
    const people = `<p class="pb-hint">${who} People you share with can use the panel and save their own copy. They cannot change yours.</p>
      <div class="ov-radios" role="radiogroup" aria-label="Who can use it">
        ${[['private', 'lock', 'Only me', 'Nobody else sees it in their list.'], ['users', 'users', 'Named users', 'Pick people below.'], ['everyone', 'globe', 'Everyone', 'Every signed-in user sees it. Guests do not.']].map(([k, ic, t, d]) => `<label class="pb-check ov-radio"><input type="radio" class="pb-check__input" name="share" data-share="${k}" ${x.share === k ? 'checked' : ''}><span class="pb-check__label">${icon(ic)} <strong>${t}</strong> <span class="pb-text-muted">${d}</span></span></label>`).join('')}
      </div>
      <div class="ed-sec ov-users" ${x.share === 'everyone' ? 'hidden' : ''}><h3 class="ed-h">People</h3>
        <div class="ed-picklist">${USERS.filter((u) => u !== x.owner).map((u) => `<label class="pb-check"><input type="checkbox" class="pb-check__input" data-with="${esc(u)}" ${x.with.includes(u) ? 'checked' : ''}><span class="pb-check__label">${esc(u)}</span></label>`).join('')}</div></div>
      <p class="pb-hint">${x.acts === 'user' ? 'This panel acts as whoever signs in to it, so each person is held to their own Rights. Sharing it never widens anyone\'s Rights.' : 'This panel acts as its own API client. Sharing it lets people open it, and what it can do stays within its own Rights.'}</p>`
    let link
    if (x.acts === 'user') link = `<div class="pb-banner pb-banner--info"><span>${icon('user')} This panel signs in as a user. There is no key to share. Whoever opens <code>/panels/${x.id}</code> enters their PIN.</span></div>
        <p class="pb-hint">To make a link that opens with no sign-in, switch it to act as an API client. Its Rights then cap what the link can do.</p>
        ${OV.canEdit(x) ? '<button class="pb-btn pb-btn--sm" data-makekey>' + icon('key-round') + ' Switch to an API client and make a key</button>' : ''}`
    else if (!x.key) link = `<p class="pb-hint">This panel has no key. Make one to get a link that opens it with no sign-in.</p><button class="pb-btn pb-btn--sm pb-btn--ok" data-makekey>${icon('key-round')} Make a key</button>`
    else link = `<div class="pb-banner pb-banner--warn"><span>${icon('shield-check')} Anyone with this link can use the panel with its Rights, until you rotate or revoke the key. Share it only with people and devices you would let patch the router.</span></div>
        <div class="pb-field"><label class="pb-label" for="ov-url">Link with the key in it</label><div class="pb-row"><input id="ov-url" class="pb-input pb-grow" readonly value="${esc(shown ? OV.keyUrl(x) : OV.keyUrl(x).replace(/key=.*/, 'key=••••••••••••••••'))}"><button class="pb-btn pb-btn--sm pb-btn--icon" data-reveal aria-label="${shown ? 'Hide' : 'Show'} the key" data-pb-tip="${shown ? 'Hide' : 'Show'}">${icon('eye')}</button><button class="pb-btn pb-btn--sm" data-copyurl>${icon('copy')} Copy link</button></div></div>
        <div class="pb-row pb-row--wrap"><button class="pb-btn pb-btn--sm" data-qr>${icon('qr-code')} Show QR code</button><button class="pb-btn pb-btn--sm" data-openkiosk>${icon('external-link')} Open as kiosk</button></div>
        <dl class="ov-kv"><dt>Key name</dt><dd>${esc(x.key.name)}</dd><dt>Made</dt><dd>${x.key.made}</dd><dt>Last used</dt><dd>${x.key.used}</dd><dt>History shows</dt><dd>${esc(x.name)} (${esc(x.key.name)})</dd></dl>
        ${confirm ? `<div class="pb-banner pb-banner--warn"><span>${confirm === 'rotate' ? 'Old links stop working and open instances sign out.' : 'The panel loses its key and goes offline.'}</span> <button class="pb-btn pb-btn--sm pb-btn--commit" data-do="${confirm}">Yes, ${confirm}</button> <button class="pb-btn pb-btn--sm pb-btn--ghost" data-do="cancel">Cancel</button></div>` : `<div class="pb-row"><button class="pb-btn pb-btn--sm" data-ask="rotate">${icon('clock')} Rotate key</button><button class="pb-btn pb-btn--sm pb-btn--ghost" data-ask="revoke">${icon('trash-2')} Revoke key</button></div>`}`
    dlg.innerHTML = `<div class="pb-row"><h3 class="ed-h pb-grow">Share ${esc(x.name)}</h3><button class="pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm" data-close aria-label="Close">${icon('x')}</button></div>
      <div class="pb-tabs" role="tablist">${[['people', 'People'], ['link', 'Link with key']].map(([k, t]) => `<button class="pb-tab" role="tab" aria-selected="${tab === k}" data-tab="${k}">${t}</button>`).join('')}</div>
      <div class="ov-pane">${tab === 'people' ? people : link}</div>`
  }
  dlg.addEventListener('click', (e) => {
    const t = e.target.closest('button,input'); if (!t) return
    const d = t.dataset
    if ('close' in d) return dlg.close()
    if (d.tab) { tab = d.tab; confirm = ''; return draw() }
    if (d.share) return OV.setShare(id, d.share)
    if (d.with) return OV.toggleWith(id, d.with)
    if ('makekey' in d) { OV.makeKey(id); tab = 'link'; return }
    if ('reveal' in d) { shown = !shown; return draw() }
    if ('copyurl' in d) return OV.say('Copied the link. It carries the key, so treat it like a password.')
    if ('qr' in d) return OV.say('Shows a QR code of the link, for pointing a tablet camera at.')
    if ('openkiosk' in d) return OV.launch(id, true)
    if (d.ask) { confirm = d.ask; return draw() }
    if (d.do) { const a = d.do; confirm = ''; if (a === 'rotate') OV.rotateKey(id); else if (a === 'revoke') OV.revokeKey(id); else draw() }
  })
  OV.on((t) => { if (dlg.open && t !== 'note') draw() })
  OV.openShare = (pid, which = 'people') => { id = pid; tab = which; confirm = ''; shown = false; draw(); dlg.showModal() }
}

// Mini picture of a panel's grid, drawn from its cell list.
export const thumb = (p, cls = '') => `<div class="ov-thumb ${cls}" aria-hidden="true">${p.cells.map((c) => `<i class="k-${c.k}" style="grid-area:${c.r + 1}/${c.c + 1}/span ${c.rs}/span ${c.cs}"></i>`).join('')}</div>`
export const status = (OV, p) => OV.online(p) ? `<span class="pb-badge pb-badge--ok"><span class="pb-dot pb-dot--ok"></span> Online${p.inst.length > 1 ? ' ' + p.inst.length : ''}</span>` : `<span class="pb-badge"><span class="pb-dot pb-dot--idle"></span> Offline</span>`
export const shareLabel = (p) => p.share === 'everyone' ? 'Everyone' : p.share === 'users' ? p.with.length + ' user' + (p.with.length === 1 ? '' : 's') : 'Only owner'
export const actsLabel = (p) => p.acts === 'api' ? 'API client' : 'Signed-in user'

// Role switch, search and toasts shared by every variant (the page frame, not the design being tested).
export function frame(OV, host) {
  host.insertAdjacentHTML('beforebegin', '<div class="pb-toast-stack" id="ov-toasts" aria-live="polite"></div>')
  const t = document.getElementById('ov-toasts')
  OV.on((k) => { if (k === 'note') t.innerHTML = OV.note ? `<div class="pb-toast pb-toast--info"><div class="pb-toast__body"><div class="pb-toast__msg">${esc(OV.note)}</div></div></div>` : '' })
  shareModal(OV)
  const q = new URLSearchParams(location.search)
  if (q.get('role')) OV.setRole(q.get('role'))
  if (q.get('share')) OV.openShare(q.get('share'), q.get('tab') || 'people')
}
export const roleSwitch = (OV) => `<div class="pb-row" role="group" aria-label="View as"><span class="pb-label">View as</span><button class="pb-btn pb-btn--sm${OV.role === 'mine' ? ' pb-btn--selected' : ''}" data-role="mine">${icon('user')} Regular user</button><button class="pb-btn pb-btn--sm${OV.role === 'trusted' ? ' pb-btn--selected' : ''}" data-role="trusted">${icon('shield-check')} Trusted user</button></div>`

// Row actions and their click wiring; variants choose where to put them.
export const act = (OV, p, o = {}) => {
  const b = (a, ic, t, cls = '') => `<button class="pb-btn pb-btn--sm ${cls}" data-act="${a}" data-id="${p.id}">${icon(ic)}${o.icons ? '' : ' ' + t}</button>`
  const ib = (a, ic, t) => `<button class="pb-btn pb-btn--sm pb-btn--ghost pb-btn--icon" data-act="${a}" data-id="${p.id}" aria-label="${t}" data-pb-tip="${t}">${icon(ic)}</button>`
  return [b('launch', 'play', 'Launch', 'pb-btn--ok'), OV.canEdit(p) ? ib('edit', 'pencil', 'Edit') : ib('copy', 'copy', 'Save a copy'), OV.canShare(p) ? ib('share', 'share-2', 'Share') : '', OV.canShare(p) && p.acts === 'api' ? ib('link', 'key-round', 'Link with key') : '', OV.canWatch(p) ? ib('watch', p.watch ? 'bell' : 'bell-off', p.watch ? 'Watching. Click to stop' : 'Watch for outages') : ''].join('')
}
export function wire(OV, root) {
  root.addEventListener('click', (e) => {
    const r = e.target.closest('[data-role]'); if (r) return OV.setRole(r.dataset.role)
    const t = e.target.closest('[data-act]'); if (!t) return
    const id = t.dataset.id, a = t.dataset.act
    if (a === 'launch') OV.launch(id); else if (a === 'edit') OV.edit(id); else if (a === 'copy') OV.copy(id)
    else if (a === 'share') OV.openShare(id); else if (a === 'link') OV.openShare(id, 'link'); else if (a === 'watch') OV.toggleWatch(id)
    else if (a === 'push') OV.say('Opens Push with ' + OV.get(id).name + ' as the source. A checkpoint of each target is kept first.'); else if (a === 'delete') { if (OV.armed === id) { OV.armed = null; OV.remove(id) } else { OV.armed = id; OV.say('Press Delete again to remove ' + OV.get(id).name + '. Its key and links stop working.'); OV.emit() } } else if (a === 'new') OV.newPanel()
  })
}
