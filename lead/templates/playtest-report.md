# Template: playtest report

> For games and apps. A playtest is a task like any other (type `visual/feel`): a fixed scenario, fixed questions, a structured report. The visionary's playtest decides acceptance; agent playtests (scripted runs, screenshots, measurements) prepare it.

## Header
- Build: <commit> · platform: <OS, input device> · tester: <visionary / agent / person>
- Scenario: <where to start, what to do, how long>, e.g. "start at checkpoint 2, cross the hall, fight the first guard pair, 5 minutes"
- Focus: <which tasks or pillars this playtest checks>

## Questions (fixed before the test)
1. <one question per pillar or feature, answerable with yes/no or a number>
   e.g. "Can you tell which attack is coming before it hits? (yes / sometimes / no)"
2. …

## Observations
| Time | What happened | Expected | Severity (blocker / annoying / cosmetic) |
|---|---|---|---|
| 01:20 | | | |

## Measurements (if available)
- frame rate (min / median), deaths, time per section, inputs that did nothing

## Result
- Answers to the questions
- Accepted: <task IDs> · rework: <task IDs with a concrete reason> · new tasks: <short list>
- One sentence on the feel: <the tester's own words, unedited>
