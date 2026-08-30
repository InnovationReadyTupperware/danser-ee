# Contributing

## Read this first

Danser-ee is not actively accepting general contributions right now.

You can still report a problem or open a pull request, but please do so
knowing that there is a high chance the work will be deferred, closed, or
implemented by the maintainer instead.

GitHub Issues are the intake for everything: bug reports, feature requests,
questions, and technical proposals. Search existing issues before opening a
new one, including closed issues where possible. A non-trivial change should
have an issue before its pull request; that gives the maintainer a chance to
redirect the work before somebody spends time implementing the wrong thing.

Even without a coding harness, read [`AGENTS.md`](AGENTS.md) before changing
code. It covers project-specific behavior, surface coverage, error handling,
logging, and verification details that are easy to miss.

## What we are most likely to accept

- Small, focused bug fixes with a reproducible problem.
- Small reliability or performance improvements with evidence of the issue.
- Narrow compatibility fixes that preserve existing workflows.
- Tests, documentation, or maintenance changes that clearly support the
  current product without expanding its scope.

## What we are least likely to accept

- Large features or changes spanning several unrelated concerns.
- Drive-by rewrites, opinionated refactors, or broad formatting churn.
- New dependencies or platform requirements without an issue explaining why
  they are necessary.
- Changes that alter replay, timing, scoring, rendering, audio, or output
  behavior without focused verification and a clear compatibility story.

## Reporting an issue

Use [GitHub Issues](https://github.com/InnovationReadyTupperware/danser-ee/issues)
for bugs, feature requests, questions, and proposals.

For a bug, include the danser version or commit, operating system, relevant
mode and command-line flags, map or replay details, steps to reproduce, and
the relevant `danser.log` or `launcher.log` contents when available.
Screenshots or a short recording are especially useful for rendering, timing,
audio, launcher, and gameplay window problems. The issue template lists the
machine details that are most useful for graphics and sound failures.

For a feature, question, or proposal, describe the user problem, the desired
observable behavior, and any constraints or modes that must continue to work.

## If you still want to open a pull request

Keep it small and focused on one concern. Link the relevant issue and explain
exactly what changed, why it should exist, and how the result was verified.
Do not mix unrelated cleanup or speculative redesign into the same change.

Use the repository's existing Conventional Commits style for the title, with
the affected area named explicitly, such as `fix(gameplay): ...`,
`feat(launcher): ...`, or `docs(readme): ...`.

If the change affects the launcher, gameplay window, CLI, recording, audio,
timing, or another user-visible surface, say which surfaces you checked. UI
and gameplay presentation changes need clear before and after images. Changes
that depend on motion, timing, transitions, or interaction details should
include a short video.

Attach review evidence to the pull request instead of committing temporary
screenshots, videos, or other PR-only files.

Update user-facing documentation or CLI help when behavior changes make the
existing explanation inaccurate. Do not include generated binaries, local
configuration, secrets, or other machine-specific artifacts.

## Be realistic

Opening a pull request does not create an obligation on our side.

We may defer it, close it, ignore it, ask you to shrink it, or reimplement the
idea ourselves later.

If you are fine with that, proceed.
