#!/usr/bin/env python3
"""Render the live bellui backlog from the Bellui Backlog GitHub project
(trendvidia/projects/2 by default; override with PLAN_PROJECT): startable
work first, by priority within tranche, with blocked items listed against
their open blockers (native GitHub blocked-by edges).

The project answers what/when; the issue body answers how.
Needs: gh (authenticated; project + repo scopes).
"""
import json
import os
import re
import subprocess
import sys

PROJECT = os.environ.get("PLAN_PROJECT", "2")
EPIC_RE = re.compile(r"^\[?Epic[\]:]")
TRANCHE_ORDER = {"Now": 0, "Next": 1, "Later": 2, "Parked": 3}
PRIO = {"P0": 0, "P1": 1, "P2": 2}


def gh(*args):
    r = subprocess.run(["gh", *args], capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit("gh %s failed: %s" % (" ".join(args[:3]), r.stderr.strip()))
    return json.loads(r.stdout) if r.stdout.strip() else None


def blockers_of(repo, num):
    got = gh("api", "repos/%s/issues/%s/dependencies/blocked_by" % (repo, num)) or []
    return [{"ref": b["repository"]["full_name"] + "#" + str(b["number"]),
             "state": b["state"]} for b in got]


# --limit must stay ahead of the board size: gh silently truncates past it,
# which made freshly-triaged items vanish from the plan once the board passed
# 200 items. Fail loudly when we hit the ceiling instead.
items = gh("project", "item-list", PROJECT, "--owner", "trendvidia",
           "--format", "json", "--limit", "1000")["items"]
if len(items) >= 1000:
    sys.exit("board has >= 1000 items; raise the --limit in plan.py")

rows = []
for it in items:
    c = it.get("content") or {}
    if not c.get("number"):
        continue
    repo, num = c.get("repository", ""), c["number"]
    # `gh project item-list` does not expose the issue's open/closed state;
    # the project's built-in workflow sets Status=Done when an issue closes,
    # so that field is the done signal. Done items are dropped from the plan
    # and skip the per-item blockers lookup (one API call each).
    done = it.get("status") == "Done"
    open_blockers = [] if done else [
        d["ref"] for d in blockers_of(repo, num) if d["state"] == "open"]
    rows.append({
        "ref": ("%s#%s" % (repo, num)).replace("trendvidia/", ""),
        "title": c.get("title", ""),
        "done": done,
        "epic": bool(EPIC_RE.match(c.get("title", ""))),
        "prio": it.get("priority"), "tranche": it.get("tranche") or it.get("wave"),
        "ws": it.get("workstream"), "status": it.get("status"),
        "blockers": [b.replace("trendvidia/", "") for b in open_blockers],
    })


def line(r, extra=""):
    bits = " · ".join(b for b in (r["prio"], r["tranche"], r["ws"], r["status"]) if b)
    return "- **%s** %s  \n  %s%s" % (r["ref"], r["title"], bits, extra)


def key(r):
    return (TRANCHE_ORDER.get(r["tranche"], 4), PRIO.get(r["prio"], 3), r["ref"])


open_rows = [r for r in rows if not r["done"] and not r["epic"]]
startable = sorted((r for r in open_rows if not r["blockers"] and r["tranche"] != "Parked"), key=key)
blocked = sorted((r for r in open_rows if r["blockers"]), key=key)
parked = sorted((r for r in open_rows if not r["blockers"] and r["tranche"] == "Parked"), key=key)
done_count = sum(r["done"] for r in rows)

print("# Backlog — live from trendvidia/projects/%s\n" % PROJECT)
print("## Startable now (unblocked, by tranche then priority)\n")
print("\n".join(line(r) for r in startable) or "(nothing — check Blocked)")
print("\n## Blocked\n")
print("\n".join(line(r, "  \n  BLOCKED BY: " + ", ".join(r["blockers"]))
                for r in blocked) or "(none)")
if parked:
    print("\n## Parked\n")
    print("\n".join(line(r) for r in parked))
if done_count:
    print("\n(%d done items hidden)" % done_count)
