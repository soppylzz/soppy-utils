---
name: v-plan
description: Add or revise a step in a demand's Plans checklist. Use when the user invokes /v-plan.
user-invocable: true
disable-model-invocation: true
allowed-tools:
  - Read
  - Write
  - Edit
  - Grep
---

# /v-plan — edit a demand's Plans checklist

Arguments: `<demand-name>/<step-id> [change description]` from `$ARGUMENTS`.

## Steps

1. Split on the first `/`. Require a lowercase kebab-case demand name and a lower camel case step ID, matching the existing `plan/*.md` checklists.
2. Open `plan/<demand-name>.md`. If it is missing, direct the user to `/v-demand`; do not create a partial document. Create `## Plans` if that section is missing.
3. Find the checklist item whose first token after the checkbox is the exact step ID. Change only that item, preserving its checked state. If absent, append an unchecked item. Include a concise English description when supplied.
4. If implementation was requested, do the work and mark each completed step `[x]` as it finishes. Leave incomplete steps `[ ]`.

## Format

Steps are markdown task items under `## Plans`:

```
## Plans

- [ ] stepOne — describe the outcome
- [x] stepTwo — describe the completed outcome
```
