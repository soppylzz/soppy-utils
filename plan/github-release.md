# githubRelease

## Demands

Build an engineering-grade CI/CD pipeline on GitHub Actions. CI lints and tests the project on every change and computes a *suggested* next version; CD lets the user decide between that suggestion and an explicit major/minor/patch bump, then tags, commits, and publishes a GitHub Release. The package is pure zsh scripts — no build artifacts and no registry publishing — so the release artifact is the source tree itself. The infrastructure uses established GitHub Actions tools plus small zsh scripts for project-specific logic.

**Current state**

- `scripts/lint.sh` runs `zsh-lint` over `bin/`, `src/`, and `scripts/`; exits non-zero on any diagnostics or when `zsh-lint` is missing from `$PATH`.
- `scripts/test.sh` discovers `test/**/*.spec.sh` and runs each with `zsh`; exits non-zero if any spec fails.
- `scripts/install.sh` installs from a local clone, `git clone`, or a tarball of `main`.
- No git tags exist yet; default branch is `main`. The project has no build step — the distributable is the source tree itself, and GitHub auto-attaches `Source code (zip/tar.gz)` to a release.

**Answers to the open questions**

- **Do lint/test need to emit output values for CI to judge success?** No. Both scripts already exit non-zero on failure, and a GitHub Actions step fails when its command exits non-zero. The exit code *is* the signal; no `$GITHUB_OUTPUT`/`::set-output` values are needed for pass/fail. Step outputs are reserved for carrying *data* between jobs — here, the computed version inside CD.
- **How is the version decided?** In two phases, split between CI and CD:
  1. **CI suggests.** After lint/test pass on `main`, CI runs the versioning logic and surfaces the *suggested* next version (from Conventional Commits since the last tag) in the workflow summary — informational, non-blocking.
  2. **CD lets the user decide.** The release workflow is manually dispatched with a `bump` input: `auto` (use the suggestion) or an explicit `major`/`minor`/`patch` override. The user reviews CI's suggestion and either accepts it or forces a bump.- **Does creating the tag re-trigger the pipeline (recursion)?** No, handled two ways:
  1. The release workflow pushes the commit and tag using the built-in `GITHUB_TOKEN`; GitHub does **not** fire new workflow runs for events produced by `GITHUB_TOKEN`, so the tag push cannot recurse.
  2. Belt-and-suspenders: CI's `push` trigger is scoped to `branches: [main]` (or `tags-ignore: ['**']`), so tag pushes never enter CI regardless.
- **What other infra is needed?** Two workflow files, a `scripts/version.sh`, minimal `contents: write` permission on the release workflow, and a tag convention (`vX.Y.Z`). No secrets or PAT are required for the default public-repo flow.

**Target state — CI (`ci.yml`)**

- Triggers on `push` (branches: `main`) and `pull_request`, with `paths-ignore` for pure-doc changes; tag pushes are excluded.
- A matrix over `macos-latest` and `ubuntu-latest` (the tools target macOS, but scripts/tests must stay portable).
- Jobs: **lint** and **test**, each calling the project's own `scripts/lint.sh` / `scripts/test.sh` so CI stays in sync with local runs.
  - Failures surface as non-zero exit codes → the workflow goes red, blocking release.
- A **suggest** step (on `push` to `main` only) runs `scripts/version.sh` and writes the suggested next version to the job summary. It does not gate anything.

**zsh-lint toolchain wiring (Go package)**

`zsh-lint` is a standalone Go binary (module `github.com/z-shell/zsh-lint`, command subdirectory `cmd/zsh-lint`), so CI must set up a Go toolchain and install it before `scripts/lint.sh` runs. The exact wiring:

1. `actions/setup-go` with a pinned `go-version` (the exact minimum Go required by `zsh-lint` is confirmed against its `go.mod` at implementation time). `setup-go` also prepends `$(go env GOPATH)/bin` to `$PATH`.
2. `go install github.com/z-shell/zsh-lint/cmd/zsh-lint@latest` compiles the Go source and places the binary named `zsh-lint` (the last path segment of the `cmd/` dir) into `$(go env GOPATH)/bin`.
3. Because `setup-go` already added that bin dir to `$PATH`, `scripts/lint.sh`'s `command -v zsh-lint` check succeeds with no further wiring — no `GOBIN` override, no manual `$PATH` edit, and no change to `lint.sh` itself.

Notes:

- Use `@latest` for the install version (simple, matches `lint.sh`'s hint); accepting that it may drift.
- `zsh-lint-survey` (the companion binary) is **not** installed — it only reports parser gaps, not lint rules, and CI runs the real `zsh-lint`.
- `scripts/lint.sh`'s existing `command -v zsh-lint` guard doubles as the CI failure signal: if install is skipped or fails, `lint.sh` exits non-zero and the lint job goes red.
- This wiring lives in the workflow file (project-agnostic CI infra), while `scripts/lint.sh` keeps only the local-development hint — consistent with the rule that `zsh-lint` setup never lives under `bin/`.

**Target state — CD (`cd.yml`)**

- Triggers on `workflow_dispatch` with a `bump` choice input: `auto` (default) / `major` / `minor` / `patch`. Manual dispatch is the "hand the decision to the user" point.
- Guard: runs only on `main` (`if: github.ref == 'refs/heads/main'`). CI's green status is **trusted** — the user dispatches a release only after CI passes; CD does not re-check CI via `gh`.
- Permissions: `contents: write` (create tag + release).
- Steps:
  1. Checkout with full history and tags (`fetch-depth: 0`).
  2. Resolve the bump: `auto` → `scripts/version.sh` computes from Conventional Commits; `major`/`minor`/`patch` → force that bump.
  3. No-op when `auto` yields no releasable change (`bump=none`), instead of failing.
  4. Create the `vX.Y.Z` tag on the current `main` HEAD and push it — a **bare commit** (no generated CHANGELOG or other files) — with `GITHUB_TOKEN`, so no recursion.
  5. Publish the GitHub Release via `softprops/action-gh-release`, pointing at the tag; GitHub auto-attaches the source archive.
- Concurrency: a `concurrency: release` group so concurrent dispatches can't race.

**Versioning script (`scripts/version.sh`)**

- Pure zsh, testable with the existing `describe`/`toBe` framework.
- Two modes: `suggest` (print the next version given the current history — used by CI) and resolve a forced `major`/`minor`/`patch` (used by CD when the user overrides).
- Rules from Conventional Commits since the last tag: `!` in type/scope or a `BREAKING CHANGE:` footer → major; any `feat` → minor; any other conventional commit → patch; no conventional commits → `none`.
- First-release bootstrap: when no tag exists yet, the base is `0.0.0` and the bump (major/minor/patch, or the auto-derived suggestion) is applied against it — e.g. `major` → `v1.0.0`, `minor` → `v0.1.0`, `patch` → `v0.0.1`. Writes results to `$GITHUB_OUTPUT` when available, guarded so it also runs locally.

**CI/CD infra checklist**

- `.github/workflows/ci.yml` and `.github/workflows/cd.yml` (decoupled).
- `scripts/version.sh` (suggest + forced-bump resolution).
- Established actions only: `actions/checkout`, `actions/setup-go`, and `softprops/action-gh-release` for the release step.
- Repo permissions: `contents: write` on the release workflow; no secrets (uses `GITHUB_TOKEN`).
- Conventions: tags `vX.Y.Z`; Conventional Commits are the source of truth for versioning; the tag commit is bare (no CHANGELOG).
- `test/version.spec.sh` so the versioning logic is itself covered by CI.

**Behavior preserved**

- `scripts/lint.sh` and `scripts/test.sh` remain the single source of truth for "passes" — workflows call them directly, no duplicated logic.
- No changes to the tools, the install flow, or the test framework beyond adding CI-facing scripts/specs.

**Out of scope**

- Build artifacts, binary/signing, and publishing to any package registry (npm, Homebrew, etc.) — this is a pure-script package; the source archive is the release.
- Branch-protection rules, required status checks, and tag-protection rules — these are repo *settings*, not code, though the workflows are written to be compatible.
- Retrofitting `install.sh` to pin a release tag instead of the `main` tarball — possible follow-up, not part of this demand.

**Decisions**

- **Release tooling**: `softprops/action-gh-release` publishes the Release (declarative action; source archive auto-attached).
- **CI green guard in CD**: trust the manual-dispatch flow — the user only dispatches a release after CI is green; CD does not re-check CI status.
- **CHANGELOG**: no generated changelog; the release tags a **bare commit** (no added files).
- **`zsh-lint` version**: `@latest` (accepts drift; matches the hint in `scripts/lint.sh`).
- **First-release bootstrap**: base `0.0.0`, apply the chosen bump on top — `major` → `v1.0.0`, `minor` → `v0.1.0`, `patch` → `v0.0.1`.

## Plans

- [x] implementVersionScript — add `scripts/version.sh` (pure zsh) with a `suggest` mode that inspects Conventional Commits since the last tag and prints the bump (`major`/`minor`/`patch`/`none`) and next version, plus a `resolve <bump>` mode that applies a forced or auto bump against the current (or `0.0.0`) base; writes `$GITHUB_OUTPUT` when available, guarded for local runs.
- [x] addVersionSpec — add `test/version.spec.sh` covering suggest (major/minor/patch/none from commit log) and resolve (forced bumps, first-release from `0.0.0`), using the existing `describe`/`toBe` framework.
- [x] addCiWorkflow — add `.github/workflows/ci.yml`: `push` (branches `main`) + `pull_request`, tags excluded; matrix `macos-latest`/`ubuntu-latest`; `actions/setup-go` + `go install github.com/z-shell/zsh-lint/cmd/zsh-lint@latest`; lint job runs `scripts/lint.sh`, test job runs `scripts/test.sh`; a suggest step on `push` to `main` runs `scripts/version.sh suggest` and writes the summary.
- [x] addReleaseWorkflow — add `.github/workflows/cd.yml`: `workflow_dispatch` with `bump` input (`auto`/`major`/`minor`/`patch`), `if: github.ref == 'refs/heads/main'`, `contents: write`, `concurrency: release`; checkout `fetch-depth: 0`; resolve bump via `scripts/version.sh`; no-op on `none`; create + push the `vX.Y.Z` tag with `GITHUB_TOKEN`; publish via `softprops/action-gh-release`.
- [x] documentCicdFlow — document the CI/CD flow, the versioning rules, and the tag convention (`vX.Y.Z`) in `CLAUDE.md` so future changes follow the same release process.
