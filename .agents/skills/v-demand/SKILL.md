---
name: v-demand
description: Create a new plan/<demand-name>.md requirement document and write its Demands analysis. Use when the user invokes /v-demand to start implementing a new requirement.
user-invocable: true
disable-model-invocation: true
allowed-tools:
  - Read
  - Write
  - Edit
  - Glob
---

# /v-demand — create a requirement document

Arguments: `<demand-name> [demand-description]` from `$ARGUMENTS`.

## Steps

1. Parse a required lowercase kebab-case `<demand-name>`. Treat the remaining arguments as the description. If the requirement is missing, ask for it before creating a file.
2. Check whether `plan/<demand-name>.md` already exists. Preserve it and direct changes to `/v-plan` or `/v-patch` instead of overwriting it.
3. Create the document in English and GitHub Flavored Markdown. Use exactly one H1, `# <demandName>` (lower camel case), followed by `## Demands`. State the user goal, observable behavior, constraints, and open decisions supported by the request; do not invent resolved choices.
4. Report the new document to the user. Add `## Plans` only when the user requested planning or implementation in the same invocation; add `## Patches` only when recording a later change. Continue implementation if the user already requested it, without an extra approval checkpoint.
