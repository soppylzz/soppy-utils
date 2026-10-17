## Go

- Return errors rather than panicking. A panic is for a state the program cannot continue
  from, not for a failure a caller could handle.
- Wrap with `%w` and add context; the caller needs to know what failed, not just that
  something did.
- Accept interfaces, return structs. The caller gets what it can use and the producer
  keeps what it can change.
- Make the zero value useful where you can. A type that needs a constructor to be valid is
  a type that will be used without one.
- Keep goroutines owned. Something must wait for every goroutine or know why it should
  not.
- Pass `context.Context` first and use it. A function that takes one and ignores it is
  worse than one that does not.
- Table-driven tests where the cases are the point, and one test per case where they are
  not.
- `gofmt` is not a preference.
