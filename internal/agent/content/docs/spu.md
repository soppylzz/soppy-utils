# This project's agent text is managed by spu

Part of what an agent reads in this project is generated. `spu agent` writes it, keeps a
record of what it wrote and with which answers, and regenerates it when the templates
behind it change. Everything generated lives in this `.agents/` directory.

To pick it up, this file is referenced once from the project's instruction file:

```markdown
@.agents/SPU.md
```

Claude Code expands that `@` and reads what follows. Codex and Pi do not have an import
syntax — they read it as an instruction, which works the same way as long as the file
named is the one they need.

## Do not edit the generated files

spu records the hash of every file it writes. A change to one is not lost the first time,
but it is a change spu has to treat as somebody else's work: it stops and asks before
replacing a file that no longer matches what it wrote.

To add to a document, write your own beside it and reference it from `AGENTS.md` or
`CLAUDE.md`. A file you own is never touched.

To change what a document says, change the answer it was generated from:

```sh
spu agent edit      # answer one item's questions again
spu agent           # choose again what to generate
spu agent status    # what is generated, missing, or changed since
```

Clearing an item in the picker stops spu tracking it. The file stays where it is and
becomes yours.

## What is here

- `spu.json` — the record: which items, which answers, and the hash of each file as written.
- `prompts/` — the conventions documents.
- `skills/` — skills, each a directory holding a `SKILL.md`.
- `SPU.md` — this file.

## The documents

<!-- spu:index -->
