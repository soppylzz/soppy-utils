---
name: v-patch
description: Append a change record to a demand's Patches section in plan/<demand-name>.md. Use when the user invokes /v-patch to record a modification to an existing demand.
user-invocable: true
allowed-tools:
  - Read
  - Write
  - Edit
---

# /v-patch — record a modification to a demand

Arguments passed: `$ARGUMENTS` = `<demand-name>/<patch-description>`.

## Steps

1. Split `$ARGUMENTS` on the first `/` into `<demand-name>` and `<patch-description>`.
2. Open `plan/<demand-name>.md`. If it does not exist, tell the user to run `/v-demand` first.
3. Analyze what the change requires and derive a short kebab-case `<patch-shortcut>` from the description.
4. Append to `## Patches` (create the section if absent) an ordered-list entry in this exact form:

```
1. <patch-shortcut><sup>@<YYYY-MM-DD></sup>:
  desc: <patch-description>.
  plans:
    - [ ] <step description>;
```

Use the current date as `<YYYY-MM-DD>`.
5. Maintain checkbox state in real time: as each `plans` step is executed, flip its `- [ ]` to `- [x]` in the file.
