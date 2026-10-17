# Code conventions

These hold in any language. Where a language disagrees, the language wins and this file
says why.

- **Name for the reader.** A name that needs a comment to be understood has not been
  named yet.
- **Make the invalid state unrepresentable.** A type that cannot hold a bad value removes
  the check, the test, and the bug.
- **One reason to change per unit.** A function that changes when the storage layer
  changes and when the report format changes is two functions sharing a body.
- **Errors are values, and they are for the caller.** Say what failed and what can be
  done. "Something went wrong" tells the reader nothing they did not know.
- **Fail loudly.** A fallback that hides a failure turns a bug into a wrong answer, which
  is worse and arrives later.
- **Delete rather than disable.** Commented-out code is code nobody dares remove and
  nobody dares use. Version control remembers it.
- **Test the behaviour, not the implementation.** A test that breaks when the code is
  refactored is a test of the refactoring.
