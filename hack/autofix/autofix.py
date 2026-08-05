#!/usr/bin/env python3
"""Unattended backlog fixer for the Bellui Backlog GitHub project
(trendvidia/projects/2 by default; override with PLAN_PROJECT).

Pipeline:
  1. fetch    — project items (same source as hack/plan/plan.py) plus per-issue
                details (createdAt, labels, body) for the survivors
  2. filter   — deterministic: this repo only, open, real issues (no epics,
                no Parked, no linked PRs, no open blockers), then the cutoffs:
                --before DATE drops issues created after DATE, --max-issue N
                drops issues numbered above N
  3. triage   — one claude call marks issues that need a human decision (or
                cannot be completed autonomously) and groups the rest by
                similarity: one group == one session == one PR
  4. execute  — per group: git worktree off origin/<default branch>, headless
                `claude -p --dangerously-skip-permissions` implements the fix
                with unit + functional tests, applies the CLAUDE.md follow-up
                rule (files discovered work as issues before the PR), commits,
                pushes and opens the PR; the script then verifies the PR exists

Needs: gh (keyring-authenticated with project + repo scopes) and claude on
PATH. GH_TOKEN/GITHUB_TOKEN are stripped from the environment for every child
process — an env token shadows the keyring token that carries read:project.
CGO_ENABLED is stripped too so sessions can cross-compile (wasm legs).

Typical use:
  hack/autofix/autofix.sh --list-only                 # what would be considered
  hack/autofix/autofix.sh --before 2026-08-01 --dry-run
  hack/autofix/autofix.sh --max-issue 280 --max-groups 2
"""
import argparse
import datetime
import json
import os
import re
import subprocess
import sys
import time

PROJECT = os.environ.get("PLAN_PROJECT", "2")
OWNER = "trendvidia"
EPIC_RE = re.compile(r"^\[?Epic[\]:]")
PRIO = {"P0": 0, "P1": 1, "P2": 2}
TRIAGE_BODY_LIMIT = 3500   # chars of issue body shown to the triage call
FIX_BODY_LIMIT = 12000     # chars of issue body inlined into a fix prompt

# Env for every child process: env GH tokens shadow the keyring token that has
# read:project; the interactive shell exports CGO_ENABLED=1 which breaks
# cross/wasm builds inside fix sessions.
ENV = {k: v for k, v in os.environ.items()
       if k not in ("GH_TOKEN", "GITHUB_TOKEN", "CGO_ENABLED")}


def note(msg):
    print(msg, file=sys.stderr, flush=True)


def run(cmd, **kw):
    kw.setdefault("env", ENV)
    kw.setdefault("text", True)
    return subprocess.run(cmd, **kw)


def gh(*args):
    r = run(["gh", *args], capture_output=True)
    if r.returncode != 0:
        sys.exit("gh %s failed: %s" % (" ".join(args[:3]), r.stderr.strip()))
    return json.loads(r.stdout) if r.stdout.strip() else None


def open_blockers(repo, num):
    got = gh("api", "repos/%s/issues/%s/dependencies/blocked_by" % (repo, num)) or []
    return ["%s#%s" % (b["repository"]["full_name"], b["number"])
            for b in got if b["state"] == "open"]


# ---------------------------------------------------------------- fetch/filter

def candidates(args):
    """Deterministically filtered board items, with issue details attached."""
    items = gh("project", "item-list", PROJECT, "--owner", OWNER,
               "--format", "json", "--limit", "1000")["items"]
    if len(items) >= 1000:
        sys.exit("board has >= 1000 items; raise the --limit in autofix.py")

    skipped, picked = [], []
    for it in items:
        c = it.get("content") or {}
        if not c.get("number") or c.get("type") != "Issue":
            continue
        repo, num = c.get("repository", ""), c["number"]
        if repo != args.repo or it.get("status") == "Done":
            continue
        title = c.get("title", "")
        tranche = it.get("tranche") or it.get("wave")
        if EPIC_RE.match(title) or tranche == "Parked":
            continue
        if num in args.exclude:
            skipped.append((num, title, "--exclude"))
            continue
        if it.get("linked pull requests"):
            skipped.append((num, title, "already has linked PR(s)"))
            continue
        picked.append({"repo": repo, "number": num, "title": title,
                       "tranche": tranche, "prio": it.get("priority"),
                       "ws": it.get("workstream")})

    note("board: %d items, %d candidates before cutoffs" % (len(items), len(picked)))

    kept = []
    for it in picked:
        if args.max_issue and it["number"] > args.max_issue:
            skipped.append((it["number"], it["title"], "number > --max-issue"))
            continue
        d = gh("issue", "view", str(it["number"]), "--repo", it["repo"], "--json",
               "number,title,body,createdAt,labels,state")
        if d["state"] != "OPEN":
            continue
        created = datetime.datetime.strptime(
            d["createdAt"], "%Y-%m-%dT%H:%M:%SZ").date()
        if args.before and created > args.before:
            skipped.append((it["number"], it["title"],
                            "created %s > --before" % created))
            continue
        it["created"] = str(created)
        it["labels"] = [lb["name"] for lb in d["labels"]]
        it["body"] = d["body"] or ""
        kept.append(it)

    unblocked = []
    for it in kept:
        blockers = open_blockers(it["repo"], it["number"])
        if blockers:
            skipped.append((it["number"], it["title"],
                            "blocked by " + ", ".join(blockers)))
        else:
            unblocked.append(it)

    unblocked.sort(key=lambda r: (PRIO.get(r["prio"], 3), r["number"]))
    return unblocked, skipped


# --------------------------------------------------------------------- triage

TRIAGE_PROMPT = """\
You are triaging backlog issues for unattended automated fixing. Below is a
JSON array of open, unblocked issues from %(repo)s. Decide for each issue
whether an autonomous coding agent can complete it WITHOUT a human decision.
Exclude an issue when any of these hold:
- the body leaves a design or product choice open ("decide", "should we",
  "open question", competing options with none chosen)
- the spec is too vague to derive acceptance criteria from
- it primarily needs hardware, credentials, manual/on-device verification,
  or coordination outside this repository
- it is a release, process, or tracking item rather than a code change

Group the remaining issues so each group fits ONE focused coding session and
ONE pull request: group only issues that touch the same subsystem/files or
share a root cause; otherwise keep them separate. Prefer 1-3 issues per
group; never more than 4.

Reply with ONLY a JSON object — no markdown fences, no commentary:
{"excluded":[{"number":<int>,"reason":"<short>"}],
 "groups":[{"slug":"<kebab-case, max 40 chars>","issues":[<int>,...],
            "rationale":"<why these belong together>"}]}
Every input issue number must appear exactly once across excluded and groups.
Do not use any tools; answer from the data below alone.

Issues:
%(issues)s
"""


def claude_json(prompt, model):
    r = run(["claude", "-p", "--model", model, "--output-format", "json"],
            input=prompt, capture_output=True)
    if r.returncode != 0:
        sys.exit("claude triage failed: %s" % r.stderr.strip()[:2000])
    envelope = json.loads(r.stdout)
    if envelope.get("is_error"):
        sys.exit("claude triage errored: %s" % envelope.get("result"))
    text = envelope.get("result", "").strip()
    if text.startswith("```"):
        text = re.sub(r"^```[a-z]*\s*|\s*```$", "", text)
    start, end = text.find("{"), text.rfind("}")
    return json.loads(text[start:end + 1])


def triage(cands, args):
    brief = [{"number": c["number"], "title": c["title"], "priority": c["prio"],
              "tranche": c["tranche"], "workstream": c["ws"],
              "labels": c["labels"], "created": c["created"],
              "body": c["body"][:TRIAGE_BODY_LIMIT] +
                      (" …[truncated]" if len(c["body"]) > TRIAGE_BODY_LIMIT else "")}
             for c in cands]
    prompt = TRIAGE_PROMPT % {"repo": args.repo,
                              "issues": json.dumps(brief, indent=1)}
    want = {c["number"] for c in cands}
    err = ""
    for attempt in range(2):
        got = claude_json(prompt + err, args.triage_model)
        excluded = got.get("excluded", [])
        groups = got.get("groups", [])
        seen = [e["number"] for e in excluded] + \
               [n for g in groups for n in g["issues"]]
        if sorted(seen) == sorted(want):
            break
        err = ("\n\nYour previous answer did not cover every issue exactly "
               "once (expected %s, got %s). Reply again, JSON only." %
               (sorted(want), sorted(seen)))
    else:
        sys.exit("triage output never covered all issues; raw: %s" % got)

    by_num = {c["number"]: c for c in cands}
    used = set()
    for i, g in enumerate(groups):
        slug = re.sub(r"-+", "-", re.sub(r"[^a-z0-9-]", "-",
                      (g.get("slug") or "").lower())).strip("-")[:40]
        g["slug"] = slug if slug and slug not in used else "group-%d" % (i + 1)
        used.add(g["slug"])
        g["items"] = [by_num[n] for n in g["issues"]]
    groups.sort(key=lambda g: min(
        (PRIO.get(it["prio"], 3), it["number"]) for it in g["items"]))
    return excluded, groups


# -------------------------------------------------------------------- execute

FIX_PROMPT = """\
You are working unattended in a dedicated git worktree of %(repo)s on branch
%(branch)s, forked from origin/%(base)s. Fix the following backlog issue(s)
end to end in this one session. The issue body is the work spec; the full
bodies are inlined below, and `gh issue view N --repo %(repo)s --comments`
has any later discussion.

%(issues)s

Protocol — follow CLAUDE.md (loaded from the worktree) plus:
1. Read the docs/notes/ file for each area before changing it; follow the
   Fork Conventions.
2. Add unit tests, and functional/behavioral tests (test.Assert* helpers,
   markup or image assertions, interaction tests) where possible.
3. Verify before committing: gofumpt -d -e . and goimports -e -d . are clean
   on changed files; go vet ./...; scoped `go test -race -tags ci` for the
   affected packages plus anything that imports them (x/ submodules are
   separate modules — cd into them). Staticcheck: use
   `go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 ./...` — the locally
   installed staticcheck binary is stale.
4. No silent tech debt (CLAUDE.md follow-up rule): discovered work is either
   done here or filed NOW with `gh issue create --repo %(repo)s` before the PR
   opens — never a TODO comment or a "later" note. Put suggested
   Workstream/Priority/Tranche in the new issue body (the add-to-project
   workflow picks it up) and cross-link it from the PR body.
5. Commit with messages referencing the issue number(s), e.g.
   "widget: fix X (#N)".
6. Push the branch (`git push -u origin %(branch)s`) and open ONE pull request
   for this group: `gh pr create --repo %(repo)s --base %(base)s`. The PR body
   must contain "Fixes #N" for each completed issue, summarize the change,
   list the tests added, and list any follow-up issues filed.
7. If an issue in the group turns out to need a human decision or hardware you
   do not have, do NOT guess: leave that issue out (no "Fixes" line for it),
   complete the rest, and explain the omission in the PR body. If nothing in
   the group is completable, do not open a PR and make your final line
   "AUTOFIX-ABORT: <reason>".

Make the final line of your reply the PR URL alone (or the AUTOFIX-ABORT line).
"""


def fix_prompt(group, args):
    blocks = []
    for it in group["items"]:
        body = it["body"][:FIX_BODY_LIMIT] + \
               (" …[truncated — read the rest with gh issue view]"
                if len(it["body"]) > FIX_BODY_LIMIT else "")
        blocks.append("## %s#%d — %s\n(%s)\n\n%s" % (
            it["repo"], it["number"], it["title"],
            " · ".join(x for x in (it["prio"], it["tranche"], it["ws"]) if x),
            body))
    return FIX_PROMPT % {"repo": args.repo, "branch": "autofix/" + group["slug"],
                         "base": args.base, "issues": "\n\n".join(blocks)}


def execute(groups, args, root, run_dir):
    run(["git", "-C", root, "fetch", "origin", args.base], capture_output=True)
    results = []
    for g in groups:
        branch = "autofix/" + g["slug"]
        nums = ", ".join("#%d" % n for n in g["issues"])
        if run(["git", "-C", root, "ls-remote", "--exit-code", "origin",
                "refs/heads/" + branch], capture_output=True).returncode == 0:
            results.append((g, "SKIPPED", "branch %s already on origin" % branch))
            continue
        wt = os.path.join(run_dir, "wt", g["slug"])
        log = os.path.join(run_dir, "logs", g["slug"] + ".log")
        r = run(["git", "-C", root, "worktree", "add", wt, "-b", branch,
                 "origin/" + args.base], capture_output=True)
        if r.returncode != 0:
            results.append((g, "FAILED", "worktree add: " + r.stderr.strip()))
            continue
        note("→ %s (%s) — session started, log: %s" % (g["slug"], nums, log))
        t0 = time.time()
        out, errout, status = "", "", "FAILED"
        try:
            r = run(["claude", "-p", "--model", args.fix_model,
                     "--dangerously-skip-permissions"],
                    input=fix_prompt(g, args), capture_output=True, cwd=wt,
                    timeout=args.timeout_min * 60 or None)
            out, errout = r.stdout, r.stderr
        except subprocess.TimeoutExpired as e:
            out = (e.output or b"").decode("utf-8", "replace") \
                if isinstance(e.output, bytes) else (e.output or "")
            errout = "TIMEOUT after %d min" % args.timeout_min
        with open(log, "w") as f:
            f.write(out + ("\n--- stderr ---\n" + errout if errout else ""))
        prs = gh("pr", "list", "--repo", args.repo, "--head", branch,
                 "--json", "url")
        detail = ""
        if prs:
            status, detail = "PR", prs[0]["url"]
            if run(["git", "-C", wt, "status", "--porcelain"],
                   capture_output=True).stdout.strip() == "":
                run(["git", "-C", root, "worktree", "remove", wt],
                    capture_output=True)
            else:
                detail += "  (worktree dirty, kept: %s)" % wt
        else:
            last = [l for l in out.strip().splitlines() if l.strip()]
            detail = (last[-1] if last else errout.strip()[:200] or
                      "no output") + "  (worktree kept: %s)" % wt
        note("← %s: %s in %dm" % (g["slug"], status, (time.time() - t0) / 60))
        results.append((g, status, detail))
    return results


# ----------------------------------------------------------------------- main

def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--before", type=lambda s: datetime.datetime.strptime(
        s, "%Y-%m-%d").date(), metavar="YYYY-MM-DD",
        help="drop issues created after this date (created on it are kept)")
    ap.add_argument("--max-issue", type=int, metavar="N",
                    help="drop issues numbered above N")
    ap.add_argument("--repo", help="issues repo (default: this checkout's)")
    ap.add_argument("--exclude", default="", metavar="N,M",
                    help="issue numbers to skip outright")
    ap.add_argument("--list-only", action="store_true",
                    help="stop after deterministic filtering (no model calls)")
    ap.add_argument("--dry-run", action="store_true",
                    help="triage and group, but run no fix sessions")
    ap.add_argument("--max-groups", type=int, default=0, metavar="N",
                    help="run at most N groups (0 = all)")
    ap.add_argument("--triage-model", default="claude-fable-5")
    ap.add_argument("--fix-model", default="claude-fable-5")
    ap.add_argument("--timeout-min", type=int, default=90, metavar="MIN",
                    help="per-session wall clock cap (0 = none, default 90)")
    ap.add_argument("--work-dir", default=os.path.expanduser("~/.bellui-autofix"),
                    help="worktrees + logs live under here")
    args = ap.parse_args()
    args.exclude = {int(n) for n in args.exclude.split(",") if n.strip()}

    root = run(["git", "-C", os.path.dirname(os.path.abspath(__file__)),
                "rev-parse", "--show-toplevel"],
               capture_output=True).stdout.strip()
    if not args.repo:
        args.repo = gh("repo", "view", "--json", "nameWithOwner")["nameWithOwner"]
    args.base = gh("repo", "view", args.repo, "--json",
                   "defaultBranchRef")["defaultBranchRef"]["name"]

    cands, skipped = candidates(args)
    print("# Autofix — %s, %d candidate issue(s)\n" % (args.repo, len(cands)))
    for c in cands:
        print("- **#%d** %s  (%s)" % (c["number"], c["title"], " · ".join(
            x for x in (c["prio"], c["tranche"], c["ws"], c["created"]) if x)))
    if skipped:
        print("\n## Skipped deterministically\n")
        for num, title, why in sorted(skipped):
            print("- #%d %s — %s" % (num, title, why))
    if args.list_only or not cands:
        return

    note("\ntriaging %d issues with %s ..." % (len(cands), args.triage_model))
    excluded, groups = triage(cands, args)
    print("\n## Triaged out (needs a human)\n")
    for e in sorted(excluded, key=lambda e: e["number"]):
        print("- #%d — %s" % (e["number"], e["reason"]))
    print("\n## Groups (one session + PR each)\n")
    for g in groups:
        print("- **%s**: %s — %s" % (
            g["slug"], ", ".join("#%d" % n for n in g["issues"]), g["rationale"]))
    if args.max_groups:
        groups = groups[:args.max_groups]
        print("\n(--max-groups: running the first %d)" % len(groups))
    if args.dry_run:
        print("\n(dry run — no sessions started)")
        return

    run_dir = os.path.join(args.work_dir, time.strftime("%Y%m%d-%H%M%S"))
    os.makedirs(os.path.join(run_dir, "logs"), exist_ok=True)
    os.makedirs(os.path.join(run_dir, "wt"), exist_ok=True)
    print("\nrun dir: %s\n" % run_dir)
    results = execute(groups, args, root, run_dir)

    print("\n## Results\n")
    for g, status, detail in results:
        print("- **%s** (%s): %s — %s" % (
            g["slug"], ", ".join("#%d" % n for n in g["issues"]), status, detail))
    print("\nlogs: %s" % os.path.join(run_dir, "logs"))
    if any(s != "PR" for _, s, _ in results):
        sys.exit(1)


if __name__ == "__main__":
    main()
