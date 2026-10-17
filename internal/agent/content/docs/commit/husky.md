
## Enforcing it

`commitlint` rejects a message that does not parse, which is worth more than a convention
nobody can check. It runs from a `commit-msg` hook:

```sh
npx --no-install commitlint --edit "$1"
```

Husky installs the hook. Keep the rules in `commitlint.config.js` next to the code so a
change to the convention is reviewable like any other change.
