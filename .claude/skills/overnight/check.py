#!/usr/bin/env python3
"""Overnight run gate. Run from the repo root.

Default: every open issue with the label has a LEDGER.md row, and skipped or
deferred rows give a reason. --stop also needs no todo or running rows and a
REPORT.md that names every skipped or deferred issue.
"""
import argparse
import json
import os
import re
import subprocess
import sys

STATUSES = {"todo", "running", "merged", "skipped", "deferred"}
# The last row for an issue wins, so a later table overrides an earlier one.
NEEDS_NOTE = {"skipped", "deferred"}


def open_labeled(label):
    out = subprocess.run(
        ["gh", "issue", "list", "--state", "open", "--label", label,
         "--limit", "1000", "--json", "number,title"],
        check=True, capture_output=True, text=True).stdout
    return {i["number"]: i["title"] for i in json.loads(out)}


def brief_date(n):
    out = subprocess.run(
        ["gh", "issue", "view", str(n), "--json", "comments,body"],
        check=True, capture_output=True, text=True).stdout
    d = json.loads(out)
    dates = [c["createdAt"][:10] for c in d["comments"] if "## Agent Brief" in c["body"]]
    return dates[-1] if dates else ("body" if "## Agent Brief" in d["body"] else None)


def ledger_rows(path):
    cols, rows = None, {}
    for line in open(path, encoding="utf-8"):
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if not line.lstrip().startswith("|"):
            continue
        # A ledger may hold several tables (phase 3 adds one); each header resets the columns.
        if cells[0] == "#":
            cols = cells if "status" in cells else None
            continue
        m = re.fullmatch(r"#(\d+)", cells[0])
        if not m or cols is None:
            continue
        row = dict(zip(cols, cells))
        rows.setdefault(int(m.group(1)), []).append(row)
    if not rows:
        sys.exit(f"{path}: no rows under a table with '#' and 'status' columns")
    return rows


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dir", default=".scratch/overnight")
    ap.add_argument("--label", default="ready-for-agent")
    ap.add_argument("--stop", action="store_true")
    a = ap.parse_args()

    ledger = os.path.join(a.dir, "LEDGER.md")
    if not os.path.exists(ledger):
        sys.exit(f"missing {ledger}")
    rows = ledger_rows(ledger)
    errs, dropped = [], []

    want = open_labeled(a.label)
    for n in sorted(set(want) - set(rows)):
        errs.append(f"#{n} is open with {a.label} but has no ledger row: {want[n]}")

    for n, rs in sorted(rows.items()):
        st =rs[-1].get("status", "").strip("`").lower()
        if st not in STATUSES:
            errs.append(f"#{n} status {st!r} is not one of {sorted(STATUSES)}")
        note = rs[-1].get("notes", "")
        if st in NEEDS_NOTE and not note:
            errs.append(f"#{n} is {st} with no reason in notes")
        # A stale "needs your decision" once dropped a briefed issue; citing the brief proves it was read.
        if st in NEEDS_NOTE and n in want:
            bd = brief_date(n)
            dropped.append(f"#{n} {st}: {note or '(no reason)'}" + (f" [brief {bd}]" if bd else ""))
            if bd and f"brief {bd}" not in note:
                errs.append(f"#{n} is {st} but has an Agent Brief ({bd}) its notes don't cite. "
                            f"Read it, then put 'brief {bd}' and the reason in notes")

    if a.stop:
        for n, rs in sorted(rows.items()):
            st = rs[-1].get("status", "").strip("`").lower()
            if st in {"todo", "running"}:
                errs.append(f"#{n} is still {st}")
        report = os.path.join(a.dir, "REPORT.md")
        if not os.path.exists(report):
            errs.append(f"missing {report}")
        else:
            text = open(report, encoding="utf-8").read()
            for n, rs in sorted(rows.items()):
                st = rs[-1].get("status", "").strip("`").lower()
                if st in NEEDS_NOTE and not re.search(rf"#{n}\b", text):
                    errs.append(f"#{n} is {st} but REPORT.md does not name it")

    for d in dropped:
        print("not built:", d)
    for e in errs:
        print(e)
    if errs:
        sys.exit(1)
    print(f"ok: {len(rows)} ledger rows, {len(want)} open {a.label} issues covered")


if __name__ == "__main__":
    main()
