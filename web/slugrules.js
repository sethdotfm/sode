// A panel's short name (slug), as /panels/<slug> opens it. Pure. Keep in step with validPanelSlug (internal/app/panelstore.go).

export const SLUG_MAX = 32;

// Typing is forgiving: capitals fold down, spaces and underscores become dashes, anything else is dropped.
export const cleanSlug = (text) =>
  String(text || '')
    .toLowerCase()
    .replace(/[\s_]+/g, '-')
    .replace(/[^a-z0-9-]/g, '')
    .slice(0, SLUG_MAX);

// '' when the slug is acceptable; an empty slug is fine and means the panel has none.
export function slugProblem(slug) {
  if (!slug) return '';
  if (!/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/.test(slug)) return 'Start and end with a letter or digit.';
  if (slug === 'editor' || /^p-[0-9a-f]{8}$/.test(slug)) return 'That name is reserved. Pick another.';
  return '';
}
