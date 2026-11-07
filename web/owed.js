// A Lock that wsSend refused (socket down), sent again when a lockState shows the session still
// unlocked. Sign out is not kept: signOutNow clears the stored tokens first, so the next hello
// is signed out anyway. Pure; send is injected (wsSend returns false when refused).

export function createOwed(send) {
  let owed = false;
  const owe = () => {
    owed = !send({ t: 'userLock' });
  };
  return {
    owe,
    // Call with each lockState. True when it resent the Lock, so the caller skips the state.
    arrived(m) {
      if (owed && !!m.user && !m.held) {
        owe();
        return true;
      }
      owed = false;
      return false;
    },
  };
}
