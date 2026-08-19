---
name: v-demand
description: Create a new plan/<demand-name>.md requirement document and write its Demands analysis. Use when the user invokes /v-demand to start implementing a new requirement.
user-invocable: true
allowed-tools:
  - Read
  - Write
  - Edit
---

# /v-demand — create a requirement document

Arguments passed: `$ARGUMENTS` = `<demand-name> [demand-description]`.

## Steps

1. Parse `<demand-name>` (required) and the optional `<demand-description>`.
   - `<demand-name>` must be kebab-case (lowercase, hyphen-separated).
   - If no `<demand-description>` is given, ask the user for the requirement before writing anything.
2. Create `plan/<demand-name>.md`.
3. Write the file with:
   - Exactly one H1: `# <demandName>` — the kebab-case name converted to camelCase.
   - Every heading in the file uses camelCase.
   - A `## Demands` section holding the requirement analysis (English + GFM).
4. Stop and ask the user to review `## Demands`. Do NOT write `## Plans` or `## Patches` until the user approves the analysis.
