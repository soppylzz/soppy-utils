---
name: v-plan
description: Edit a step of a demand's Plans checklist in plan/<demand-name>.md. Use when the user invokes /v-plan to add or change a plan step.
user-invocable: true
allowed-tools:
  - Read
  - Write
  - Edit
  - Grep
---

# /v-plan — edit a demand's Plans checklist

Arguments passed: `$ARGUMENTS` = `<demand-name>/<step-of-plan> [change description]`.

## Steps

1. Split `$ARGUMENTS` on the first `/` into `<demand-name>` and `<step-of-plan>`.
2. Open `plan/<demand-name>.md`. If it does not exist, tell the user to run `/v-demand` first.
3. Under `## Plans`, find the checklist item matching `<step-of-plan>` (a kebab-case step identifier):
   - If it exists, update its `- [ ] <step description>` text to reflect `<change description>`.
   - If it does not exist, append a new `- [ ] <step description>` item.
4. Maintain checkbox state in real time: as each executed step completes, flip its `- [ ]` to `- [x]` in the file.

## Format

Steps are markdown task items under `## Plans`:

```
## Plans

- [ ] stepOne
- [ ] stepTwo
```
