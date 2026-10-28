---
name: test-first
description: Write or extend a failing test before changing behaviour. Use whenever a task adds a feature, changes behaviour or fixes a bug in code that has tests.
---

# Test first

1. Find the existing tests for the code you will change and the command that runs them.
2. Write a test that describes the wanted behaviour (or reproduces the bug). Keep it small and deterministic: no real time, randomness without a fixed seed, network or order dependence.
3. Run it and **watch it fail** for the expected reason. A test that passes before the change proves nothing.
4. Write the smallest change that makes it pass.
5. Run the whole test suite, not only the new test.
6. Clean up while everything is green, then run the suite again.

Never change an existing test's expectation just to get green. If an expectation is really wrong, say so explicitly in the summary.
