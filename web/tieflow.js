// A take with routes across routers (docs/tielines-design.md, Takes with plain routes and tie routes). The Tie line
// picker opens for each tie route in plan order. A take with a plain route then opens the churning-letter confirm with
// every route, and the tie routes say "Via TIE n". In a take of tie routes only, the last picker sends the take, as the
// picker replaces the confirm. The rules are in tietakerules.js.
//
// The plan of each tie route counts the routes before it (planRequest). Two destinations that want one source share a
// tie line. A tie line that another route uses for a different source is not offered. Clear this route drops one
// tie route from the take. The take goes as one tieTake (tiepage.go): POST /takes with every route.

import { emit, request, routerData, routerOfItem } from './core.js';
import { showResult } from './banner.js';
import { noteLast } from './plans.js';
import { pickRoute } from './tiepicker.js';
import { pickState, reopenState, takeRoute, chosen, oneOffsOf } from './tiepickrules.js';
import { tieIndexes, nextTie, finalAt, planRequest, takeRequest, keptItems, changedAt, resetFrom, failedText } from './tietakerules.js';

const ref = (it) => ({ router: routerData(routerOfItem(it)).uid, n: it.n });

// routesOf gives each route of plan, in plan order (tietakerules.js).
function routesOf(plan) {
  const out = [];
  for (const item of plan.items) {
    for (const g of item.groups) {
      const src = ref(g.src);
      const dst = ref(item.dst);
      out.push({ src, dst, levels: g.levels, tie: src.router !== dst.router, item, label: `${g.src.label} to ${item.dst.label}` });
    }
  }
  return out;
}

// tieTake runs the pickers and the confirm of plan (planBlocked gave tie). prot: the Protects of its plain routes.
// opts: onSent(plan) after the last picker sent the take, onClose() after a cancel, confirm(plan, prot) opens the
// churning-letter confirm (its plan sends with send).
export function tieTake(plan, prot, opts) {
  const routes = routesOf(plan);
  const f = { plan, prot, opts, routes, picks: [] };
  step(f, nextTie(routes, f.picks, 0), pickState);
}

// oneOffs: the One-off tie lines that the picker of route k offers again (a reopen).
function step(f, k, state, oneOffs = []) {
  if (k < 0) return finish(f);
  const ties = tieIndexes(f.routes);
  const many = ties.length > 1 || f.routes.some((r) => !r.tie);
  const final = finalAt(f.routes, k);
  const on = () => step(f, nextTie(f.routes, f.picks, k + 1), pickState);
  pickRoute({
    routes: () => planRequest(f.routes, f.picks, k),
    state,
    oneOffs,
    head: many ? `Route ${ties.indexOf(k) + 1} of ${ties.length} across routers` : '',
    final,
    clear: many
      ? () => {
          f.picks[k] = { cleared: true };
          on();
        }
      : null,
    onPick: (st) => {
      const hops = chosen(st).hops.slice(0, -1);
      const names = hops.map((h) => h.tie.name || h.tie.id);
      const take = takeRoute(st);
      const prot = take.ties.flatMap((t, i) => (t.protect ? [names[i]] : []));
      // ids name the chosen path (pathKey): a One-off tie line goes to the server as its ends, not its id.
      f.picks[k] = { take, names, prot, ids: hops.map((h) => h.tie.id) };
      if (final) return send(f, true);
      on();
    },
    onClose: () => f.opts.onClose?.(),
  });
}

// finish: every tie route has a pick or is cleared. The confirm lists every route that stays.
function finish(f) {
  const items = keptItems(f.plan.items, f.routes, f.picks);
  if (!items.length) {
    showResult('info', 'Nothing to take', 'You cleared every route.');
    return f.opts.onClose?.();
  }
  f.opts.confirm(
    {
      ...f.plan,
      items,
      send: () => {
        send(f, false);
        return true;
      },
    },
    f.prot,
  );
}

// textOf says what the take does: each route, and the tie lines of a tie route.
function textOf(f, at) {
  return at
    .map((i) => {
      const r = f.routes[i];
      if (!r.tie) return r.label;
      const { names, prot } = f.picks[i];
      return `${r.label} Via ${names.join(', ')}${prot.length ? `, then Protect ${prot.join(', ')}` : ''}`;
    })
    .join('; ');
}

async function send(f, direct) {
  const { routes, at } = takeRequest(f.routes, f.picks);
  const text = textOf(f, at);
  showResult('info', 'Taking', text);
  const m = request('tieTakeResult', { t: 'tieTake', routes });
  if (direct) f.opts.onSent?.(f.plan);
  answer(f, await m, text, at);
}

function answer(f, m, text, at) {
  const rs = m.body?.results;
  if (m.status === 200 && rs) {
    if (m.body.ok) {
      // Last holds the typed source and destination, not the hops, as after a plain take.
      if (noteLast({ ...f.plan, items: keptItems(f.plan.items, f.routes, f.picks) })) emit('last');
      // A Protect that failed leaves the take a success (docs/tielines-design.md, Protect in a tie take).
      const bad = protectFailures(m.body.protects);
      if (bad) return showResult('warn', 'Success, Protect failed', `${text}. ${bad}`);
      return showResult('ok', 'Success', text);
    }
    const bad = rs.flatMap((r, i) => (r.result === 'failed' ? [`${f.routes[at[i]].label}: ${failedText(r)}`] : []));
    return showResult('error', 'Take failed', bad.join('; ') || text);
  }
  if (m.body?.error?.code === 'tie_line_changed') return reopen(f, m.body, at);
  showResult('error', 'Not sent', m.body?.error?.message || m.err || 'The server refused the take.');
}

// protectFailures names each Protect of the take that failed, or ''.
const protectFailures = (ps = []) =>
  ps
    .filter((p) => !p.ok)
    .map((p) => `${p.tieName || p.tie}: ${p.error || 'failed'}`)
    .join('; ');

// reopen: the server refused the take with 409 tie_line_changed and sent nothing (docs/tielines-design.md, The stale
// check, rule 5). The page says which tie line changed and opens the picker again on the first route that it names.
// It chooses the same tie lines when they still make a path. The routes after it are chosen again.
function reopen(f, body, at) {
  const k = changedAt(body, at);
  const r = body.routes?.find((x) => at[x.route] === k);
  const names = (r?.changed || []).map((t) => t.name || t.id);
  const fresh = r?.newAffected || [];
  const what = names.length ? `${names.join(', ')} changed` : fresh.length ? 'The Affected destinations changed' : 'A tie line changed';
  showResult('error', 'Tie line changed', `${what}. Sode sent nothing. Choose again.`);
  if (k < 0) return;
  const { ids, take } = f.picks[k];
  const oneOffs = oneOffsOf(take);
  const prot = ids.filter((id, i) => take.ties[i]?.protect);
  const changed = (r?.changed || []).map((t) => t.id);
  f.picks = resetFrom(f.picks, k);
  step(f, k, (route) => reopenState(route, ids, changed, fresh, prot), oneOffs);
}
