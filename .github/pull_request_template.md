<!--
Keep pull requests small and focused. Link the Issue that motivated this change.
If the change is not ready for review, open it as a draft.
-->

## What changed

<!-- Describe the implementation and keep the scope tight. -->

## Why

<!-- Explain the problem being solved and why this approach is appropriate. -->

## Related issue

<!-- Use "Closes #123" when this PR fully resolves an Issue. -->

## Verification

- [ ] I ran `go vet ./...`.
- [ ] I ran `scripts/test.ps1 ./...` on Windows or `bash scripts/test.sh ./...` on Linux/macOS.
- [ ] I manually checked the affected behavior when automated tests were not sufficient.

## Product surfaces checked

- [ ] Launcher
- [ ] Gameplay window
- [ ] CLI or non-interactive output
- [ ] Replay, cursor-dance, osu!standard play, or knockout modes
- [ ] Recording or screenshot output
- [ ] Not applicable

## User-facing evidence

<!--
For UI changes, include before/after screenshots.
For animation, timing, audio, or interaction changes, include a short video.
Delete this section when not applicable.
-->

## Checklist

- [ ] This PR is small and focused on one concern.
- [ ] I updated user-facing documentation or CLI help when needed.
- [ ] I did not include generated binaries, local configuration, logs, replays, videos, or secrets.
