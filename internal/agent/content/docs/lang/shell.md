## Shell

- `#!/usr/bin/env bash`, and `set -euo pipefail` at the top. A script without them fails
  quietly halfway through.
- Quote every expansion: `"$var"`. Unquoted, a path with a space becomes two arguments.
- Prefer `[[ ]]` to `[ ]` in Bash. It does not word-split, and it has `&&` without
  escaping.
- Never parse `ls`. Glob, or use `find -print0` with `read -d ''`.
- Portable means the version the target ships. macOS ships Bash 3.2, so no associative
  arrays, no `mapfile`, no `${var^^}`.
- A script that cannot be re-run safely will be re-run unsafely.
