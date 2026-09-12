# Agent Instructions for danser-ee

## Summary

`danser-ee` is a maintained fork of `danser-go`, a Go application that turns
osu!standard beatmaps and replay data into interactive gameplay views and
rendered media.

It combines interactive gameplay, visualization, and media output around one
gameplay core. A change can affect what users see, hear, judge, or save, so
treat those observable results as part of the feature rather than incidental
implementation detail.

## What matters here

The project sits at the intersection of game rules, animation, audio, and
video. The promises worth protecting are simple:

- Replay playback should remain faithful to the source client the replay came
  from.
- Gameplay, audio, rendering, and recording should agree on one timeline.
- The launcher, gameplay window, and command-line workflows should not quietly
  grow different behavior for the same feature.

## Hit every surface

The most common danser regression is a change that works on the path someone
happened to test and is missing everywhere else. Before calling a feature done,
walk through the surfaces that apply and say which ones you checked.

- **Entry points.** Running `danser` without arguments opens the GUI launcher;
  arguments enter the command-line/gameplay path. Shared behavior deserves a
  decision in both paths, even when one path only needs a small adapter.
- **Launcher.** Map catalog and search, settings, replay and file-drop input,
  mode selection, child-process progress, and launcher dialogs are all part of
  the product. When the launcher builds arguments, verify that the gameplay
  process receives and honors them.
- **Gameplay window.** This is the SDL/OpenGL window where users watch or play
  a map. Check the relevant playback, seek, pause, restart, overlay, audio,
  and failure paths. It can be opened directly from the CLI or started by the
  launcher.
- **Command-line and non-interactive output.** Flags cover map and replay
  selection, generated movement, TAG and knockout modes, recording, and
  screenshots. Exercise the direct invocation when a change touches any of
  those contracts.
- **Modes and participants.** Cursor dance, replay playback, play mode,
  replay-driven knockout, and solo knockout do not all obtain movement or
  judgement from the same source. Include generated participants, replay
  participants, and mixed Stable/Lazer lineups when the change can reach them.
- **Outputs.** Watching, recording a video, and taking a screenshot have
  different timing and finalization paths. Check the output file, progress,
  audio sync, and failure behavior that apply.
- **Platforms.** Windows x64 is the primary target and Linux is best effort.
  Native dialogs, filesystem paths, OpenGL setup, and bundled runtime files
  can behave differently even when the Go code is shared. macOS is unsupported.

## Errors users can see

An error that exists only in a log is invisible to most GUI users. Route the
failure according to the process that owns it, not according to whether the
overall session began from the CLI or the launcher:

- **CLI:** write an actionable error to the normal log or command-line output,
  include the operation and relevant map, replay, or output, and return a
  failure result. If a visual dialog is needed on the direct CLI/gameplay path,
  it must be native; launcher in-app popups are not available there.
- **GUI launcher:** use an in-app modal for launcher-owned failures only when
  the launcher is initialized, rendering, responsive, and able to receive
  input. If the launcher may be blocked or unresponsive, use the native platform
  dialog path instead.
- **Gameplay child/window:** failures originating in the gameplay process use
  the gameplay/native dialog path regardless of whether that process was
  started directly from the CLI or spawned by the launcher. Once the SDL
  window exists, parent native dialogs to it when possible. This includes
  runtime failures such as graphics, audio, rendering, recording, or resource
  allocation errors.
- **Early startup and fatal paths:** if SDL, ImGui, or the normal UI cannot be
  created or trusted to keep rendering, use the platform's native fallback and
  still preserve a useful log entry.

## Logging

Danser uses Go's standard `log` package. Startup configures it through
`platform.StartLogging`: the launcher and gameplay process each write a named
log under the runtime data directory and mirror messages to standard output.
That makes the log useful for a CLI session without assuming that a GUI user
has a console open.

- Prefix messages with the subsystem that owns them, as in `Launcher:`,
  `DatabaseManager:`, or `SettingsManager:`. Describe the operation and the
  object involved, then include the error with `%v` or `%q` formatting as
  appropriate.
- Keep normal lifecycle and progress messages readable. Use an explicit
  `Warning` or `Failed` message when work was skipped, degraded, or could not
  complete; do not silently swallow an error that changes the requested
  result.
- Logging is diagnostic evidence, not the GUI error surface. Pair an
  operation failure with the dialog or gameplay-window error required above,
  and keep the CLI log actionable enough to identify the failing operation and
  its inputs.
- Prefer returning or propagating errors over introducing `log.Fatal` or
  `log.Panic` in library and worker code. Those calls terminate before the
  owning lifecycle can clean up resources or choose the correct visual surface.
- Never put credentials, access tokens, refresh tokens, or other secret values
  in a log. Log a safe identifier or path when it is needed to diagnose the
  operation.

## Common ways to break the experience

Use these broad failure modes as review prompts when a change crosses more
than one part of the product.

1. **Fix only the path you tested.** Check the relevant entry points, modes,
   and output types so shared behavior does not diverge between workflows.
2. **Lose track of provenance.** Keep rules and presentation aligned with the
   source and mode that own them; avoid turning a participant- or map-specific
   decision into a global switch.
3. **Let timelines diverge.** Gameplay, audio, rendering, seeking, and
   recording need an explicit relationship so the result remains coherent.
4. **Blur ownership and lifecycle.** Make resource ownership, thread
   boundaries, cancellation, synchronization, and teardown order explicit,
   especially around native resources and background work.
5. **Hide or lose failures.** Surface errors where the user is working, keep
   diagnostics actionable, and preserve recoverable output when a later step
   can fail.

## Glossary

- **osu!standard:** The osu! ruleset this project visualizes and plays. Other
  osu! rulesets are outside the supported gameplay path.
- **osu!stable:** The original desktop osu! client. A replay from the stable
  generation selects danser's Stable-compatible gameplay behavior.
- **osu!lazer:** osu!'s newer client and rules implementation. Lazer-versioned
  replays and playback without replay provenance use danser's Lazer behavior.
- **PP rework / SR/PP rework:** Player-facing shorthand for an osu!standard
  performance points and star-rating update.
- **osu! replay (`.osr`):** A file containing recorded play input and metadata.
  Danser loads it alongside the beatmap and renders and judges the play; it is
  not a pre-rendered video.
- **Cursor dance:** Generated cursor movement derived from the beatmap and
  cursor-dance settings instead of a player's recorded input.
- **Singular slider:** A slider with no usable traversal path or positive time
  span, handled as one effective point where traversal is required.
- **Pathological slider:** A valid slider routed through bounded presentation
  fallbacks because its geometry or derived workload is unusually expensive.
- **TAG:** A generated multi-cursor arrangement where cursors take turns on
  objects. In solo knockout, the generated cursors are full scored
  participants instead.
- **Mandala:** Repeated or mirrored views of cursor movement and hit objects,
  used for collage-style output. The `-cursors` setting controls the number of
  repeated views.
- **Knockout:** A multi-participant mode that ranks players and removes them as
  they fail. Participants can come from selected replays or be generated by
  danser.
- **Solo knockout:** Map-driven knockout with generated Danser participants;
  each participant is scored as a full play while sharing cursor-dance
  controls.
- **Classic (CL):** The Classic compatibility setting or mod. It restores
  selected legacy presentation or behavior without changing whether a
  participant's gameplay provenance is Stable or Lazer.
- **Danser gameplay window:** The SDL/OpenGL window that displays map playback
  or live play. It is separate from the launcher and is also the visual error
  surface once it has been created.
- **Launcher:** The no-argument GUI for catalog browsing, settings, mode
  selection, and supervising a gameplay or recording process.

## Engineering rules

- Keep changes straightforward and maintainable in the existing code. Prefer
  clear names, small cohesive types and functions, and local patterns; avoid
  broad reorganizations or abstractions unless the task calls for them.
- Use standard Go naming and idioms. The module is
  `github.com/innovationreadytupperware/danser-ee` and the toolchain target is Go 1.27.1.
- Preserve public API names and serialized configuration keys unless a
  compatibility-preserving migration is part of the task.
- Document exported identifiers and non-obvious behavior. Comments should
  explain intent, invariants, lifecycle, platform or concurrency assumptions,
  and cross-layer decisions rather than restate syntax.
- Prefer concise comments without trailing punctuation when they express a single short thought.
- Before finishing, reread the change as a maintainer returning six months
  later. Resolve unclear naming, ownership, error handling, or intent while the
  surrounding context is still available.
- Use US English and ASCII punctuation in new identifiers, labels, comments,
  and documentation. Do not add decorative banner comments or separators.
- The upstream maintainer intentionally uses lowercase `danser` as the short
  brand; this is a branding convention, not a typo. Preserve that spelling in
  the middle of sentences, settings descriptions, logs, and inherited runtime
  labels. Capitalize it as `Danser` when it begins a normal sentence. Use
  `danser-ee` for the fork's compact name and reserve `Danser Enterprise
  Edition` for deliberate long-form display branding. Do not globally
  title-case `danser`.

## Development and verification

The program uses CGO, native runtime files, and repository assets, so run
development commands from the repository root and keep that directory as the
working directory for launched binaries.

- Run `gofmt -l` on touched Go files and leave the result clean.
- Run `go build ./...` to compile all packages.
- When behavior changes, also run `go build .` and exercise the resulting
  program from the repository root.
- Prefer `./scripts/test.ps1 ./...` on Windows or `bash scripts/test.sh ./...`
  on Linux and macOS. The wrappers arrange the native library search path and
  an ignored build cache for test binaries.
- Rendering, audio, timing, or skinning changes require a human-verification
  note with the exact command, inputs, and expected observable behavior.
- For packaging, release scripts, or toolchain changes,
  validate the affected launcher and main artifacts from the repository root;
  report any platform check that could not run.

## Pull requests

- Never make a PR unless the developer explicitly asks you to do so.
- Conventional commit titles, plain language:
  `fix(gameplay): show startup errors in the gameplay window`.
- Body: explain the problem in a sentence or two, then how you fixed it. End
  with the model and harness that did the work.
- UI changes need before/after images. Motion or timing needs a short video.
- Upload PR evidence to GitHub. Never commit PR-only screenshots or assets such
  as `.github/pr-assets/`.
- One concern per PR. If the description says "also", split it.

## Plans and work artifacts

- Do not commit implementation plans, research notes, or agent scratch files.
  Keep temporary working material outside the worktree.
- Track active maintainer work in the GitHub Issue that owns it. External
  proposals use GitHub Issues and follow [`CONTRIBUTING.md`](CONTRIBUTING.md).
- Put durable architecture, constraints, and decisions in `README.md` or a
  focused documentation page when one exists. Update documentation when the
  product changes so it describes current behavior instead of abandoned
  intentions.
- A merged PR is the implementation record. Close or update its linked issue
  when the work lands; do not preserve a second checklist in the repository.
