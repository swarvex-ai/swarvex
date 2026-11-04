---
name: root-cause-debugging
description: Find the cause of a failure before changing code. Use for any failing test, error message, crash, wrong output or slowdown.
---

# Root cause first

## 1. Build a loop that goes red on this bug

Everything else depends on one command that shows the failure. Find or write it before reading code for theories:

- a failing test at the place where the bug shows (unit, integration, end to end),
- a script that calls the program or the HTTP endpoint with the input that fails,
- a headless browser script for UI bugs, a replayed input file, or a comparison of the old and the new version on the same input.

The loop is ready when it is **red-capable** (it checks the exact symptom that was reported, not "runs without error"), **deterministic** (fixed time, seeded randomness, no network), **fast** (seconds) and **runs without a human**. Run it once and keep its output. For a bug that only shows sometimes, raise the rate (repeat it 100 times, add load) until it shows often enough to work with. If no loop is possible, say so in the summary with what you tried; do not guess a fix.

## 2. Reproduce and shrink

Confirm the loop shows the reported failure, not a neighbouring one. Then remove inputs, steps and configuration one at a time, re-running after each cut, until every remaining part is needed for the failure. The small case becomes the regression test.

## 3. Three to five hypotheses, ranked

Write them down before testing any of them, each with a prediction: "If X is the cause, then changing Y makes the failure disappear." A hypothesis without a prediction is a hunch: sharpen it or drop it.

## 4. Test one hypothesis at a time

Change one thing per experiment. Prefer a debugger or one focused log line at the boundary that separates two hypotheses. Mark every temporary log with one tag, for example `[DEBUG-a4f2]`, so all of them can be found and removed. For slowness, measure first (a timing harness or profiler), then change.

## 5. Fix the cause, guard it

Turn the small case into a test at the place where the bug really happens, watch it fail, fix the cause (no catch-all error handling, no sleeps, no special case for the test), watch it pass, then run the loop from step 1 again on the original case. If no test can reach the bug at the right place, write that down in the summary: it is a finding about the design.

## 6. Clean up and report

Remove every tagged log and throwaway script. The summary names the cause that turned out to be right and the command that shows the fix, so the next person learns from it.

If three hypotheses in a row are wrong, stop: write down what you know, re-read the code from the entry point, and check the project's engine or library reference.
