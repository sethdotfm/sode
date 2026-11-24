# Alpha bugs: multi-router build

Working list of bugs found on branch `alpha` (cut from `integration/overnight` at 7f281f87). Test setup: `.dev/run-alpha.sh` (local only, not in git) runs two swp08sim routers (A on :8910, B on :8911) and Sode on :8080. Elsewhere, run them by hand: `SIM_PORT=8910 go run ./cmd/swp08sim`, `SIM_PORT=8911 go run ./cmd/swp08sim`, then add router B in Settings > Routers.

Workflow:

1. **Capture.** Record each bug as reported, one entry at a time. Do not fix anything yet.
2. **Grill.** Go through the entries with the owner and fill in Expected, Decisions and Open questions until none is open.
3. **Hand off.** A cloud agent works the entries marked `ready`, in order. It follows AGENTS.md and makes one commit (or PR) per bug.

Status values: `captured` → `grilling` → `ready` → `in progress` → `done` (or `wontfix`).

---

## Template

### AB-00: short title

- **Status:** captured
- **Area:** (main page, Settings > Routers, tie lines, History, panels, API, …)
- **Reported:** what the owner saw, in their words
- **Steps:** how to reproduce on the two-router sim
- **Expected:**
- **Decisions:**
- **Open questions:**

---

## Bugs

<!-- New entries go below, numbered AB-01, AB-02, … -->
