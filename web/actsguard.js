// The one guard on an Authentication mode switch. Every caller that changes a panel's acts (today only the editor's
// Access tab; an Assign never writes acts) goes through it, so a switch that makes or ends the client never applies
// without the Confirm.
import { actsSwitch } from './panelsrules.js';

// box is a confirmBox (confirmbox.js). Resolves true when the switch may apply.
export async function guardActs(box, from, to) {
  const s = actsSwitch(from, to);
  if (!s?.confirm) return true;
  return box.ask(s.text, `Switch to ${s.name}`);
}
