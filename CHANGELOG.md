# Changelog

## 1.0.0 - 2026-09-05

## What's new in Danser Enterprise Edition 1.0.0

### Highlights

- Gameplay now follows osu!lazer's model
  - Lazer-compatible behavior is the default, while osu!stable replays receive `CL` / Classic compatibility automatically
  - The old `LZ` workaround is no longer needed
- Hitsounds and storyboard audio now occur at their intended times, and video recordings stay synchronized through speed changes and audio stalls
- Song catalog updates no longer hold up launcher startup
- High-refresh gameplay is smoother, especially with custom FPS caps on Windows
- Solo knockout can now run with generated participants instead of replay files
- Lazer-compatible sliders now judge heads, ticks, repeats, and tails separately and can show skin-aware markers for supported misses

### Breaking changes

- OpenGL 4.5 core is now required

### Added

- Added replay-free solo knockout with fully scored participants
  - New `-solo-knockout` flag to enable it, with `-tag` setting the participant count and `-cursors` the mirrored cursor views
- Added Lazer-compatible osu!standard judgments for slider parts
  - New `Objects.Sliders.ShowSliderJudgmentMarkers` displays supported misses with the `slidertickmiss` and `sliderendmiss` skin components
- Added Lazer-compatible hit-error statistics and detailed feedback controls
  - Includes timing lines, customizable bars, and median hit error on the results screen
  - Added `Gameplay.HitErrorMeter.ShowColorBar`, `ShowMovingAverage`, `JudgmentLineThickness`, and `PointFadeOutTime` controls
- Added separate hit-animation controls for objects and sliders
  - New `Objects.HitAnimations` and `Objects.Sliders.HitAnimations` settings, both enabled by default
  - Disabling object hit animations makes successful hit circles disappear almost immediately
- Added osu!lazer-compatible spinner movement for cursor dance
  - Generated Lazer-compatible participants use the OD 11 spinner-completion rate by default
  - New `CursorDance.SpinnerBehavior.SpinAtLowestRPM` instead uses the completion rate for the map's OD, and is disabled by default
  - osu!stable replays retain their historical spinner rate
  - Spinner targets scale with the map and movement accounts for spinner duration
- Added combo color normalization ported from osu!lazer (`Objects.Colors.ComboColorNormalization`)
- Added F grades for failed plays
  - The grade is drawn as text when a skin has no F texture

### Changed

- Lazer-compatible osu!standard gameplay is now the default, replacing the old `LZ` mod workaround
  - osu!lazer replays and replay-free playback use Lazer-compatible behavior
  - Replays from osu!stable receive Classic compatibility automatically while retaining their Stable provenance
  - Mod compatibility, slider timing, and score handling follow the selected gameplay implementation
  - Classic changes supported compatibility behavior without changing whether playback is Stable- or Lazer-compatible
  - `Fade hit circles earlier` is now available with the other Classic settings
- Slider presentation and cursor dance now follow the selected gameplay implementation more closely
  - Lazer-compatible sliders use fractional end times and frame-computed, seek-safe snaking
  - Slider endpoints and hit animations follow the active skin
  - Slider bodies use the active slider timing for their fade
  - Cursor dance uses participant-specific slider paths, timing, endpoints, and score points
- Recording validates its complete output setup before allocating persistent resources
  - Preflight checks the selected encoder configuration, GPU limits, projected memory use, color contract, and timeline
  - Each recording uses an isolated session directory
- Recording color and motion blur now follow the encoded frame timeline
  - Color conversion and encoded metadata use BT.709 limited range
  - Motion-blur samples stay centered on rendered frames
- Discord Rich Presence is disabled by default
- New gameplay profiles use 4x MSAA by default, and the launcher visualizer also uses 4x MSAA
  - Change `Graphics.MSAA` to use a different gameplay sample count; changes take effect after restarting danser
- Positional misses are hidden by default in new profiles
  - Enable `Gameplay.HitErrorMeter.ShowPositionalMisses` when needed
- New profiles use a short slider-body fade after the slider ends
  - Enable `Objects.Sliders.Snaking.OutFadeInstant` for an instant fade
- `Gameplay.HitErrorMeter.UnstableRateScale`, shown as `Numeric UR size`, now scales and positions the UR number relative to the hit-error bar
- File and folder selection now uses platform-integrated dialogs
  - Windows uses native dialogs
  - Linux uses `zenity` or `kdialog` when available
- Renamed `Speed up startup on slow HDDs` to `Skip library rescan` (disabled by default)
- `Skip library rescan` now actually skips the library work
  - While skipping, star ratings are left untouched and new maps stay unrated until a full check runs
  - Selecting a deleted map removes it from song select without a full refresh

### Performance

- Reduced gameplay stutter at high refresh rates
  - Frame pacing follows deadlines instead of repeated polling
  - High-refresh and high-FPS Windows playback uses dedicated render pacing
  - Set `DANSER_FRAME_PROBE=1` to capture slow frame samples for diagnosis
- Reduced gameplay stutter during object-dense sections, which was caused by audio state and rate changes crossing to the audio thread once per call instead of once per update
- The launcher now populates and maintains your library in the background instead of on a blocking screen
- Song select search and scrolling no longer stutter on large libraries
- New `-beatmap-path` flag starts the selected map directly for a faster start

### Fixed

- Fixed hitsound and storyboard-audio timing during late gameplay frames
  - Sounds use their intended event times instead of the frame that processes them
  - Offline renders follow the audio delivered to the encoder instead of using the upstream fixed 50 ms offset
  - Video remains synchronized through speed changes and audio stalls
- Starting playback partway through a map or seeking no longer displays skipped judgments as new hits or misses
  - Skipped objects still affect score, accuracy, health, and statistics without causing knockout eliminations
- Loud stacked hitsounds have more headroom before clipping
  - Realtime audio now uses floating-point mixing
- Cursor dance no longer freezes or crashes on singular, extreme, or otherwise unusually expensive slider geometry
  - Bounded fallbacks do not shorten authored slider timing
- Spinner RPM feedback now responds over osu!lazer's trailing measurement window and stays correctly positioned across resolutions and custom skins
  - Seeking resets its measurement history
- Spinner judgment now follows osu!lazer's end-exclusive timing by default, avoiding an extra rotation at the exact end
- Spinner audio no longer repeatedly pauses and resumes while its spin state is unchanged
- Unsupported MSAA, framebuffer, and GPU configurations now fail with an actionable error instead of hanging during gameplay startup
- Recording refuses to replace an existing final output
  - Final output is published only after both encoders finish and muxing succeeds
  - Successful publication removes the isolated recording session
- Failed recordings retain their session directory and any produced audio or video intermediates
  - The reported error includes an FFmpeg command for recovering completed streams manually
- Startup and window-creation failures now produce visible errors
  - Gameplay startup failures use platform-integrated dialogs where available
  - CLI failures remain visible in command-line output
- Running the command-line binary without input now shows usage guidance instead of opening an unrelated web page

### Removed

- Removed the old `LZ` mod and the `Gameplay.Mods.ShowLazerMod` and `Gameplay.LazerClassicScore` settings in favor of the Classic mod

---

## danser 0.12.0 snapshot changes carried forward

### BIGGEST CHANGE: osu!lazer Compatibility, PP Updates, and SDL

### General

- Switched the windowing backend from GLFW to SDL, enabling headless rendering
  on Linux
- Updated the default bundled pp/SR model through `251020`, the rework deployed
  on October 29, 2025
- Added selectable `260321`, a pre-release March 2026 pp-dev snapshot
- Limited custom speed-mod rates to 0.1x through 10x
- Updated BASS
- Fixed HT/DT playback jitter
- Fixed maps with extreme timing-point beat lengths
- Fixed beatmap combo colors when maps define additional colors
- Spinner bonus sounds now follow the active timing point's sample volume
- Fixed rotated text rendering
- Fixed excessive CPU use with VSync in windowed mode on Windows

### Danser

#### Scoring (Lazer)

- Updated osu!lazer hit windows
- Fixed osu!lazer's health calculator freezing on maps with a single circle
- Fixed osu!lazer slider ticks after inherited timing points before the first
  authored timing point
- Fixed affected maps with unstable or very short circular slider arcs
- Fixed `SpunOut` in osu!lazer plays
- Fixed generated playback missing objects on slowed maps
- Fixed generated playback dropping circles that overlap a spinner start
- Replays now fail only after their input frames are exhausted
- Fixed circles reappearing with Traceable

#### Cursordance

- Added `CursorDance.Resolve2BAfterTAG`
- Added `Objects.Sliders.DrawReverseArrows`, independently controlling reverse
  arrows instead of tying them to slider-end circles
- Improved cursor dance spacing around rapid and double-click patterns
- Fixed 2B slider conflict detection overlooking earlier overlapping objects
- Fixed cursor dance spinners running too slowly with Half Time

#### Gameplay

- Added `Gameplay.AimErrorMeter.UseFallbackSkin`
- Added an option to always skip map intros
- Added `mapId` and `setId` to gameplay-stat templates
- Replay loading statistics now report the input frame count

#### Recording

- Removed unsupported Software AV1 preset 13

### Launcher

- Fixed the launcher crashing after a video render completed
- Fixed left clicks being interpreted as context-menu clicks
- Fixed map backgrounds missing from song selection

Happy dansing!
