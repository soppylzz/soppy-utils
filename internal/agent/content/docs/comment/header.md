# Comments

A comment says what the code cannot. If it repeats the line beneath it, delete it.

- Explain *why*, not *what*. The reader can see what the code does; they cannot see what
  you knew when you wrote it.
- Say what was tried and rejected when that is the question the next person will ask.
- Describe the constraint, not the workaround. "The API returns duplicates" outlives
  "dedupe the list"; the workaround usually does not.
- Do not narrate the change. "Added validation" belongs in the commit message, and the
  commit message already has it.
- Keep a comment next to the code it describes. A comment that drifted to another
  function is worse than none, because it is believed.
- Delete a comment that has stopped being true. A stale comment is a lie with a
  timestamp.
