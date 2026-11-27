// A take with routes across routers (docs/tielines-design.md, Takes with plain routes and tie routes). The Tie line
// picker asks one question for each hop that the tie routes share (nextQuestion). One source to many destinations on
// router B is one question. A take with a plain route then opens the churning-letter confirm with every route, and
// the tie routes say "Via TIE n". In a take of tie routes only, the last question sends the take, as the picker
// replaces the confirm. The rules are in tietakerules.js.
//
// Each question plans every route (planRequest). Two destinations that want one source share a tie line. A tie line
// that another route uses for a different source is not offered. Clear this route drops the routes of one question
// from the take, and "Choose per destination" gives each of them its own question. The take goes as one tieTake
// (tiepage.go): POST /takes with every route.

import { S, emit, request, routerData, routerOfItem, numText } from './core.js';
import { showResult } from './banner.js';
import { noteLast } from './plans.js';
import { noteSent, noteOwn } from './stepback.js';
import { pickRoute } from './tiepicker.js';
import { pickState, reopenState, storedState, lastState, takeRoute, chosen, fitQuestion, withOneOffs, pathKey } from './tiepickrules.js';
import {
  openAt,
  nextQuestion,
  questionIds,
  questionHead,
  finalAt,
  planRequest,
  takeRequest,
  keptItems,
  changedAt,
  reopenPicks,
  failedText,
  takeRan,
} from './tietakerules.js';

const ref = (it) => ({ router: routerData(routerOfItem(it)).uid, n: it.n });

// routesOf gives each route of plan, in plan order (tietakerules.js).
function routesOf(plan) {
  const out = [];
  for (const item of plan.items) {
    for (const g of item.groups) {
      const src = ref(g.src);
      const dst = ref(item.dst);
      // stored: a preset's tie lines (plans.js presetPlan), so its picker is the tie confirm page. last: the tie lines
      // of a take that this Redo repeats (plans.js tieAgainItem).
      const stored = g.ties?.length ? g.ties : null;
      out.push({ src, dst, levels: g.levels, tie: src.router !== dst.router, item, label: `${g.src.label} to ${item.dst.label}`, stored, last: !!g.last });
    }
  }
  return out;
}

// tieTake runs the questions and the confirm of plan (planBlocked gave tie). prot: the Protects of its plain routes.
// opts: onSent(plan) after the server ran the take, onClose() after a cancel, confirm(plan, prot) opens the
// churning-letter confirm (its plan sends with send).
// f.protect: the tie lines that the person wants Protected. f.oneOffs: the One-off tie lines that the person stated.
// f.split: the questions opened per destination. f.prev: the tie lines of each route before a reopen.
export function tieTake(plan, prot, opts) {
  const f = { plan, prot, opts, routes: routesOf(plan), picks: [], seq: 0, split: [], protect: [], oneOffs: [], prev: [], reopen: null };
  next(f);
}

// askPlans plans every route of the take. It gives { req, plans, all }, with plans[i] the route of the 409 body for
// route i. When the server gave no plan, it gives { m } with the answer.
async function askPlans(f) {
  const req = planRequest(f.routes, f.picks);
  const m = await request('tiePlanResult', { t: 'tiePlan', routes: withOneOffs(req.routes, f.oneOffs) });
  const code = m.body?.error?.code;
  if (m.status !== 409 || (code !== 'tie_line_needed' && code !== 'no_path')) return { m };
  const plans = [];
  for (const r of m.body.routes || []) plans[req.at[r.route]] = r;
  return { req, plans, all: m.body.routes };
}

// next asks the next question, or ends the take when every tie route has a path or is cleared.
async function next(f) {
  if (!openAt(f.routes, f.picks).length) return f.routes.every((r) => r.tie) ? send(f) : finish(f);
  const { req, plans, all, m } = await askPlans(f);
  if (!plans) {
    showResult('error', 'Not sent', m.body?.error?.message || m.err || 'The server refused the route.');
    return f.opts.onClose?.();
  }
  ask(f, nextQuestion(f.routes, plans, f.picks, f.split), req, plans, all);
}

// stateOf gives the first state of question q: the tie lines before a reopen, a preset's or a Redo's tie lines, or
// the best path.
function stateOf(f, q) {
  const r = f.routes[q.at];
  const prev = f.prev[q.at];
  const re = f.reopen && q.members.includes(f.reopen.k) ? f.reopen : null;
  let base = pickState;
  if (re) base = (route) => reopenState(route, prev || re.ids || [], re.changed, re.fresh);
  else if (prev) base = (route) => reopenState(route, prev);
  else if (r.stored) base = r.last ? (route) => lastState(route, r.stored) : (route) => storedState(route, r.stored);
  return (route) => fitQuestion({ ...base(route), protect: [...f.protect] }, q);
}

// pickOf is the pick of a route with its whole path, from the state of a picker.
function pickOf(f, st) {
  const hops = chosen(st).hops.slice(0, -1);
  const names = hops.map((h) => h.tie.name || h.tie.id);
  const take = takeRoute(st);
  const prot = take.ties.flatMap((t, i) => (t.protect ? [names[i]] : []));
  // ids name the chosen path (pathKey): a One-off tie line goes to the server as its ends, not its id.
  return { take, names, prot, ids: hops.map((h) => h.tie.id), seq: ++f.seq };
}

function ask(f, q, req, plans, all) {
  const at = (k) => req.at.indexOf(k);
  const preset = f.plan.verb === 'preset' ? `Preset ${f.plan.preset}` : '';
  const names = q.members.map((k) => numText(f.routes[k].item.dst));
  const prefix = (uid) => routerData(uid).prefix;
  const others = f.routes.some((r, i) => !q.members.includes(i) && !f.picks[i]?.cleared);
  const state = stateOf(f, q);
  f.reopen = null;
  pickRoute({
    routes: () => planRequest(f.routes, f.picks).routes,
    at: at(q.at),
    first: { route: plans[q.at], all },
    members: q.members.map((k) => ({ at: at(k), dst: f.routes[k].item.dst })),
    state,
    oneOffs: f.oneOffs,
    head: (st) => [preset, questionHead(q, chosen(st), names, prefix)].filter(Boolean).join(': '),
    final: finalAt(f.routes, f.picks, q),
    clear: others
      ? () => {
          for (const k of q.members) f.picks[k] = { cleared: true };
          next(f);
        }
      : null,
    clearText: q.members.length > 1 ? 'Clear these routes' : 'Clear this route',
    split:
      q.members.length > 1
        ? () => {
            f.split.push(q.key);
            next(f);
          }
        : null,
    onPick: (st, oneOffs) => answerQuestion(f, q, st, oneOffs),
    onClose: () => f.opts.onClose?.(),
  });
}

// answerQuestion puts the choice of question q on each of its routes. Route at gets its pick from the picker. A
// member whose choice reaches its destination gets its pick from a plan through the chosen tie lines. Thus its take
// has what each of its hops carries and its own Affected destinations.
async function answerQuestion(f, q, st, oneOffs) {
  f.oneOffs = oneOffs || [];
  f.protect = st.protect || [];
  const need = [];
  for (const { k, ids, full } of questionIds(q, chosen(st), f.routes)) {
    if (k === q.at && full) f.picks[k] = pickOf(f, st);
    else {
      f.picks[k] = { ids };
      if (full) need.push(k);
    }
  }
  if (need.length) {
    const { plans } = await askPlans(f);
    for (const k of need) {
      const ids = f.picks[k].ids;
      const route = plans?.[k];
      const i = route ? route.paths.findIndex((p) => pathKey(p) === ids.join(' ')) : -1;
      // No path through the choice: the route gets a question of its own.
      f.picks[k] = i < 0 ? undefined : pickOf(f, { route, path: i, protect: f.protect });
    }
  }
  next(f);
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
      // false: sendTakes neither notes the step nor clears the boxes. send does both once the server accepts.
      send: () => {
        send(f);
        return false;
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

async function send(f) {
  const { routes, at } = takeRequest(f.routes, f.picks);
  if (!routes.length) {
    showResult('info', 'Nothing to take', 'You cleared every route.');
    return f.opts.onClose?.();
  }
  const text = textOf(f, at);
  showResult('info', 'Taking', text);
  // A preset's take names it, for History and the preset right (tiepage.go).
  const preset = f.plan.verb === 'preset' ? { preset: f.plan.preset, presetId: f.plan.presetId || '' } : {};
  const m = await request('tieTakeResult', { t: 'tieTake', routes, ...preset });
  answer(f, m, text, at);
  // The take is sent here, not by sendTakes: the Redo list changes here (Step back, stepback.js). Only a take that the
  // server ran counts, one time. A refusal (tie_line_changed) asks the questions again, and its send is the same step.
  // The patch boxes stay until then too.
  if (takeRan(m)) {
    noteSent(f.plan);
    f.opts.onSent?.(f.plan);
  }
}

function answer(f, m, text, at) {
  const rs = m.body?.results;
  if (takeRan(m)) {
    // The destination entries are this tab's takes, for Ctrl+Z. The server sends each entry before this answer.
    noteOwn(
      rs.map((r) => r.entry).filter((id) => id != null),
      S.hist,
    );
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
// check, rule 5). The page says which tie line changed and asks the questions again. Each one chooses the same tie
// lines when they still make a path. The question of the route that changed marks what changed.
function reopen(f, body, at) {
  const k = changedAt(body, at);
  const r = body.routes?.find((x) => at[x.route] === k);
  const names = (r?.changed || []).map((t) => t.name || t.id);
  const fresh = r?.newAffected || [];
  const what = names.length ? `${names.join(', ')} changed` : fresh.length ? 'The Affected destinations changed' : 'A tie line changed';
  showResult('error', 'Tie line changed', `${what}. Sode sent nothing. Choose again.`);
  if (k < 0) return;
  f.prev = f.picks.map((p) => p?.ids);
  f.reopen = { k, ids: f.picks[k]?.ids, changed: (r?.changed || []).map((t) => t.id), fresh };
  f.picks = reopenPicks(f.picks);
  f.seq = 0;
  next(f);
}
