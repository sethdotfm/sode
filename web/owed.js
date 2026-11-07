// One message the server may still owe us: a Lock or a sign out that wsSend refused (socket down). The
// slot holds one, so a later one replaces an earlier one. Pure; send is injected (wsSend returns false
// when refused).

// The server's answer says whether it still needs the message. A Lock is owed while the session answers
// unlocked; a sign out while the session still answers signed in or held.
const stillOwed = {
  userLock: (m) => !!m.user && !m.held,
  userSignOut: (m) => !!m.user || !!m.held,
};

export function createOwed(send) {
  let owed = '';
  return {
    owe(t) {
      owed = send({ t }) ? '' : t;
    },
    clear() {
      owed = '';
    },
    pending: () => owed,
    // Call with each lockState. True when it resent the message, so the caller skips the state.
    arrived(m) {
      if (owed && stillOwed[owed](m)) {
        this.owe(owed);
        return true;
      }
      owed = '';
      return false;
    },
  };
}
