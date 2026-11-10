// Templates in the panel editor: one toolbar button opens a modal to apply a saved layout or save this one. The
// rules (what a save sends, how an apply fills the grid) are templaterules.js; the server is /api/v1/panel-templates.
// initTemplates(editor) takes what runEditor returns.
import { $, el, on } from './core.js';
import { sessionFetch, jsonCall } from './sessionkey.js';
import { check } from './formbits.js';
import { trapDialog } from './dialogkeys.js';
import { saveTemplateBody, templateGrid, groupTemplates, templateNote, pinNeeded } from './templaterules.js';
import { pagesOf } from './panelpages.js';

const JSON_HEADERS = { 'Content-Type': 'application/json' };

// The Lock covers the page, so an open modal must not stay in front of it.
let closeOpen = null;
on('locking', () => closeOpen?.());

const call = jsonCall(sessionFetch, { down: 'Sode did not answer.', failed: (s) => `The request failed (${s})` });

export function initTemplates(editor) {
  const open = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Templates');
  open.type = 'button';
  open.id = 'ed-templates';
  open.setAttribute('aria-haspopup', 'dialog');
  $('ed-name')
    .closest('.sode-ed__grp')
    .after(el('div', 'sode-ed__grp', open));
  open.addEventListener('click', openDialog);

  function openDialog() {
    const opener = document.activeElement;
    let templates = null; // null while loading
    let scope = 'me';
    let pinAsked = false;
    let busy = false;

    const note = el('span', 'sode__hint');
    note.setAttribute('role', 'status');
    const listBox = el('div', 'sode-tpl__list');
    const name = el('input', 'pb-input');
    name.type = 'text';
    name.maxLength = 64;
    name.autocomplete = 'off';
    name.id = 'tpl-name';
    name.value = editor.panel()?.name || '';
    const scopes = el('div', 'pb-btn-group');
    scopes.setAttribute('role', 'radiogroup');
    scopes.setAttribute('aria-label', 'Who sees this template');
    const withSettings = check('Include the panel settings', { id: 'tpl-settings' });
    const withSources = check('Store with Sources (and Presets)', { id: 'tpl-sources' });
    const withDests = check('Store with Destinations', { id: 'tpl-dests' });
    const pin = el('input', 'pb-input');
    pin.type = 'password';
    pin.autocomplete = 'off';
    pin.id = 'tpl-pin';
    pin.setAttribute('aria-label', 'Advanced Settings PIN');
    const pinRow = el('label', 'sode-tpl__pin', 'Everyone templates need a Trusted user or the Advanced Settings PIN', pin);
    pinRow.hidden = true;
    const saveBtn = el('button', 'pb-btn pb-btn--ok', 'Save template');
    saveBtn.type = 'button';
    saveBtn.id = 'tpl-save';
    const closeBtn = el('button', 'pb-btn pb-btn--outline', 'Close');
    closeBtn.type = 'button';

    const form = el(
      'div',
      'sode-tpl__form',
      el('h3', 'sode-tpl__head', 'Save this layout'),
      el('label', 'sode-tpl__field', 'Name', name),
      scopes,
      el(
        'p',
        'sode__hint',
        'A template keeps the rows, columns and button types. Cells come back unassigned unless you store them with Sources or Destinations.',
      ),
      withSettings.label,
      withSources.label,
      withDests.label,
      pinRow,
      saveBtn,
    );

    function paintScopes() {
      scopes.replaceChildren(
        ...[
          ['me', 'Me'],
          ['everyone', 'Everyone'],
        ].flatMap(([id, text]) => {
          const r = el('input', 'pb-btn-check');
          r.type = 'radio';
          r.name = 'tpl-scope';
          r.id = `tpl-scope-${id}`;
          r.checked = scope === id;
          r.addEventListener('change', () => {
            scope = id;
            pinRow.hidden = !(scope === 'everyone' && pinAsked);
          });
          const l = el('label', 'pb-btn pb-btn--outline', text);
          l.htmlFor = r.id;
          return [r, l];
        }),
      );
    }

    function paintList() {
      listBox.replaceChildren();
      if (templates === null) return void listBox.append(el('p', 'sode__hint', 'Loading templates.'));
      const { me, everyone } = groupTemplates(templates);
      if (!templates.length) listBox.append(el('p', 'sode__hint', 'No templates yet.'));
      for (const [label, group] of [
        ['Me', me],
        ['Everyone', everyone],
      ]) {
        if (!group.length) continue;
        listBox.append(el('h3', 'sode-tpl__head', label));
        for (const t of group) listBox.append(row(t));
      }
    }

    function row(t) {
      const apply = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Apply');
      apply.type = 'button';
      apply.dataset.apply = t.id;
      apply.setAttribute('aria-label', `Apply ${t.name}`);
      apply.disabled = busy || editor.readOnly();
      apply.addEventListener('click', () => applyTemplate(t));
      const kids = [el('span', 'sode-tpl__name', t.name), el('span', 'sode__hint', templateNote(t)), apply];
      if (t.canEdit) {
        const del = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Delete');
        del.type = 'button';
        del.setAttribute('aria-label', `Delete ${t.name}`);
        del.disabled = busy;
        del.addEventListener('click', () => remove(t));
        kids.push(del);
      }
      return el('div', 'sode-tpl__row', ...kids);
    }

    function applyTemplate(t) {
      editor.setPages(pagesOf(t));
      editor.setGrid(templateGrid(t), { fresh: true, what: 'template' });
      if (t.settings) editor.setFields({ settings: { ...editor.fields().settings, ...t.settings } });
      editor.status(t.sources && t.dests ? `Applied ${t.name}. Save to keep it.` : `Applied ${t.name}. Fill the unassigned cells, then save.`);
      close();
      $('ed-grid').focus();
    }

    async function remove(t) {
      busy = true;
      paintList();
      try {
        await call(`api/v1/panel-templates/${t.id}`, { method: 'DELETE' });
        templates = templates.filter((x) => x.id !== t.id);
        note.textContent = `Deleted ${t.name}.`;
      } catch (e) {
        note.textContent = e.message;
      }
      busy = false;
      paintList();
    }

    saveBtn.addEventListener('click', async () => {
      if (!name.value.trim()) return void (note.textContent = 'Give the template a name.');
      busy = true;
      saveBtn.disabled = true;
      note.textContent = '';
      try {
        const made = await call('api/v1/panel-templates', {
          method: 'POST',
          headers: JSON_HEADERS,
          body: JSON.stringify(
            saveTemplateBody({
              name: name.value,
              scope,
              grid: editor.storedGrid(),
              pages: editor.pages(),
              settings: editor.fields().settings,
              withSettings: withSettings.input.checked,
              withSources: withSources.input.checked,
              withDests: withDests.input.checked,
              pin: pin.value,
            }),
          ),
        });
        pin.value = '';
        templates = [...(templates || []), made];
        note.textContent = `Saved ${made.name} to ${made.scope === 'me' ? 'Me' : 'Everyone'}.`;
      } catch (e) {
        if (pinNeeded(e.status, e.code)) {
          pinAsked = true;
          pinRow.hidden = false;
          pin.focus();
        }
        note.textContent = e.message;
      }
      busy = false;
      saveBtn.disabled = false;
      paintList();
    });

    const root = el('div', 'pb-modal-backdrop sode__scrim');
    const dialog = el('div', 'pb-modal sode-tpl');
    dialog.setAttribute('role', 'dialog');
    dialog.setAttribute('aria-modal', 'true');
    dialog.setAttribute('aria-labelledby', 'tpl-title');
    dialog.tabIndex = -1;
    dialog.append(
      el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Templates'), { id: 'tpl-title' })),
      el('div', 'pb-modal__body', listBox, form),
      el('div', 'pb-modal__footer', note, closeBtn),
    );
    root.append(dialog);
    document.body.append(root);
    const dlg = trapDialog(root, dialog, close);

    function close() {
      closeOpen = null;
      dlg.hide();
      root.remove();
      if (opener?.isConnected) opener.focus();
    }
    closeOpen = close;
    closeBtn.addEventListener('click', close);

    paintScopes();
    paintList();
    dlg.show();
    dialog.focus();
    call('api/v1/panel-templates')
      .then((r) => (templates = r.templates))
      .catch((e) => {
        templates = [];
        note.textContent = `Templates could not be loaded. ${e.message}`;
      })
      .finally(paintList);
  }
}
