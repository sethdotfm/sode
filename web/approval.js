// The server checks the one message with `approval` against the approver's Role; nothing stays on this browser.
// send returns false when it could not send; the caller then calls answer.fail or answer.done.
// req: req.action() is the message the server holds for Request approval (approvalcards.js); req.onEnd hears the end.

import { el, on } from './core.js';
import { btn } from './formbits.js';
import { pinPrompt } from './pinprompt.js';
import { requestApproval, canApprove } from './approvalcards.js';
import { approverKey } from './approvalcardrules.js';

let ask = null; // { what, who, sendFn, req } of the open popup

const request = btn('pb-btn--outline', 'Request approval', sendRequest, 'approval-request');
const requestRow = el('div', '', request);
const prompt = pinPrompt({
  id: 'approval',
  title: 'Approval',
  go: 'Approve',
  name: { placeholder: 'Name', label: 'Name of the person who approves', missing: "Enter the approver's name." },
  pin: { placeholder: 'PIN', 'aria-label': 'PIN of the person who approves' },
  extra: [requestRow],
  send: (pin, user) => ask.sendFn({ user, pin }, { fail: prompt.fail, done: () => prompt.close() }),
  onClose: () => (ask = null),
  dropCloses: true,
});

function paintRequest() {
  if (!ask?.req) return;
  const ok = canApprove(approverKey(ask.who));
  request.disabled = !ok;
  if (ok) delete request.dataset.pbTip;
  else request.dataset.pbTip = 'No approver signed in';
}

async function sendRequest() {
  if (!ask?.req || request.disabled) return;
  const { what, who, sendFn, req } = ask;
  request.disabled = true;
  const err = await requestApproval(req.action(), { what, onEnd: req.onEnd, onTimeout: () => askApproval(what, who, sendFn, req) });
  if (ask?.req !== req) return;
  if (!err) return prompt.close();
  paintRequest();
  prompt.err.textContent = err;
  prompt.err.hidden = false;
}

// what names the action, for example 'Saving these changes'; who names the Role it needs.
export function askApproval(what, who, sendFn, req = null) {
  prompt.close();
  ask = { what, who, sendFn, req };
  requestRow.hidden = !req;
  paintRequest();
  prompt.open(`${what}: Approval from ${who}.`);
}

on('approvers', paintRequest);
