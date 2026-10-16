## TypeScript

- `strict` is not optional. A project without it is JavaScript with extra steps.
- Prefer `unknown` to `any`. `any` turns off the compiler exactly where it was needed.
- Model with unions rather than optional fields. `{kind: "a", x: number} | {kind: "b"}` 
  makes an impossible combination unrepresentable.
- Let the compiler infer what it can and annotate what it cannot. An annotation on a
  local is usually noise.
- `readonly` on a type that is not meant to change says more than a comment saying so.
- Prefer `const` and immutable updates. Shared mutable state is the bug you will spend
  the afternoon on.
