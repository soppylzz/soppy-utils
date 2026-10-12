---
name: v-patch
description: Append a change record to a demand's Patches section in plan/<demand-name>.md. Use when the user invokes /v-patch to record a modification to an existing demand.
user-invocable: true
disable-model-invocation: true
allowed-tools:
  - Read
  - Write
  - Edit
---

# /v-patch — record a modification to a demand

Arguments: `<demand-name>/<patch-description>` from `$ARGUMENTS`.

## Steps

1. Split on the first `/`. Require a lowercase kebab-case demand name and a nonempty description; preserve any later `/` in the description.
2. Open `plan/<demand-name>.md`. If it is missing, direct the user to `/v-demand`; do not create a partial document.
3. Derive a short kebab-case patch name. Append one entry under `## Patches`, creating that section if absent. Use the current local date and valid GitHub Flavored Markdown indentation:

```
1. `<patch-shortcut>` <sup>@YYYY-MM-DD</sup>
   - Description: <patch-description>.
   - Plans:
     - [ ] <step description>
```

4. Preserve earlier entries. If implementation was requested, do the work and mark each completed patch step `[x]` as it finishes. Leave incomplete steps `[ ]`.
