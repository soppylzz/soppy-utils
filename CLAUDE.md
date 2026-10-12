# CLAUDE.md

Agent guidance for this repository lives in [AGENTS.md](AGENTS.md) — the harness-agnostic prompt shared by all coding agents. Claude Code imports it below. Keep all cross-agent guidance in `AGENTS.md`; only Claude Code-specific notes belong in this file.

@AGENTS.md

## Claude Code specifics

- Skills are discovered from `.claude/skills/` — a symlink to `../.agents/skills`, where the canonical definitions live — and exposed as slash commands: `/v-demand`, `/v-plan`, `/v-patch`.
- Harness settings (permissions, modes) live in `.claude/settings.json`.
