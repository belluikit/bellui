# Bellui session protocol

- Read the `docs/notes/` file for each area before changing it; the repository
  guide's Fork Conventions are binding.
- Verify before committing: `gofumpt -d -e .` and `goimports -e -d .` clean on
  changed files; `go vet ./...`; scoped `go test -race -tags ci` for the
  affected packages plus anything that imports them (the `x/` submodules are
  separate Go modules — cd into them). Run staticcheck as
  `go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 ./...` — the locally
  installed binary is too old for this Go toolchain.
- Follow-up issues you file are added to the backlog project by hand at
  triage; nothing adds them automatically. Put suggested
  Workstream/Priority/Tranche in the issue body so triage can set the fields.
- A platform-bridge change is not verified by unit tests alone (the repository
  guide: live-verify harnesses in `hack/*probe`). If your change touches a platform
  bridge and you cannot run the matching probe on the required
  device/GPU/session, say so explicitly in the PR body instead of claiming
  verification.
