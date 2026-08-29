# Agent Instructions

This repository is danser-ee, a maintained fork of
[Wieku/danser-go](https://github.com/Wieku/danser-go) - a Go GUI/CLI
visualizer for osu!standard maps that renders replays and records them to
video. Treat it like sensitive, long-lived production software.

After this file, read `.project/docs/index.md` and the core expectations in
`.project/docs/expectations/user-and-project.md` and
`.project/docs/expectations/behavioral-expectations.md`. Then use
`.project/docs/expectations/index.md` to find situational knowledge relevant to the
work.

## Engineering Standard

Treat every change as long-lived production code maintained by multiple
senior engineers over time. Optimize for clarity, explicit ownership,
stable boundaries, and future modification.

Use clear names, small cohesive types and functions, and explicit
separation between replay parsing, rendering, audio scheduling,
settings/configuration, UI, and native interop. Keep responsibilities
narrow. Do not hide cross-layer behavior behind convenience abstractions
unless the boundary and ownership model remain obvious.

Default toward writing more comments, not fewer. Treat missing context as
the more expensive failure than an occasional comment a reader didn't
strictly need: a maintainer who has to reconstruct intent from git blame,
native-library docs, and cross-file reading costs far more than a sentence
that turned out to be slightly redundant. When unsure whether reasoning is
worth writing down, write it down.

Write comments that explain why a function, type, branch, constraint,
workaround, invariant, or integration detail exists, not just what the code
does. Capture assumptions, lifecycle expectations, timing invariants,
platform behavior, vendor quirks (the BASS, SDL3, and OpenGL interop
surface especially), concurrency concerns, error-handling rationale, and
non-obvious tradeoffs. Also comment: non-obvious control flow, a choice
made between two plausible approaches and why the chosen one won, state or
data that crosses a module or boundary, anything that took investigation,
reference-source reading, or a bug report to get right, and any code a
future reader might be tempted to simplify in a way that would silently
reintroduce a bug.

Judge comments against the returning maintainer described below, not
against an idealized expert who already holds full context. Do not withhold
a comment on the assumption that a sufficiently strong engineer would
already know it; the whole point of the comment is to save that engineer
the reconstruction work. The only comments to skip are ones that just
restate the literal syntax of the line beneath them with no added
reasoning.

Do not use decorative separators, banner comments, ASCII art, boxed
sections, or other purely decorative comment formatting.

Write Go doc comments liberally on exported identifiers to clarify public
behavior, ownership, invariants, failure modes, and integration contracts.
Internal implementation comments should be as long as the underlying
reasoning requires; do not compress a rationale into a fragment if it needs
a full sentence or two to hold together.

Before finishing, reread the changes as a senior engineer returning six
months later with no memory of writing this code. Add any comment you find
yourself wishing were there, and improve unclear structure, naming,
ownership, error handling, documentation, and intent before calling the
work done.

Use only ASCII punctuation in repository text and source comments. Do not
use curly quotes, smart quotes, em dashes, en dashes, or similar Unicode
punctuation. Use `'`, `"`, and `-` instead.

## Documentation Authority

Use production code, builds, real program runs, and recent git history to
establish what exists. `.project/docs/index.md` defines the documentation categories
and their authority boundaries. Use `.project/docs/project-roadmap.md` for global
milestone status and delivery sequence. Delivered work is recorded in the
roadmap, plus an owning record under `.project/docs/implementation-history/`, in the
same change that ships it. Specialized plans under `.project/docs/plans/`, once any
exist, own their subsystem's architecture and remaining design; register a
new plan's authority in `.project/docs/index.md` in the change that creates it.

Use `.project/docs/expectations/` for durable operational knowledge about this
repository, machine, and user - never for product priority or active task
state. Use `.project/docs/implementation-history/` only to understand why a
completed slice took its recorded shape; it is explanatory evidence, never
current status.

`.project/docs/osu!lazer source code/` is a shallow single-branch clone of ppy/osu
master kept as the local citation authority for upstream osu! behavior.
Cite it with concrete source paths; never edit it, and never commit it -
it stays outside this repository's history by policy. History and blame are
unavailable locally by design; the refresh procedure lives in
`.project/docs/expectations/workarounds.md`.

When two documents disagree, do not append another contradictory update.
Verify the implementation, correct the authoritative document, and prune or
reframe the stale claim in the same change.

Active documentation should describe the current project, current
intentional relationships, and useful next implementation context. Delete
or consolidate superseded plans, rejected approaches, and stale detail when
they no longer help current work; Git provides historical archaeology. If
an old lesson still constrains current implementation, state that current
constraint in the current owning documentation. Do not retain obsolete text
merely to preserve its history.

## Go Guidance

Module `github.com/wieku/danser-go`. Go is scoop-managed on this machine
and satisfies the `go.mod` toolchain pin natively; any future pin above the
installed release auto-downloads via GOTOOLCHAIN=auto. The JetBrains-injected
`GOROOT` caveat is recorded in `.project/docs/expectations/workarounds.md`. Run
`gofmt` cleanliness as mandatory and follow standard Go idiom: short
lowercase package names, exported identifiers only when needed by callers.

The build uses CGO throughout - BASS audio, SDL3, and Dear ImGui bindings -
so a MinGW-family gcc is part of the toolchain. The machine's current gcc
provenance and its linking caveat are recorded in workarounds; treat
linking success as compile evidence only until a run has exercised the
audio path after a toolchain change.

Preserve upstream's architectural separation: `app/` owns gameplay-facing
core logic, `framework/` is the upstream-owned graphics/input layer, and
parsing, rendering, audio scheduling, and UI concerns stay in their layers.
Divergence from upstream structure must be deliberate and recorded in
documentation rather than incidental, and commits stay small and
single-concern so future synchronization with upstream dev remains
tractable.

Native interop deserves specific care: BASS and SDL handles are unmanaged
resources whose lifetime mistakes crash the process rather than panic
cleanly. Comment non-obvious handle ownership, teardown ordering, and any
call whose failure mode is silent.

Upstream ships no unit tests (`*_test.go` count is zero), so `go test ./...`
verifies nothing. Adding focused table-driven tests for isolated pure logic
is acceptable when the maintenance cost is justified; do not scaffold test
frameworks or broad suites unprompted.

## Go Skills

Route Go writing, reviewing, and refactoring through the vendored skills
under `.agents/skills/`. Start from `go-skills-router` when a task spans
several concerns or ownership between skills is unclear; when a task maps
cleanly to one skill, load that skill directly. The router indexes an
upstream collection larger than this checkout - its tables sometimes name
skills that are absent here (for example `go-code-review`); fall back to
the closest present owner instead of hunting for missing ones.

Two collections coexist by scope:

- Flat `go-*` skills own engineering practice: coding standards, error
  handling, context/concurrency review, performance and test quality,
  troubleshooting, modernization.
- `cc-skills-golang/skills/golang-*` covers the library ecosystem: DI
  frameworks (uber-fx, google-wire, samber-do), cobra/viper/testify/slog-
  class libraries, gopls, linting, CI, OpenAPI. Prefer it when work centers
  on those libraries or tools.

`git-commit` generates Conventional-Commits-style messages; the commit
convention below overrides its subject format - use it for scoping and
atomicity discipline only.

Treat all three as vendored references: read and follow them, edit them
only as a deliberate vendor-update task.

## Verification

Default bar per change: `gofmt -l` clean on touched files, `go build ./...`
compiling the tree, and a runnable artifact via `go build .` when behavior
changed. Rendering, audio, timing, or skinning changes additionally need
stated human-verification steps precise enough to execute mechanically:
command, inputs, expected observable behavior. A human performs them; the
change states them exactly.

If a check cannot be run, report exactly what was skipped and why. Broader
evidence applies when the change touches packaging or distribution scripts,
upstream merge resolution, or toolchain-sensitive behavior - in those
cases prove the launcher and main binary both build and launch from the
repository root.

First builds after dependency changes take minutes under CGO. Long-running
commands do not need periodic progress commentary; wait silently and report
in the final handoff.

## Agent Progress Updates

Long-running commands do not need periodic progress commentary. Start the
command and wait silently for completion. Send an interim update only when
the user asks, user action is required, a material result changes the task,
or a stalled command requires a decision. The final handoff must still
report the completed verification and any failure or skipped check.
