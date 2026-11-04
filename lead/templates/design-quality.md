# Design and usability standard

> Loaded by the design, UX, content and frontend roles of websites and apps, and by the UX role of games for menus and HUD. One standard for every project, so that a submission from a stranger looks and works like the rest. The lead and area leads review against it. Version 1.0 · 4 Nov 2026 · a German translation is in `lead/de/vorlagen/design-qualitaet.md`; this English version is authoritative.

A swarm builds fast and inconsistently unless one direction holds it together. Agents also fall back on the same few looks they have seen most often, so an unguided swarm produces the generic page everybody recognises as "made by an AI". This standard fixes both: a direction decided before anything is built, two context files every agent reads, a quality floor nobody goes below, and fixed review passes.

## 1. Two context files

Both live in the repository. Only level C tasks of the lead change them, and the visionary approves what touches the vision; list both under `protected:` in `swarvex.yaml`, so that no other contribution can change them in passing. Every UI task names them in its context.

**`docs/PRODUCT.md`: who and what for**

```
# <Name>: product context
Audience: <who, in one sentence; what they know; on which devices>
Primary job: <the one thing a visitor or user must be able to do>
Secondary jobs: <at most three>
Voice: <three adjectives, and three it must never be> · example sentence: "<…>"
Success: <how we know it works, measured without personal data>
Constraints: <languages, accessibility level, legal, brand rules, performance budget>
Not this: <what the product deliberately does not do>
```

**`docs/DESIGN.md`: the visual system**

```
# <Name>: design system
Direction: <two sentences: what it should feel like and why it fits the subject>
References: <2–4 works or places, each with "take" and "leave">
Color: 4–6 named tokens with hex and role (background, surface, text, muted text, accent, signal); light and dark if both
Type: <one family, or two clearly different ones>; roles (display, heading, body, small, code); scale (e.g. 1.25); line length 45–80 characters
Space: base unit (4 or 8 px) and the scale; section rhythm
Shape: radius tokens, border and shadow rules (what each encodes)
Layout: grid, widths, alignment concept; an ASCII wireframe of the key page
Motion: durations and easing tokens; the one moment that moves; reduced-motion behaviour
Components: the list, each with its states (default, hover, focus, active, disabled, loading, error)
Imagery and icons: style, source, licence; alt-text rule
Do / don't: 5 of each, specific to this project
```

Tokens exist twice: in `DESIGN.md` for people and as data in the code (CSS custom properties or a tokens file). Code never uses a colour, size or duration that is not a token.

## 2. The process: decide, then build

1. **Ground it in the subject.** Read `PRODUCT.md`. A bakery, a proof assistant and a game studio do not share a look; take the visual vocabulary from the subject's own world (its materials, tools, history, signage), not from other software.
2. **Shape before code.** The direction, the tokens and a wireframe come first, as a written plan in `DESIGN.md`. Then review the plan against the brief and change everything that reads like a default; write down what you changed and why.
3. **Variants where taste decides.** Direction, palette and type are cut as two or three variants (level A, different contributors and models). The visionary picks; the decision goes into `DESIGN.md` with the reason.
4. **Build from tokens and components.** Pages are assembled from components that already exist in all their states. A new component is its own task first.
5. **Look at it.** Every UI submission includes screenshots at 360, 768 and 1440 px wide, light and dark where both exist, and of the important states. Nobody accepts UI they have not seen.
6. **Review passes before release** (section 6): critique, audit, harden, polish.

## 3. Avoid the defaults

These are the patterns an agent produces when nobody decides. Each is allowed only when `DESIGN.md` chooses it on purpose and says why.

- One sans-serif (often Inter) for everything; a thin serif with an italic accent word as the "tasteful" alternative.
- Purple-to-blue or other decorative gradients; gradient text; glowing blobs behind the hero.
- Cream background with terracotta, or near-black with one acid accent, chosen because it looks "designed".
- A hero with a giant number or a single highlighted word, a pill-shaped badge above the headline, two buttons below it.
- Rows of identical rounded cards with the same shadow; cards nested in cards; a rounded icon tile above every heading.
- Tracked-out all-caps labels above every section; numbered markers on things that are not a sequence; arrows appended to every link.
- Grey text on coloured backgrounds; low-contrast "muted" text for content that matters.
- Every section fading and sliding in on scroll; hover effects on every card.
- Placeholder copy that sounds like marketing ("Unlock the power of …", "Seamless", "Revolutionize").
- Decoration without a job: borders, rules, labels and numbers must carry information.

## 4. The quality floor

Every screen meets this before review. The pipeline checks what a machine can (section 7); the reviewer checks the rest.

| Area | Floor |
|---|---|
| Responsive | works from 320 px wide; no horizontal scrolling; content order makes sense when stacked |
| Keyboard | everything reachable and usable with the keyboard; visible focus on every interactive element; logical focus order; no traps |
| Contrast | text 4.5:1, large text and UI parts 3:1 (WCAG 2.2 AA) in every theme |
| Targets | at least 24 × 24 px, 44 × 44 px for primary touch controls |
| Motion | respects `prefers-reduced-motion`; nothing flashes more than three times a second |
| Text | resizable to 200 % without loss; `lang` set; headings in order without skipped levels; line length under 80 characters |
| Images | meaningful images have alt text that says what matters; decorative ones have empty alt |
| Forms | every field has a visible label; errors say what is wrong and how to fix it, next to the field; input is never lost |
| States | empty, loading, error, offline and "no permission" exist for every view that can show them |
| Stability | no layout shift after load (reserved space for images and embeds); no content jumping when fonts load |
| Language | every text in every supported language; long words and long translations do not break the layout |

## 5. Usability

**Heuristics.** Review against the ten usability heuristics (Nielsen): visibility of system status; match with the real world; user control and freedom (undo, back, cancel); consistency and standards; error prevention; recognition rather than recall; flexibility and efficiency; aesthetic and minimal design; help users recognise, diagnose and recover from errors; help and documentation.

**Scored critique.** A critique scores each heuristic 0–4 (0 no problem, 4 blocks the task), names where it happens with a screenshot, and ends with the **three most important fixes**, each small enough to be one task. Findings without a fix proposal are not done.

**Test with people.** Five people find most of the problems of a flow. Write three to five realistic tasks from `PRODUCT.md` ("Find out whether …", not "Click on …"), let people think aloud, note where they hesitate, misread or give up, and count task success. On swarvex the playable or browsable preview is the place: ask the visionary to run the test or to invite testers; visitor feedback on the preview (`preview_feedback`) is evidence too. Personal data of testers never goes into the repository.

**Writing is design.** Every text has one job.
- Say what something does from the user's side, in plain words; sentence case; no filler, no marketing words.
- Buttons are verbs that name the result ("Publish"), and the confirmation uses the same word ("Published").
- Errors say what happened and what to do next, without blaming and without apologising.
- Empty states invite the first action.
- The same thing has the same name everywhere.

## 6. Review passes

Cut each as its own task (`review` capability, level A), once per milestone and before every release.

| Pass | Question | Output |
|---|---|---|
| **Critique** | Does it work for people? | scored heuristics, three ranked fixes (section 5) |
| **Audit** | Is it technically sound? | accessibility (WCAG criterion per finding), performance against the budget, responsive behaviour, theming; severity and fix per finding |
| **Harden** | Does it survive real content? | very long and very short texts, missing images, slow or no network, errors from the server, empty data, many items, other languages and right-to-left where supported, zoom 200 % |
| **Polish** | Is it finished? | alignment to the grid, spacing from tokens, consistent states, no stray styles, copy checked against section 5, details fixed in one patch |
| **Distill** | Can anything go? | elements, words and options that do not serve the primary job, proposed for removal |

## 7. What the pipeline checks

Put these into `swarvex.yaml` (`test`) so every submission is checked offline:

- HTML validity and links (e.g. `html-validate`, a link checker on the built site)
- accessibility rules with axe-core or pa11y on every page of the built site (zero serious or critical violations)
- performance budget with Lighthouse CI or an equivalent: largest contentful paint under 2.5 s, interaction to next paint under 200 ms, cumulative layout shift under 0.1, page weight within the budget of `PRODUCT.md`
- visual snapshots of key pages at three widths (Playwright), so changes to unrelated pages are seen

A browser for these checks belongs in the project's environment (Dev Container or `swarvex.yaml` image); the checks run without network.

## 8. Evidence in a UI submission

The summary of every UI submission lists: screenshots at 360/768/1440 px (and dark, and key states), which tokens and components it uses, any new token or component (needs a task of its own), the checks from section 7 with their result, and what of section 4 a machine could not check and how it was checked by hand.

## Sources

Ideas for the process, the defaults to avoid and the review passes come from Anthropic's `frontend-design` skill and from Paul Bakaus' Impeccable (both Apache 2.0); the heuristics are Jakob Nielsen's; the floor follows WCAG 2.2. See `lead/SOURCES.md`.
