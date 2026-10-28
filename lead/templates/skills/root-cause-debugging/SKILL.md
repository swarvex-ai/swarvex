---
name: root-cause-debugging
description: Find the cause of a failure before changing code. Use for any failing test, error message, crash or unexpected behaviour.
---

# Root cause first

1. **Reproduce.** Run the failing command and read the complete error, including the first error in the output, not only the last line.
2. **Locate.** Follow the stack trace or error to the exact line. Read that code and its callers.
3. **Hypothesis.** State one concrete cause ("X is null because Y is set after Z").
4. **Check it** with the smallest possible experiment: a print, a focused test, reading the value. Do not change several things at once.
5. **Fix the cause**, not the symptom. No catch-all error handling, no sleeps, no special cases for the test.
6. **Guard it** with a test that fails without the fix, when the project has tests for that area.

If three hypotheses in a row turn out wrong, stop guessing: write down what you know, re-read the relevant code from the top and the project's engine or library reference.
