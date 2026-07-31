# Self-Hosted CI Runners

This fork's GitHub Actions run entirely on **self-hosted runners** (no
GitHub-hosted minutes). No GPU is required — the GL/rendering tests run headless
under `xvfb` + Mesa software rendering on Linux, and in a logged-in desktop
session on macOS/Windows.

## Runner inventory

| Runner | Platform | `runs-on` that reaches it | Runs |
|--------|----------|---------------------------|------|
| `bellui-linux-parallels` | Linux, arm64 | `[self-hosted, Linux, ARM64]` | Nearly everything: Static Analysis, Web Tests, Mobile Tests, X Submodule Tests (linux), `platform_tests (linux)`, `webgpu_tests (linux)`, CLI Tests (linux) + `cli_static_analysis` + `dex_regenerated_with_java`, Local GLFW Suite |
| `bellui-trader` | Linux, x86_64 | `[self-hosted, Linux, X64]` | `platform_tests (linux-x64)` — and nothing else; it is the only job in the repo targeting Linux X64 |
| `bellui-mini` | macOS, arm64 | `[self-hosted, macOS, ARM64]` | `platform_tests (macos)`, `webgpu_tests (macos)`, X Submodule Tests (macos), CLI Tests (macos) |
| `bellui-omen` | Windows, x86_64 | `[self-hosted, X64, Windows]` | `windows_tests`, CLI Tests (windows) |
| `bellui-win-parallels` | Windows, arm64 | `[self-hosted, ARM64, Windows]` | `windows_arm64_tests` |

Label matching is case-insensitive, so the capitalised `Linux`/`ARM64`/`X64` in
the workflows matches the lowercase labels GitHub assigns a runner by default.

**The Linux split is the part that surprises people.** Only one job in the whole
repo targets Linux **X64** — the `linux-x64` leg of Platform Tests, which exists
so amd64-specific breakage (SIMD, cgo struct layout, unsafe pointer arithmetic)
is caught. Every other Linux job targets **ARM64** and lands on
`bellui-linux-parallels`. A change that needs "the Linux runner" almost always
means that one.

None of these jobs is `continue-on-error`; a failure on any runner fails the
workflow. (`continue-on-error` appears exactly once in the repo, on the advisory
`buf breaking` step in `static_analysis.yml`.) In particular `windows_arm64_tests`
is **not** non-blocking, and is not a matrix leg of `windows_tests` — it is a
separate job on purpose, because matrixing would rename the x64 check and
silently drop it from any branch-protection rule naming it. See the comment above
that job in `platform_tests.yml`.

### Keeping this table honest

The names above are the ones GitHub actually reported, recovered from job
assignments rather than from memory — an earlier version of this file listed four
runners, got all four names wrong, and omitted `bellui-linux-parallels` (which
runs most of CI) entirely. Re-derive them the same way after any
re-registration:

```bash
# Every runner that picked up work in the last 12 runs of a workflow.
for id in $(gh run list --repo belluikit/bellui --workflow platform_tests.yml \
              --limit 12 --json databaseId --jq '.[].databaseId'); do
  gh api repos/belluikit/bellui/actions/runs/$id/jobs \
    --jq '.jobs[] | select(.runner_name != "") | "\(.runner_name)\t\(.name)"'
done | sort -u
```

`gh api repos/belluikit/bellui/actions/runners` is the direct answer but needs a
token with admin scope on the repo, which an ordinary contributor token does not
have — hence the roundabout route above.

### Hosts

Carried over from the previous version of this file and **not re-verified** —
job assignments report a runner's name, never its hardware. Correct anything
wrong here in passing:

| Runner | Host |
|--------|------|
| `bellui-linux-parallels` | unrecorded — the name suggests a Parallels VM, but nobody has confirmed it. Worth filling in: it runs most of CI |
| `bellui-trader` | `trader` (Linux, x86_64, 8c/31 GB) |
| `bellui-mini` | Mac mini (macOS, arm64) |
| `bellui-omen` | `windows_omen` (Windows 11, AMD64) |
| `bellui-win-parallels` | `windows_parallels` (Windows, arm64 VM) |

## Why each platform runs the way it does

- **Linux** runs fine headless: jobs wrap GUI tests in `xvfb-run -a` and the
  Mesa `libgl/egl/gles` packages provide software rendering. Runners are
  ordinary systemd services (`svc.sh install`), so they come back on reboot with
  no login.
- **macOS / Windows** native drivers (`no_glfw`) open **real windows**, so their
  runners must run in an **interactive logged-in desktop session**, not a
  headless service/`session 0`. `bellui-mini` runs `run.sh` (or a LaunchAgent) in
  a logged-in session; the two Windows runners each run from a Scheduled Task
  with `LogonType Interactive`.

  This is the single most common cause of a job sitting in `queued` forever: the
  box is up, but nobody is logged in, so the listener never started. `bellui-omen`
  and `bellui-win-parallels` are both affected. Symptom: the job stays `queued`
  with an empty `runner_name` while every Linux and macOS job on the same run
  finishes normally — see Diagnosing a stuck job below.

## Gotchas hit while setting these up (and the fixes)

These only surface on self-hosted runners — GitHub-hosted runners are fresh VMs
per job, so they never collide or inherit local config.

1. **Parallel Xvfb display collision** — multiple runners on one box fight over
   display `:99`. Fix: `xvfb-run -a` (auto-pick a free display number).
2. **dpkg lock contention** — parallel `apt-get` on one box. Fix:
   `apt-get -o DPkg::Lock::Timeout=600`.
3. **Windows: ssh-launched runner dies on disconnect** — OpenSSH kills child
   processes when the session closes. Fix: run via a Scheduled Task (Task
   Scheduler owns the process), not from the ssh session.
4. **Windows: `actions/checkout` fails with "Could not read from remote
   repository"** — the user's global git config had
   `url.ssh://git@github.com/trendvidia.insteadOf=https://github.com/trendvidia`,
   which rewrites the token-authed https clone to SSH. git 2.26 on that box
   predates `GIT_CONFIG_GLOBAL`, so the runner sets `HOME` to a private dir with
   a clean `.gitconfig` (the user's personal config is left untouched). The
   arm64 box has no such rewrite and needs no isolation.
5. **Windows: PowerShell ExecutionPolicy** — must be at least `RemoteSigned`
   (LocalMachine or CurrentUser) or the runner's generated step scripts fail
   with `PSSecurityException`. Set once per box.
6. **Windows: CGO needs a C compiler** — `-race` requires CGO. amd64 uses
   WinLibs UCRT gcc; arm64 uses LLVM-MinGW UCRT (provides `aarch64-w64-windows`
   `gcc`). Both installed via `winget`, on the runner user's PATH.

## Adding / re-registering a runner

```bash
# 1. Mint a registration token (needs repo admin)
gh api -X POST repos/belluikit/bellui/actions/runners/registration-token --jq .token

# 2. Download the runner for the platform, then:
./config.(sh|cmd) --url https://github.com/belluikit/bellui --token <TOKEN> \
  --labels self-hosted,<os>,<arch> --name <name> --work _work --unattended --replace

# 3a. Linux: install as a service
sudo ./svc.sh install $(whoami) && sudo ./svc.sh start

# 3b. macOS/Windows: run in the logged-in desktop session
#     macOS:   ./run.sh   (or a LaunchAgent)
#     Windows: a Scheduled Task, AtLogOn, LogonType Interactive, RunLevel Highest
```

## Operational notes

- **Reboot persistence:** the Windows/macOS desktop-session runners only come
  back after a user logs in. Enable **autologon** (e.g. Sysinternals
  `Autologon.exe`) on those boxes so they recover automatically after a reboot.
- **Runner auto-update:** the runners here were configured **without**
  `--disableupdate`, so the listener auto-updates itself when GitHub ships a new
  version (it restarts in place). No manual cadence needed unless a runner sits
  offline long enough to fall behind — then re-run the download+`config` step.
- **Capacity:** add more instances in separate directories on the same box for
  more parallelism. An earlier version of this file recorded three parallel
  Linux runners on `trader`; only `bellui-trader` has been observed taking work,
  but that proves nothing either way — it is the sole runner for Linux X64 and
  has exactly one job to run, so a second instance would rarely be needed.
  Confirm against the runner list before relying on the number.
- **Cleanup:** `actions/checkout` cleans the workspace each job, but `_work` and
  the Go build cache persist between jobs (intentional, for speed).

## Diagnosing a stuck job

A job sitting in `queued` while the rest of the run goes green means no runner
with its labels is connected — not that the job is slow. The run itself also
reports `queued`, so a workflow-level status is not enough to tell them apart.

```bash
# Which jobs are stuck, and which runner took the ones that are not.
gh api repos/belluikit/bellui/actions/runs/<RUN_ID>/jobs \
  --jq '.jobs[] | "\(.status)\t\(.conclusion // "-")\t\(.runner_name // "-")\t\(.name)"'
```

An empty `runner_name` on a job whose siblings all name a runner is the tell.
Check that the box is not merely powered on but has the listener running — for
`bellui-omen` and `bellui-win-parallels` that means somebody is logged in and the
Scheduled Task fired.

Rapid merges to `main` add a second, harmless-looking symptom: with
`cancel-in-progress: false`, GitHub keeps only **one** pending run per
concurrency group, so intermediate commits show `cancelled` across every
workflow while the newest waits behind a run that cannot finish. Cancelling the
older run releases the newest one — but only the head commit's result matters, so
that is usually the right move anyway.
