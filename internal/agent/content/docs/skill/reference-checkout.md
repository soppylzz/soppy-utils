---
name: reference-checkout
description: Read a third-party package's source from a local checkout instead of fetching it over the network. Use when answering a question about a dependency's API, behaviour, licence, or internals, when a library's documentation is thin, or when the same package would be looked up more than once.
---

# Reading somebody else's source

When a question is about a package this project depends on, read its source from a local
checkout rather than making repeated web requests. A checkout is faster, works offline,
answers questions the published documentation does not, and costs nothing to re-read.

Checkouts live in `{{reference-dir}}`. The variable `SPU_REFERENCE_CACHE` overrides that
when it is set, so honour it if it is.

## Look before you fetch

```sh
ls "{{reference-dir}}" 2>/dev/null
```

If the package is already there at a version that answers the question, read it and stop.
Re-fetching something already on disk is the waste this skill exists to avoid.

## Fetching

Clone shallow, and ask for the version that matters rather than the tip of the default
branch:

```sh
mkdir -p "{{reference-dir}}/<host>/<owner>" && \
git clone --depth 1 --branch <version> https://<host>/<owner>/<repo> \
  "{{reference-dir}}/<host>/<owner>/<repo>"
```

- **A major is usually what matters.** A v3 is not a newer v2 but a different library that
  happens to share a name, so write the checkout path with the major in it when two majors
  could both be wanted.
- **Find the version before cloning**, rather than guessing a ref:

  ```sh
  git ls-remote --tags --refs https://<host>/<owner>/<repo> 'refs/tags/v<major>.*' \
    | awk '{print $2}' | sed 's|refs/tags/||' | sort -V | tail -1
  ```

  Tags are not branches, and `--branch 8` is not a thing that exists. Sort versions
  numerically — `sort -V`, not `sort` — or v10 lands before v9.
- **`--depth 1` is enough for reading.** It is much faster and the history is almost never
  what the question is about. It also means `git describe --tags` will not work afterwards,
  so note the version you asked for rather than trying to recover it later.

## A specific version, or the newest?

Match what the project already depends on — the lockfile, `go.mod`, `package.json`, or the
imports in the code — because that is the version whose behaviour the answer has to
describe. Then say which version you read, so the answer can be checked against it.

If answering means reading a version the project does **not** depend on, say which version
you want and why, and let the person decide. Pulling a pinned version without being asked
is how a cache acquires things nobody wanted.

## Reading, not writing

Treat `{{reference-dir}}` as read-only. It holds somebody else's source, shared across
projects: an edit there is invisible in this project's history and surprising in the next
one. If a change to a dependency is the actual goal, that is a different task — a fork, a
patch file, or a contribution upstream.
