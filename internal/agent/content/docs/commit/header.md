# Commit messages

Write the message for whoever reads the log in six months, not for whoever reviews the
diff today.

- Use Conventional Commits: `type(scope)?: description`. The types are `feat`, `fix`,
  `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
- Keep the description lowercase, imperative, and under about 72 characters. It completes
  the sentence "applying this commit will ...".
- Mark a breaking change with `!` after the type or scope, and explain it in a
  `BREAKING CHANGE:` footer.
- Explain *why* in the body when the reason is not visible in the diff. The diff already
  says what changed.
- One logical change per commit. A commit that needs "and" in its description is two.
