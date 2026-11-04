---
name: test-first
description: Write or extend a failing test before changing behaviour. Use whenever a task adds a feature, changes behaviour or fixes a bug in code that has tests.
---

# Test first

1. Find the existing tests for the code you will change and the command that runs them. Use the project's terms from `docs/glossary.md` in test names.
2. Pick the place to test: the **public interface** of the module you change, the same surface its callers use. Tests there survive refactoring; tests of private functions break whenever the inside changes.
3. Write a test that describes the wanted behaviour (or reproduces the bug), named like a sentence of the specification ("player cannot jump while stunned"). Keep it small and deterministic: no real time, randomness only with a fixed seed, no network, no dependence on order.
4. Run it and **watch it fail** for the expected reason. A test that passes before the change proves nothing.
5. Write the smallest change that makes it pass. One behaviour, one test, one change; then the next.
6. Run the whole test suite, not only the new test.
7. Clean up while everything is green, then run the suite again.

Good tests take their expected values from an independent source: a known-good literal, a worked example, the acceptance criteria. Three kinds of tests look like coverage and check nothing:

- **Tautological:** the expected value is computed the way the code computes it (`expect(add(a, b)).toBe(a + b)`), so it can never disagree with the code.
- **Coupled to the inside:** it mocks the module's own collaborators or reads its private state, so it breaks on a refactor while behaviour stays the same.
- **All tests first:** a batch of tests written before any code tests the shape you imagined, not the behaviour you built. Go one test at a time.

Keep every existing test's expectation unless it is really wrong, and then say so explicitly in the summary.
