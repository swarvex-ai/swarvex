# Baseline packages per project type

> For phase 2 (LEAD.md, "Planning", step 0). The interview trees settle **decisions**; the baseline packages list the **building blocks** every project of a type needs, whether or not anyone mentioned them: a game needs a main menu, an app an error page, a CLI `--help`. Visionaries rarely ask for these, and that is exactly why they get forgotten.
> The packages are a minimum. Add anything the specific project needs.

## How to use them

1. **Pick the packages:** "All projects" plus the package of the project type. A hybrid takes the union (a game with a level editor: game + tool). A clean-room rebuild takes the package of the type it rebuilds.
2. **Every item gets a decision**, none stays silent:
   - **Mx**: in milestone x (it becomes epics and tasks in the plan),
   - **later**: with the milestone where it is due,
   - **not needed**: with a one-sentence reason; it also goes into the non-goals.
   Your recommendation follows the column "Default".
3. **Record the result** in the project brief, section "Baseline package" (one row per item). The visionary confirms it together with the brief. For a project that already has a confirmed brief, ask it as one small interview round (LEAD.md, phase 0, "Small interviews along the way").
4. **Turn items into tasks** like any other work: usually a design or specification task first (`design.ui`, `text.en`), then the implementation. Build one reference (one menu screen, one settings page, one command) as an extension point, so the rest become level B tasks.
5. **Check again at every milestone sign-off** (LEAD.md 7.1): is everything that was due in this milestone done?

**Default column:** **first** = the first playable or usable milestone (the vertical slice), **v1** = before version 1, **release** = before a public release, **if** = only when the condition in the item applies.

## All projects

| ID | Building block | Done when | Default |
|---|---|---|---|
| B1 | **README** | says what it is, shows a screenshot or example, explains how to install or start it and how to contribute | first |
| B2 | **Licenses and credits** | `LICENSE` files for code and assets; `CREDITS.md` with contributors, models and services used, third-party licenses | first |
| B3 | **Version and "about"** | the product shows its version; an about screen, `--version` or footer names license and project page | v1 |
| B4 | **Changelog and versioning** | `CHANGELOG.md`, version scheme decided (e.g. SemVer), every release has an entry | v1 |
| B5 | **Release build** | a reproducible build per target platform, with download or install instructions | v1 |
| B6 | **Security contact** | `SECURITY.md`: how to report a vulnerability without making it public | release |
| B7 | **Bug report path** | issue template with steps, expected and actual result, version, system | v1 |
| B8 | **Name, logo, icon** | name checked for conflicts, logo and icon in the sizes the platforms need | v1 |
| B9 | **Project page material** | short description, 3–5 screenshots or a short clip for swarvex.ai and stores | release |

## Game

| ID | Building block | Done when | Default |
|---|---|---|---|
| G-1 | **Start and title** | splash with required logos (engine, licenses), title screen with name and logo; from launch to title in under 5 s | first |
| G-2 | **Main menu** | continue, new game, options, credits, quit; fully usable with gamepad and keyboard alone, focus always visible | first |
| G-3 | **Pause menu** | resume, options, restart from checkpoint, back to main menu; game and sounds pause; opens on focus loss | first |
| G-4 | **Options** | audio (master, music, effects, voice), display (window mode, resolution, VSync, frame limit), controls (remapping, sensitivity), language; saved and applied on the next start | first (remapping: v1) |
| G-5 | **Accessibility basics** | subtitles, screen shake and flashing switchable, flashing ≤ 3 Hz, text size, danger readable without color alone | first |
| G-6 | **Input** | keyboard/mouse and gamepad; button glyphs follow the last used device; controller hot-plug; no dead end that needs a mouse | first |
| G-7 | **Saving** | checkpoints or autosave, settings stored apart from progress, versioned save format, a broken save file does not crash the game | first |
| G-8 | **Onboarding** | the first minutes teach the controls through play (prompts, safe space), not through text walls | first |
| G-9 | **HUD and feedback** | health and resources, hit feedback, interaction prompts; readable at 1080p and on a handheld screen | first |
| G-10 | **Death, retry, ending** | death or failure screen, quick retry, end screen with credits | first |
| G-11 | **Menu feel** | menu transitions and animations, UI sounds (move, confirm, back, error), menu music; consistent with the style guide | v1 |
| G-12 | **Loading** | loading screen or indicator; no freeze longer than 1 s without feedback | v1 |
| G-13 | **Credits in the game** | contributors, models and services, third-party licenses, reachable from the main menu | v1 |
| G-14 | **Ready for translation** | all player-facing texts in a table, no texts in code, font covers the planned languages | first |
| G-15 | **Performance and quality levels** | target frame rate on reference hardware measured; quality presets (low/medium/high) | v1 |
| G-16 | **Window behavior** | focus loss pauses, resolution and window mode change without restart, clean quit | v1 |
| G-17 | **Logs and crashes** | log file in a documented place, crash leaves a readable report, players know how to send it | v1 |
| G-18 | **Audio mix** | volume levels balanced, important sounds stay audible (ducking) | v1 |
| G-19 | **Release material** | store page text, screenshots, trailer, key art, press kit | release |
| G-20 | **Achievements, cloud saves, platform services** | only when a store or platform requires or the visionary wants it | if |

## App (web, mobile, desktop)

| ID | Building block | Done when | Default |
|---|---|---|---|
| A-1 | **App shell and navigation** | main navigation, back behaves as expected, deep links or URLs per screen | first |
| A-2 | **First start** | short onboarding or an empty state that explains the next step; optional sample data | first |
| A-3 | **States of every screen** | loading, empty, error, offline and "no permission" designed and implemented, not only the happy path | first |
| A-4 | **Forms** | inline validation, understandable error messages, warning before losing unsaved input | first |
| A-5 | **Feedback and safety** | confirmations, undo or confirm dialog for destructive actions | first |
| A-6 | **Settings** | language, light/dark theme, notifications, and everything else the brief allows users to change | v1 |
| A-7 | **Accounts** | sign up, sign in, sign out, reset access, delete account | if user accounts |
| A-8 | **Accessibility** | WCAG 2.2 AA: contrast, keyboard, visible focus, screen reader labels, reduced motion respected | first |
| A-9 | **Responsive** | phone, tablet and desktop widths, both orientations where relevant | first |
| A-10 | **Motion and micro-interactions** | transitions and states (hover, pressed, loading) from the design tokens; off when reduced motion is set | v1 |
| A-11 | **Ready for translation** | texts in a table, dates, numbers and currencies formatted per language | first |
| A-12 | **Legal pages** | privacy policy, licenses and credits; imprint and terms where required | release |
| A-13 | **User data rights** | export and deletion of one's own data | if user data |
| A-14 | **Error pages and reporting** | 404 and error page with a way back; error reports only with consent | v1 |
| A-15 | **Performance budget** | start time and size measured against a budget in the test pipeline | v1 |
| A-16 | **Icons and previews** | app icon, favicon, splash screen, social and store preview images | v1 |
| A-17 | **Updates and migration** | update notice, local data survives an update | if installed app |
| A-18 | **Distribution** | store listing, PWA manifest or installer, signing | release |
| A-19 | **Help** | help or FAQ for the main flows, about screen with version | v1 |

## Website (landing page, documentation, content)

| ID | Building block | Done when | Default |
|---|---|---|---|
| W-1 | **Layout and navigation** | header, footer, navigation that works on phones | first |
| W-2 | **Home page** | says in one screen what it is and offers one clear next step | first |
| W-3 | **404 page** | with a way back and search or main links | first |
| W-4 | **Responsive and accessible** | phone to desktop widths, WCAG 2.2 AA | first |
| W-5 | **Search engine basics** | title and description per page, sitemap, `robots.txt`, preview images (Open Graph) | v1 |
| W-6 | **Legal pages** | privacy policy, imprint where required; no cookie banner unless something needs consent | release |
| W-7 | **Performance** | optimized images, no layout shift, budget measured in the pipeline | v1 |
| W-8 | **Contact** | a way to reach the people behind it | v1 |
| W-9 | **Search** | full-text search | if documentation |
| W-10 | **Languages** | language switch, translated pages linked to each other | if several languages |
| W-11 | **Statistics** | none, or privacy-friendly without personal data | if wanted |
| W-12 | **Icons** | favicon, touch icon | first |
| W-13 | **Deployment** | automatic build and preview per change, documented hosting | first |

## Web service or API (server, backend)

| ID | Building block | Done when | Default |
|---|---|---|---|
| S-1 | **API contract** | machine-readable (e.g. OpenAPI), versioned, with sample requests and responses | first |
| S-2 | **Sign-in and permissions** | who may do what is tested, not only documented | if users or clients |
| S-3 | **Input validation and error format** | every input checked; one error format with code, message and hint | first |
| S-4 | **Health check** | health and readiness endpoints for monitoring and the orchestrator | first |
| S-5 | **Configuration** | via environment or file, documented example, no secrets in the repository | first |
| S-6 | **Logs and metrics** | structured logs without personal data, basic metrics (requests, errors, duration) | v1 |
| S-7 | **Database migrations** | versioned migrations, seed data for development | if database |
| S-8 | **Limits** | rate limits, maximum request size, timeouts | v1 |
| S-9 | **Backup and restore** | backup documented and a restore actually rehearsed | if persistent data |
| S-10 | **Deployment** | container image and an example setup (e.g. Compose) with instructions | v1 |
| S-11 | **Security** | TLS, dependency updates, security headers, `SECURITY.md` | v1 |
| S-12 | **Data protection** | retention periods, deletion, overview of stored personal data | if personal data |
| S-13 | **Clean shutdown** | running requests finish, no half-written data | v1 |
| S-14 | **Local development** | starts with one command, including the services it needs | first |

## Tool (command line)

| ID | Building block | Done when | Default |
|---|---|---|---|
| C-1 | **Help** | `--help` for the tool and every command, with examples; `--version` | first |
| C-2 | **Error messages** | say what went wrong and what to do; on stderr | first |
| C-3 | **Exit codes** | documented and stable; 0 only on success | first |
| C-4 | **Configuration** | file, environment and flags with a documented order of precedence | v1 |
| C-5 | **Output for scripts** | `--json` or another stable format, `--quiet` and `--verbose` | v1 |
| C-6 | **Colors** | only on a terminal, `NO_COLOR` respected | v1 |
| C-7 | **Installation and removal** | packages, binaries or a one-liner; removal documented | v1 |
| C-8 | **Shell completion** | bash, zsh, fish | release |
| C-9 | **Reference** | man page or reference page generated from the help texts | release |
| C-10 | **Platforms** | paths, line endings and encodings correct on every supported system | v1 |
| C-11 | **Interruption** | Ctrl-C ends cleanly, no half-written files | first |
| C-12 | **Progress** | progress display for long operations | v1 |
| C-13 | **Dry run** | `--dry-run` for operations that change or delete data | if destructive operations |

## Library or SDK

| ID | Building block | Done when | Default |
|---|---|---|---|
| L-1 | **Public interface** | clearly separated from internals, every public element documented, reference generated | first |
| L-2 | **Quickstart** | from installation to the first working call in under 5 minutes | first |
| L-3 | **Examples** | an examples folder that runs in the test pipeline | v1 |
| L-4 | **Versioning and deprecation** | SemVer, changelog, deprecation warnings at least one minor version before removal | v1 |
| L-5 | **Package** | published in the language's registry with complete metadata | release |
| L-6 | **Supported versions** | runtime and language versions tested as a matrix in the pipeline | v1 |
| L-7 | **Types** | type information for the languages that use it | v1 |
| L-8 | **Errors** | distinguishable error types instead of generic exceptions or text | first |
| L-9 | **No hidden state** | no global state; thread safety documented | first |
| L-10 | **Dependencies** | as few as possible, licenses compatible | first |
| L-11 | **Benchmarks** | reproducible measurement in the pipeline | if performance matters |
| L-12 | **Migration guide** | for every breaking change | if breaking changes |

## Plugin or extension (for a host program)

| ID | Building block | Done when | Default |
|---|---|---|---|
| P-1 | **Compatibility** | supported host versions declared and tested | first |
| P-2 | **Install and remove** | through the host's usual way; removal leaves nothing behind | first |
| P-3 | **Settings** | where the host keeps settings, with sensible defaults | v1 |
| P-4 | **Fits the host** | follows the host's UI guidelines, shortcuts and terms | v1 |
| P-5 | **Fails gracefully** | an error in the plugin never takes the host down and is reported understandably | first |
| P-6 | **Marketplace listing** | description, screenshots, version notes | release |
