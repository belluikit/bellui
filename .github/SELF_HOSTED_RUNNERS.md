# Self-Hosted CI Runners

This fork's GitHub Actions run entirely on **self-hosted runners** (no
GitHub-hosted minutes). No GPU is required — the GL/rendering tests run headless
under `xvfb` + Mesa software rendering on Linux, and in a logged-in desktop
session on macOS/Windows.

## Runner inventory

| Runner | Host | Labels | Runs | How it runs |
|--------|------|--------|------|-------------|
| `trader-bellui`, `-2`, `-3` | `trader` (Linux, x86_64, 8c/31 GB) | `self-hosted, linux, x64` | Static Analysis, Web Tests, Mobile Tests, Linux Platform Tests | 3× systemd services (`svc.sh install`), 3-way parallelism |
| `mac-arm64-mini` | Mac mini (macOS, arm64) | `self-hosted, macOS, arm64` | macOS Platform Tests | `run.sh` in a logged-in session |
| `win-amd64-omen` | `windows_omen` (Windows 11, AMD64) | `self-hosted, windows, x64` | Windows Tests (required) | Scheduled task in the desktop session |
| `win-arm64-parallels` | `windows_parallels` (Windows, arm64 VM) | `self-hosted, windows, arm64` | Windows Tests (arm64, **non-blocking**) | Scheduled task in the desktop session |

Workflow `runs-on` targets these labels. The Windows matrix has an arm64 leg
gated with `continue-on-error` because it lives on a Parallels VM that may be
offline.

## Why each platform runs the way it does

- **Linux** runs fine headless: jobs wrap GUI tests in `xvfb-run -a` and the
  Mesa `libgl/egl/gles` packages provide software rendering. Runners are
  ordinary systemd services.
- **macOS / Windows** native drivers (`no_glfw`) open **real windows**, so their
  runners must run in an **interactive logged-in desktop session**, not a
  headless service/`session 0`. On Windows this is a Scheduled Task with
  `LogonType Interactive`.

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
- **Capacity:** `trader` runs 3 parallel Linux runners; add more instances in
  separate directories for more parallelism. macOS/Windows are single runners.
- **Cleanup:** `actions/checkout` cleans the workspace each job, but `_work` and
  the Go build cache persist between jobs (intentional, for speed).
