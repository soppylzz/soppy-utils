# bashGoMigration

## Demands

Migrate the repository from zsh-only scripts to a Bash 3.2 + Go application while keeping `spu <util> [...args]` as the public command. Go owns persistent structured configuration, migration, terminal forms, and generated artifacts; Bash owns the launcher, updater swap process, and simple shell integrations. Publish precompiled Go binaries so users do not need a Go installation. Support macOS and Linux shells, and all `spu` commands in Windows Git Bash. Native PowerShell and CMD entry points are outside this migration.

This plan supersedes the zsh-specific implementation details in earlier demand documents. Their completed checkboxes describe historical work and must not be reset.

### Migration boundary

| Component | Migration | Target owner and behavior |
| --- | --- | --- |
| `spu welcome` | Full | Go parses options, runs the TUI, renders the generated Bash script, manages the generated file and optional `.bashrc`/`.zshrc` hooks, and uninstalls its own state. |
| `spu license` | Full | Go parses both one-shot CLI and interactive TUI input, renders MIT/ISC text from embedded templates, and writes stdout or a chosen file. |
| `spu git` | Full | Rename the public command and move its global Git include management, structured preferences, migration, and TUI editor into Go. Git remains the config authority. |
| `spu vscode` | Partial | Port the existing command to Bash 3.2 and add the Windows Git Bash VS Code user directory. |
| `spu update`, launcher, install/uninstall, test, lint, version scripts | Infrastructure | Add a package update command that reports configuration mismatches and directs users to manual migration commands; replace zsh entry points and helpers with Bash 3.2 equivalents, and add the Go build/release flow. |

“Full” means the tool's parsing, interaction, rendering, file operations, and tests live in Go. The Bash dispatcher only starts its compiled helper. “Partial” means the tool remains in Bash, with limited changes for Bash syntax and platform support. `vscode` does not need a Bash-to-Go protocol.

### Proposed source layout

```text
bin/spu                         Bash entry point
src/spu.sh                      Bash dispatcher and overview help
src/vscode.sh                   Bash-owned util
src/usage/vscode.sh             Bash-owned help text
src/utils/{style,usage}.sh      Bash 3.2-compatible presentation helpers
cmd/spu-helper/main.go          Go helper entry point
internal/cli/                   Go command routing and terminal checks
internal/configstore/           JSON Schema validation and in-memory polyfills
internal/managedfile/           Ownership checks and staged replacement
internal/paths/                 Native/Git Bash home and path conversion
internal/ui/                    Reusable Huh field and validation adapters
internal/welcome/               Form, migration, renderer, hooks, state
internal/welcome/schemas/       Versioned JSON Schemas
internal/welcome/templates/banner.sh.tmpl
internal/license/               Form, defaults, migration, renderer, output
internal/license/schemas/       Versioned JSON Schemas
internal/license/templates/{mit,isc}.tmpl
internal/gitconfig/             Preferences, migration, renderer, Git include
internal/gitconfig/schemas/     Versioned JSON Schemas
internal/update/                Release selection and upgrade coordination
internal/buildinfo/             Build-time spu version and script revisions
libexec/spu-helper[.exe]        Build output, present in release archives only
scripts/{install,update,uninstall}.sh
scripts/dev/{format,hooks,lint,package,smoke,test,version}.sh
test/                            Bash specs and shared Bash assertions
go.mod, go.sum
```

The installed root is `$HOME/.local/share/soppy-utils`; `$HOME/.local/bin/spu` is a small managed Bash launcher pointing to that root. User-generated state stays in `$HOME/.config/soppy-utils`, outside the installed root. The release archive contains the Bash runtime, templates needed by Bash, uninstall script, and the correct `libexec/spu-helper` binary; it does not require source code or `go` at runtime.

The Go helper is an internal executable, not a second public command. The Bash dispatcher routes `license`, `welcome`, and `git` to the helper and routes `update` to a dedicated updater process. Go returns each tool's exit status and writes generated license text only to stdout when stdout is selected. Prompts and diagnostics use the terminal/stderr so piping `spu license --mit` remains clean. Bash owns the top-level command list and error for an unknown util; each tool owns its detailed `--help`. Preserve brief `-h` and detailed `--help` behavior in both languages. Remove obsolete `src/license.sh`, `src/welcome.sh`, `src/git-custom-ini.sh`, their zsh usage files, `src/utils/template.zsh`, and `src/welcome-banner.template` only after Go replacements pass migration tests.

### Shared structured configuration and migrations

All three Go-owned utils have persistent preferences in separate directories: `$HOME/.config/soppy-utils/{welcome,license,git}/`. Each directory contains a plain UTF-8 `VERSION` file with the util's **configuration contract version** (for example `2\n`) and a `config.json` file with the same integer in `schema_version`, a util identifier, and typed `data`. Increment this version only when the persisted data contract changes; the package's `spu --version` is separate and does not cause a mismatch merely because a new binary was released. `license` stores preferred type, author, and default output path; `git` stores alias, editor/diff/merge, and line-ending preferences; `welcome` stores banner content and rc targets. Generated LICENSE files remain user-owned outputs.

Contract versions are independent per util: `license/VERSION=2` and `git/VERSION=1` can both be current for the same spu release. A warning is based on each util's own required contract, never on equality with the global package SemVer.

Each util also has a `state.json` manifest for hashes and generated-artifact provenance. Welcome's manifest records the generated Bash script hash, script compatibility schema, template revision, and producing spu release; Git's records the managed `.gitconfig` hash and path. This keeps `VERSION` focused on configuration compatibility and avoids mixing version comparison with artifact ownership. `VERSION` and `config.json.schema_version` must agree; disagreement is a warning and must not be silently repaired by an ordinary command.

| Version marker | Location | When it changes | Ordinary-command effect |
| --- | --- | --- | --- |
| spu release | Build metadata | Every package release | Display through `spu --version`; no config warning by itself. |
| Util config contract | `<util>/VERSION` and `config.json.schema_version` | Persisted structure or validation contract changes | Warn and use a known-version in-memory polyfill until manual migration. |
| Generated template revision | `<util>/state.json` where applicable | Generated output changes without breaking compatibility | Show an optional refresh; no config warning. |
| Generated artifact schema | `<util>/state.json` where applicable | Generated file's management contract changes | Prevent unsafe replacement by an older binary. |

A simpler alternative is to store only `schema_version` inside `config.json` and omit `VERSION`; that removes a duplicate value and a possible disagreement state. The chosen two-file contract keeps the requested human-readable `VERSION` marker. Treat disagreement as a conflict, show both values, and require an explicit repair rather than deciding silently which one wins.

- Keep versioned [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12) documents under each util and embed them in the Go binary. Pin a tested Go validator such as [`jsonschema/v6`](https://github.com/santhosh-tekuri/jsonschema); use JSON Schema for structure and Go validation for domain rules such as allowed rc targets, paths, and Git command safety. JSON Schema's [`default` annotation does not populate missing values](https://json-schema.org/understanding-json-schema/reference/annotations), so each util must implement explicit, testable polyfill functions. Validate old data against its known schema before applying defaults, and validate the effective data against the current schema before use or persistence.
- `internal/configstore` owns bounded JSON reads, version comparison, JSON Schema validation, and in-memory default polyfills. `internal/managedfile` owns content hashes, ownership checks, staging, backups, and replacement; `internal/paths` owns home resolution and Git Bash/native conversion. `internal/ui` provides shared Huh form setup and common field validators. Each util owns its schemas, defaults, renderer, and explicit pure `vN -> vN+1` migrations. The shared layer does not know a util's field meanings or Git commands.
- An ordinary `spu <util> ...` command with a recognized older config emits one concise warning to stderr naming the detected and required versions and the exact `spu <util> migrate` command. It computes an **in-memory** current-shape view by applying known migrations and filling missing fields from documented defaults. It does not rewrite `VERSION`, `config.json`, or managed artifacts just to silence the warning. CLI flags override effective defaults for that invocation. Commands that write managed artifacts still require valid ownership checks before writing.
- `spu <util> migrate` explicitly persists the current config contract version and any necessary generated-artifact updates after validation and, when new fields need choices, a Huh form. `spu <util> edit` opens that same form at any time and can migrate an older config before saving. An optional versioned `--answers <file>` input supplies values without a terminal; it uses the same validators. Both commands stage files before replacement and keep the old state on failure or cancellation.
- For an unknown newer schema, malformed JSON, a `VERSION`/JSON disagreement, or an unrecognized legacy file, do not guess how to rewrite it. Emit a warning and use safe built-in defaults for read-only generation where possible; refuse mutations of managed output until the user resolves the conflict with a compatible tool or explicit migration. This is the limit of default polyfills: they fill known missing fields, but cannot safely reinterpret unknown future data.
- Keep a cross-process package lock during update and per-util locks during edits or migration. The updater scans only util config directories that exist, reports mismatches, and never creates settings for unused utils. The first Go release must recognize the old welcome checksum and Git include path without evaluating old shell assignments; a user can convert legacy data with each util's explicit `migrate` or `edit` command.

### Full migration: `welcome`

- Preserve `spu welcome install` and `spu welcome uninstall`, including `--name`, `--device`, `--message`, and `--rc none|bash|zsh|both`. A terminal form collects missing install values; supplied flags prefill or skip the relevant fields. When stdin is not a usable terminal, a fresh install uses documented defaults while an existing config reuses its saved values. An explicit interactive option must report a clear error without a terminal. Cancellation exits without changing files. `spu welcome edit` reopens the same form and commits changed structured data and its rendered script together.
- Use [Huh v2](https://github.com/charmbracelet/huh) for the form, with fields for banner text and startup hook targets. Pin a tested release in `go.mod` and use the same validation for flags and form answers. Keep TUI output away from stdout when stdout is a data channel.
- Move the banner to a trusted embedded Go `text/template`, loaded with `go:embed`. Render a Bash script with `#!/usr/bin/env bash`. `text/template` does not escape shell syntax; a dedicated, tested quoting function must encode each user value as inert shell data. Test quotes, dollar signs, command substitutions, backslashes, Unicode, and newlines. Do not interpolate raw form input into executable shell syntax.
- Store the generated script and its metadata under `$HOME/.config/soppy-utils/welcome/`. Manage only marked blocks in `.bashrc` and `.zshrc`, selected by `--rc`; `--rc none` removes any previously managed hook and leaves other rc content untouched. The hook checks that its **parent shell** is interactive before executing the generated Bash script, because a child Bash process itself is noninteractive even when launched from an interactive zsh or Bash session. Pass an explicit `SPU_WELCOME_INTERACTIVE=1` signal from the guarded hook. The generated script exits silently unless that signal is present and either stdout or the controlling terminal is a TTY. Powerlevel10k instant prompt can redirect stdout while loading `.zshrc`, so checking stdout alone hides the banner at shell startup. Hooks must be idempotent and must not remove user text outside their markers.
- Preserve safe ownership checks: never overwrite an unmanaged or user-modified generated file; remove the generated file during uninstall only when its recorded content still matches. Detect malformed or duplicate hook markers before mutation. Stage a new script and hook edits in temporary files, preserve existing permissions where relevant, and recover cleanly if a later write fails. Document how an existing zsh-generated banner and checksum file are adopted or left for manual resolution.
- Warn when the legacy `$SOPPY/shell/welcome.sh` hook is still present so two banners are not printed. Do not edit the user's legacy source script.

### Generated welcome version and compatibility

Keep `$HOME/.config/soppy-utils/welcome/{welcome.sh,VERSION,config.json,state.json}` together. `VERSION` is a plain integer equal to `config.json.schema_version`; it changes only when the welcome configuration contract changes. `state.json` records the script hash, script compatibility schema, template revision, and the spu release that generated it. The release version is build metadata exposed through `spu --version`, not a reason to warn about config migration. A development build uses a visibly nonrelease value such as `dev+<commit>`.

```text
welcome/VERSION       1
welcome/config.json   {"util":"welcome","schema_version":1,"data":{...}}
welcome/state.json    {"script_schema":1,"template_revision":4,"script_sha256":"..."}
```

- `spu welcome status` reports the stored config contract version, current required version, script schema, template revision, ownership state, and exact migration or refresh command. A compatible old config uses in-memory polyfills and warns on `spu welcome` commands until the user runs `spu welcome migrate` or `spu welcome edit`. A template-only revision change does not trigger a config warning; `spu welcome install` explicitly refreshes the generated script after ownership validation.
- `spu update` does not regenerate `welcome.sh`, alter its hooks, or open a form. When welcome's config contract differs, it prints a warning to run `spu welcome migrate`; the old self-contained banner remains runnable. The generated startup path does not invoke `spu` or read `VERSION`, `config.json`, or `state.json`.
- Before a refresh or explicit migration, validate all state and rc markers. Stage the script, config, version, and metadata, then replace them with rollback for a failed write. `spu welcome uninstall` removes valid managed hook blocks and an unchanged generated script/manifest, while retaining `VERSION` and `config.json` for future reinstallation or editing; `--purge` also removes an unchanged owned config and version file. Preserve changed or unmanaged files and explain what was left behind.
- Existing deployments may have `$HOME/.config/soppy-utils/welcome.sh` plus the old `welcome.cksum` but no structured config. Recognize that legacy pair only when the old checksum and managed header match. Keep the old script active and warn that `spu welcome migrate` or `spu welcome edit` must collect values before replacing it; do not parse or execute old Bash assignments as configuration. If neither valid new metadata nor valid legacy state exists, treat the script as unmanaged.

### Full migration: `license`

- Keep the existing one-shot forms: `spu license --mit` and `spu license --isc` print to stdout; `-M`, `-I`, `-o`, and `--output` remain aliases. Add `--name` and `--year` overrides for noninteractive automation. Resolve author by `--name`, saved preferred author, `git config user.name`, then `$USER`/`$USERNAME`; use the current local year by default. Reject conflicting license type flags and invalid years.
- Add a create-vue-style form: `spu license` opens a TUI when attached to a usable terminal; `spu license --interactive` forces the TUI and fails clearly without one. With no terminal and no generation flags, show usage rather than block. The form chooses MIT or ISC, edits author and year, previews the result or a concise summary, chooses an output path defaulting to `LICENSE`, and confirms before writing or replacing an existing file. Cancellation leaves the destination unchanged.
- Persist preferred license type, author, and output path in `license/config.json`; resolve a default year at execution time rather than storing a stale year. `spu license edit` opens a TUI for these defaults. The create form starts from saved defaults and offers an explicit “save as defaults” choice; CLI overrides and generated LICENSE files do not automatically change preferences. When a contract version differs, `spu update` warns and the user chooses when to run `spu license migrate` or `spu license edit`.
- Both entry modes build one validated `LicenseRequest` and call the same renderer and output writer. CLI stdout remains the default when a type flag is supplied. An explicit `--output` retains the current overwrite behavior; interactive mode confirms replacement. File writes use a temporary file in the destination directory and an atomic rename where supported, so a render or write failure never leaves a truncated license. A package update or profile edit never rewrites an existing LICENSE file.
- Move the existing MIT and ISC data into embedded `text/template` files with `{{.Name}}` and `{{.Year}}` fields. Preserve the rendered legal text and its trailing newline. Do not run arbitrary user-supplied templates; license templates are trusted assets shipped with the binary. Remove the old zsh associative-array renderer after output parity tests pass.

### Full migration: `git`

- Rename the public command to `spu git install|uninstall|edit|status`; the Go helper owns its argument parsing, TUI, rendering, and Git operations. Keep the existing `$HOME/.config/soppy-utils/git-custom-ini.gitconfig` generated data path during migration so an upgrade does not create a second global include. Register that file with the user's **global** Git configuration through `include.path`; do not alter a repository's local config.
- Remove `spu git-custom-ini` from the public dispatcher in the migration release; it is not an alias. Help and examples show only `spu git`. Preserve the existing managed Git config file path and detect old registrations so existing global Git settings can be adopted without duplication.
- Preserve existing aliases, VS Code editor/merge/diff defaults, and `core.autocrlf` policy (`input` on macOS/Linux, `true` in Git Bash). Model these as typed preferences in `git/config.json`; `spu git edit` changes them through Huh and safely regenerates the managed Git config. `spu update` only warns on a contract mismatch; `spu git migrate` or `spu git edit` explicitly updates stored preferences and generated Git config after validation.
- Idempotent install adds the exact global include once; `spu git uninstall` removes only that include and an unchanged managed file while retaining `git/config.json`. `spu git uninstall --purge` also removes an unchanged owned preference file. Never remove a user's other global include entries. Invoke Git through `os/exec` with argument arrays, not a shell command string. On Windows, normalize native and Git Bash path spellings before detecting or removing an existing include, and test homes with spaces.

### Partial migration: `vscode`

- Preserve `-S/--snippets`, `-s/--settings`, `-k/--keys`, and the existing `code` CLI behavior. Add the Windows user directory, normally `%APPDATA%\Code\User`, resolved from Git Bash using `APPDATA` and `cygpath` where needed. Continue using the existing macOS and Linux paths.
- Quote paths with spaces, verify `code` is available, and forward its exit status. Keep this tool in Bash; it does not need a Go helper or TUI.

### `spu update` lifecycle

`spu update` is a new public command for package upgrades and **read-only compatibility checks** of installed structured configurations. Its default source is the latest published GitHub Release, with `--version vX.Y.Z` available to pin a newer release; reject accidental downgrades. `spu update --status` reports the installed package version, available release, and per-util config contract mismatches. A new installation still uses `scripts/install.sh`; users of the old zsh release, which has no `spu update`, can run the new installer's explicit upgrade mode once to enter this lifecycle.

1. Resolve the target version, verify its release archive and checksum, and stage the package. Validate the new binary and archive layout before touching the installed root. A failed download, verification, or smoke test leaves the old package intact.
2. Copy `scripts/update.sh` to a temporary location outside the installed root and run it as a separate Bash process. It waits for any old Go helper process to exit, then swaps the package directories and launcher. This avoids replacing a running `.exe` on Windows Git Bash. Keep a bounded previous-package backup until the new package passes its own smoke test.
3. Invoke the **new** Go helper to compare each installed util's `VERSION` and `config.json.schema_version` with the contract version the new binary expects. Do not run Huh, write config, or regenerate welcome/Git output. Print one warning per mismatch, with the detected and expected versions and an exact manual command such as `spu welcome migrate`. A successful package upgrade still exits `0` when configuration warnings exist; a package failure remains nonzero.
4. Ordinary util commands use the shared read-only polyfill path for known older schemas until the user chooses to migrate. A later `spu <util> migrate` or `spu <util> edit` opens the relevant form when choices are needed and persists the migration. A repeat `spu update` reports the same mismatch if the user has not acted; it does not change settings behind their back.
5. Before shipping a release, require the new binary to read or safely default every supported older config contract. Existing welcome scripts and Git includes keep working while config migration is deferred. Package update never rewrites generated LICENSE files.

The updater prints a compact summary after the package swap. It never runs normal package uninstall or optional-state purge as part of an update.

| State after package swap | Required behavior |
| --- | --- |
| No config for a util | Skip it; do not create defaults or prompt. |
| Contract version matches | No warning or config write. |
| Config contract unchanged; only renderer revision changed | No warning or rewrite; show an optional refresh in util status. |
| Recognized older contract | Warn with `spu <util> migrate`; ordinary commands use an in-memory polyfill, while `spu update` leaves stored data untouched. |
| No terminal available | Same warning and read-only polyfill; a later TUI edit or explicit `migrate --answers <file>` can persist changes. |
| Unknown newer contract, malformed config, or modified managed output | Warn and preserve files; use built-in defaults for safe read-only operations and refuse unsafe managed-output writes. |

For example, if a future Git preferences schema adds a merge-tool choice, `spu update` prints `git: config contract 1; required 2; run 'spu git migrate'`. Until then, ordinary Git commands use the old preferences plus an in-memory default for that choice. `spu git migrate` or `spu git edit` opens the prefilled TUI and writes both `git/config.json` and the managed `.gitconfig` only after confirmation. The license migration follows the same config rule while leaving previously generated LICENSE files untouched.

### Bash runtime and cross-platform contract

- Change every project-maintained shell shebang and shell entry point from zsh to `#!/usr/bin/env bash`: `bin/`, `src/` Bash files, `scripts/`, and executable specs. Port the shared test primitives and all `*.spec.sh` files to Bash. Bash 3.2 is the compatibility floor because macOS ships it; avoid Bash 4-only associative arrays, `mapfile`, `globstar`, and namerefs. Retain the repository's `*.spec.sh` convention and its in-repo test runner.
- Replace `zstyle`, `zsh/zutil`, `typeset -A`, zsh path modifiers, and zsh dynamic function generation. Implement a small Bash usage/style helper with the same `-h`/`--help`, `NO_COLOR`, non-TTY, and styled error behavior. Keep the LICENSE body plain. Remove zsh-lint in favor of ShellCheck and `shfmt` checks for Bash, plus `gofmt`, `go vet`, and `go test` for Go. Pin tooling in CI rather than fetching floating `latest` versions.
- Use `${BASH_SOURCE[0]}` to locate Bash files. Do not rely on symlinks for the public launcher: Git Bash may implement `ln -s` as a copy. The generated `$HOME/.local/bin/spu` wrapper has an ownership marker and an absolute installed-root reference; install/uninstall replace or remove it only when it is still the managed wrapper.
- In Windows Git Bash, Bash paths and the native Go executable's paths differ. The dispatcher passes a native home path derived from `cygpath -w "$HOME"` through a named environment variable (for example `SPU_HOME_NATIVE`); Go validates and uses it for its generated files and rc hooks. Native Go APIs handle file operations; Bash calls such as `code` and `git` receive the path form they expect. Keep this conversion at the boundary and cover homes with spaces.
- Require a real terminal for TUI sessions and provide CLI flags for headless use. Validate Huh forms in an actual Git Bash terminal as well as automated noninteractive Windows CI. The Windows support statement means Git Bash, with Git for Windows and the `code` command available where those utils require them.

### Installation, release, and validation

- `scripts/install.sh` selects a GitHub Release archive for the detected OS/architecture, downloads it and its checksum, verifies it, and installs it into a staging directory before replacing the managed installation. A local development install path may use an already built helper or build with Go, but public installation never requires Go and never deletes the source checkout. A repeat run on an existing installation points to `spu update`; its explicit upgrade mode handles a legacy zsh installation that lacks the new command. `scripts/uninstall.sh` removes the managed wrapper and installed root while preserving user configuration and generated files by default; it explicitly reports that welcome hooks and global Git config remain active. `--purge` removes compatible, owned optional state through each util's uninstaller before deleting the package and refuses an unsafe purge while leaving the package installed. A normal package upgrade does not purge configuration. Preserve modified or unmanaged user files.
- Build versioned archives for macOS arm64, Linux amd64, and Windows amd64, with a manifest of SHA-256 checksums. Windows 10 and 11 share the Windows amd64 archive; Linux release validation uses Ubuntu 24.04 LTS. Use `.tar.gz` archives containing the same layout on all three systems, including `spu-helper.exe` for Windows. Test each archive by extracting it and invoking its Bash launcher, then publish assets on the existing `vX.Y.Z` GitHub Release. All three archives must pass their native smoke gates; a missing or failed target blocks the release rather than publishing a partial platform set.
- Run Bash and Go tests on macOS 15 arm64, Ubuntu 24.04 amd64, and Windows Server 2025 amd64 Git Bash. macOS 15 is the minimum CI baseline while macOS 26 uses the same arm64 archive. GitHub-hosted runners do not provide Windows 10/11 desktop x64, so Windows Server 2025 is only a CI proxy; verify desktop Windows compatibility separately. Run format/lint gates and cross-build all release targets. In CD, resolve the version, build and validate every asset, then push the tag and publish the assets; a failed build must not create a release tag. Make release retries safe if publication fails after tagging. Keep `scripts/dev/version.sh` Conventional Commit behavior while porting it to Bash.
- Test Go renderers with golden output for MIT/ISC, Git config, and banner scripts. Test CLI and TUI validation through the same request types, cancellation, overwrite handling, shell quoting, malformed rc markers, idempotent install/uninstall, global Git config isolation, Windows path conversion, and old-state migration. Cover JSON Schema validation and known-version polyfills for license, Git, and welcome; matching contracts with no warning; older contracts with one stderr warning and clean stdout; unknown newer contracts; `VERSION`/JSON disagreement; template-only upgrades; malformed metadata; checksum mismatches; manual edit/migrate; and a package update that warns but exits successfully without prompting or rewriting config. Verify that `spu update` never replaces a running helper on Windows. Run generated welcome scripts with Bash and trigger them from both interactive and noninteractive Bash and zsh startup files; noninteractive runs must emit no banner. Smoke-test all three installed archives without a Go toolchain.
- Update README files, help text, `AGENTS.md`, CI/release documentation, and install instructions after behavior is implemented. Follow the repository's English-language policy for new project text and commit messages.

### Implementation references

- [Huh v2 form API](https://github.com/charmbracelet/huh/blob/main/README.md)
- [Go `text/template` package and escaping caveat](https://pkg.go.dev/text/template)
- [Go `embed` package](https://pkg.go.dev/embed)
- [Git for Windows symbolic-link behavior](https://github.com/git-for-windows/git-for-windows.github.io/blob/main/content/symbolic-links.md)
- [GitHub-hosted runner platforms](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)

## Plans

- [x] establishContracts — record current public flags, output, exit codes, generated-file ownership rules, the `spu git` rename, legacy welcome state, and package-update behavior as acceptance cases.
- [x] createGoFoundation — add the Go module, pinned Huh v2 dependency, embedded templates, helper entry point, shared terminal detection, build metadata, and cross-platform build targets.
- [x] createConfigStore — implement per-util `VERSION` and JSON storage, embedded JSON Schemas, known-version in-memory polyfills, explicit `migrate` commands, atomic writes, and reusable Huh editing/validation primitives.
- [x] migrateLicense — implement Go license rendering, one-shot CLI, create-vue-style TUI, persisted defaults, `edit`, schema migrations, and parity/overwrite tests; retire the zsh implementation.
- [x] migrateWelcome — implement Go banner rendering, dedicated `welcome/` state with `VERSION`, `config.json`, and `state.json`, `edit`/`migrate`/`status`, interactive-only rc hooks, known-version polyfills, legacy adoption, and ownership tests; retire the zsh implementation.
- [x] migrateGit — implement Go `spu git` CLI, structured preferences, `edit`/`status`, global include lifecycle, schema migrations, Windows paths, and legacy adoption; remove the old command spelling and zsh implementation.
- [x] portBashRuntime — port the launcher, dispatcher, usage/style helpers, and `vscode` to Bash 3.2, including Git Bash paths.
- [x] portInfrastructure — port install, uninstall, lint, test, version, and all shell specs to Bash; remove zsh tooling and zsh runtime requirements.
- [x] implementUpdate — add a new `spu update` command, external Bash swap process, read-only post-update contract checks and warning commands, and a legacy-install upgrade path; leave all config migrations user-triggered.
- [x] releaseBinaries — add three OS/architecture release archives, checksum verification, native archive smoke tests, and binary-aware installation.
- [x] extendCI — add Bash/Go validation on macOS, Linux, and Windows Git Bash, and make CD build and validate all three artifacts before tagging.
- [x] updateDocumentation — revise help, README files, `AGENTS.md`, and release instructions to match the final Bash + Go architecture.
- [ ] verifyMigration — exercise license CLI/TUI and stored defaults, welcome and Git edit/migration, package update with mismatch warnings and no forced input, package install/uninstall with and without `--purge`, noninteractive banner silence, and no-Go end-user execution on all three release targets, plus a Windows 10/11 desktop check.

Earlier local macOS arm64 verification passed for lint, Bash and Go tests, the former six-target cross-build matrix, isolated install/update/uninstall cycles, read-only mismatch warnings, noninteractive banner silence, and packaged execution without Go on `PATH`. The three-target release matrix passed native tests and archive smoke tests in CI. Desktop Windows 10/11 and hands-on TUI checks remain.

The public v0.1.0 release contains the previous Zsh implementation. The v0.2.0 release is the first Bash + Go binary release; its macOS, Linux, and Windows Git Bash archive smoke tests passed in CI. Hands-on TUI and desktop Windows 10/11 checks remain.
