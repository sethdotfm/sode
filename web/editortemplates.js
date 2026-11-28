// Templates in the panel editor: one toolbar button opens a modal to apply a saved layout or save this one. The
// rules (what a save sends, how an apply fills the grid) are templaterules.js; the server is /api/v1/panel-templates.
// initTemplates(editor) takes what runEditor returns.
import { $, el, icon } from './core.js';
import { sessionFetch, jsonCall } from './sessionkey.js';
import { btn, check } from './formbits.js';
import { question } from './modal.js';
import { saveTemplateBody, templateGrid, groupTemplates, templateNote } from './templaterules.js';
import { pagesOf } from './panelpages.js';

const JSON_HEADERS = { 'Content-Type': 'application/json' };

const call = jsonCall(sessionFetch, { down: 'Sode did not answer.', failed: (s) => `The request failed (${s})` });

export function initTemplates(editor) {
  const open = el('button', { class: 'pb-btn pb-btn--outline pb-btn--sm', type: 'button', id: 'ed-templates', 'aria-haspopup': 'dialog' }, icon('layout-template'), 'Templates');
  $('ed-name').closest('.sode-ed__grp').append(open);
  open.addEventListener('click', openDialog);

  function openDialog() {
    const opener = document.activeElement;
    let templates = null; // null while loading
    let scope = 'me';
    let busy = false;

    const note = el('span', { class: 'sode__hint', role: 'status' });
    const listBox = el('div', 'sode-tpl__list');
    const name = el('input', { class: 'pb-input', type: 'text', maxLength: 64, autocomplete: 'off', id: 'tpl-name', value: editor.panel()?.name || '' });
    const scopes = el('div', { class: 'pb-btn-group', role: 'radiogroup', 'aria-label': 'Who sees this template' });
    const withSettings = check('Include the panel settings', { id: 'tpl-settings' });
    const withSources = check('Store with Sources and Presets', { id: 'tpl-sources' });
    const withDests = check('Store with Destinations', { id: 'tpl-dests' });
    const saveBtn = btn('sode-btn--primary', 'Save template', null, 'tpl-save');

    const form = el(
      'div',
      'sode-tpl__form',
      el('h3', 'sode-tpl__head', 'Save this layout'),
      el('label', 'sode-tpl__field', 'Name', name),
      scopes,
      withSettings.label,
      withSources.label,
      withDests.label,
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
      editor.replacePages(pagesOf(t), templateGrid(t));
      if (t.settings) editor.setFields({ settings: { ...editor.fields().settings, ...t.settings } });
      editor.status(`Applied ${t.name}.`);
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
      if (!name.value.trim()) return void (note.textContent = 'Name required.');
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
            }),
          ),
        });
        templates = [...(templates || []), made];
        note.textContent = `Saved ${made.name} to ${made.scope === 'me' ? 'Me' : 'Everyone'}.`;
      } catch (e) {
        note.textContent = e.message;
      }
      busy = false;
      saveBtn.disabled = false;
      paintList();
    });

    const q = question({
      back: opener,
      titleId: 'tpl-title',
      title: 'Templates',
      cls: 'sode-tpl',
      body: [listBox, form],
      foot: [note, btn('pb-btn--outline', 'Close', () => close())],
    });
    const close = () => q.close(null);

    paintScopes();
    paintList();
    q.dialog.focus();
    call('api/v1/panel-templates')
      .then((r) => (templates = r.templates))
      .catch((e) => {
        templates = [];
        note.textContent = `Templates could not be loaded. ${e.message}`;
      })
      .finally(paintList);
  }
}
