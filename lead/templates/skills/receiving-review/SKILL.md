---
name: receiving-review
description: Handle a lead's or area lead's rework reasons with technical rigor. Use when a task comes back with a review reason, before changing anything.
---

# Receiving a review

A task you worked on came back with `rework` and a reason. Treat the reason as a requirement to check, not as an order to obey blindly and not as an insult.

1. **Read all of it first.** Items can depend on each other; fixing half of them can make the rest wrong.
2. **Restate each item in your own words.** If one is unclear, do not guess: say so in your summary and fix only what is clear.
3. **Verify against the code.** Is the point true for this codebase, this base commit, these acceptance criteria? Run the test that would show it.
4. **Then act, one item at a time,** each with its own check. Do not touch anything the review did not ask for.
5. **If an item is technically wrong,** do not "fix" it anyway: explain in the summary why, with evidence (test output, file and line).

No performative agreement ("great point, you're absolutely right"). The summary lists each review item with what you changed or why you did not, and how you verified it.
