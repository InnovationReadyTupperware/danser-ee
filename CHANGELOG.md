# Changelog

## 1.1.0 - 2026-09-12

# What's new in Danser Enterprise Edition 1.1.0

### Highlights

- Added support for the July 2026 osu!standard performance points and star rating rework
- Hitsounds now follow osu!lazer's rules for sample resolution, volume, custom filenames, and stereo positioning
- Overlapping objects now stack in the same positions as in osu!lazer
- Recording and screenshot output settings have been redesigned, with screenshot time validated against the selected map or replay
- Launcher and CLI startup no longer wait for update checks

### Breaking Changes

- Hitsound stereo positioning now matches osu!lazer, including its 20% default separation, replacing `Audio.HitsoundPositionMultiplier` with `Audio.HitsoundStereoSeparation`

### Added

- Added support for the July 2026 osu!standard performance points and star rating rework
  - Song-select star ratings will be recalculated for the July 2026 rework, while older SR/PP versions remain available for gameplay overlays and calculations
  - Old `260321` and `26xxxx` development selections migrate to `260706`
- Introduced a new popup dialog design for the launcher, starting with About and output configuration
- Added osu!lazer's `Only fade approach circles` customization for the Hidden mod
- Added `Include pre-release updates` to Launcher settings and `-include-prerelease-updates` to CLI update checks
  - Stable builds continue checking only stable releases unless pre-release updates are enabled
  - Pre-release builds can still advance within their current release line when the setting is off

### Changed

- Redesigned the launcher's About page with the new popup dialog design
- Redesigned recording and screenshot output settings with the new popup dialog design
  - Screenshot time can be entered as values like `.25` without a leading zero
  - Invalid screenshot times are caught before taking a screenshot
- Launcher and CLI update checks now run without interrupting or delaying startup

### Fixed

- Object stacking now follows osu!lazer's behavior
- Hit error bar feedback now uses osu!lazer's fractional 300/100/50 timing windows, including for replays recorded in osu!stable
  - At OD5, this means +/-49.5 ms, +/-99.5 ms, and +/-149.5 ms boundaries instead of osu!stable's +/-50 ms, +/-100 ms, and +/-150 ms
- Unstable Rate now normalizes each timing hit by its gameplay rate before calculation
- The results screen shows Unstable Rate as `N/A` when no timed hits contribute
- Slider heads, ticks, repeats, loops, and tails now resolve and play hitsounds using osu!lazer's sample rules
  - Slider ticks and loops keep the slider's start-resolved sample bank, index, and volume instead of changing at later timing points
  - Slider heads, repeats, and tails use their own node sample settings
- Circles, spinners, and slider nodes now resolve hitsound control points with osu!lazer's 5 ms legacy leniency
- Hitsound layers now keep their authored volume and use osu!lazer's 5% minimum volume
- Legacy sample set `0` now falls back to the Normal bank instead of Soft
- Custom hitsound filenames now work on circles, spinners, and slider nodes while preserving requested whistle, finish, and clap layers

## 1.1.0-alpha.3 - 2026-09-12

# What's new in Danser Enterprise Edition 1.1.0 Alpha 3

### Fixed

- The About dialog keeps its title and controls visible while its content scrolls

## 1.1.0-alpha.2 - 2026-09-10

# What's new in Danser Enterprise Edition 1.1.0 Alpha 2

### Added

- Added `Include pre-release updates` to Launcher settings and `-include-prerelease-updates` to CLI update checks. With pre-release updates off, stable builds only check stable releases, while pre-release builds can finish their current release line without jumping to a newer pre-release line

### Changed

- Redesigned About
- Update checks no longer interrupt or delay startup
- Retired `Audio.HitsoundPositionMultiplier` in favor of `Audio.HitsoundStereoSeparation`, which now defaults to osu!lazer's 20% and pans hitsounds left and right the same way osu!lazer does

### Fixed

- Overlapping objects now stack in the same positions as in osu!lazer
- Unstable Rate now normalizes each timing hit by its gameplay rate before calculation
- Results now show `N/A` when no timed hits contribute to UR
- Slider heads, ticks, repeats, loops, and tails now resolve and play hitsounds using osu!lazer's sample rules
  - Slider ticks and loops keep the slider's start-resolved sample bank, index, and volume instead of changing at later timing points
  - Slider heads, repeats, and tails use their own node sample settings and osu!lazer's legacy control-point leniency
- Hitsound layers now keep their authored volume and use osu!lazer's 5% minimum volume
- Legacy sample set `0` now falls back to the Normal bank instead of Soft
- Custom hitsound filenames now work on circles, spinners, and slider nodes while preserving requested whistle, finish, and clap layers

## 1.1.0-alpha.1 - 2026-09-08

# What's new in Danser Enterprise Edition 1.1.0 Alpha 1

### Added

- Added osu!lazer's `Only fade approach circles` customization for the Hidden mod
- Added a new experimental popup dialog in the launcher
  - The `Configure` button now opens redesigned output settings using the new dialog

### Changed

- Updated osu!standard star rating and performance points to the July 2026 SR/PP rework
  - Existing catalog ratings are recalculated once because the stored star-rating version advances to 20260706
  - Song-select catalog stars always use the current released model, while the gameplay SR/PP selector can still use supported historical models
  - Old `260321` and `26xxxx` development selections migrate to `260706`
- Stable and Lazer gameplay now use their respective object-stacking behavior

## 1.0.1 - 2026-09-08

# What's new in Danser Enterprise Edition 1.0.1

### Fixed

- Combo color normalization no longer renders the wrong color
  - Saturated blues showed green and saturated reds showed black at any amount above 0%
- Combo colors no longer shift by one on maps whose first object isn't a new combo

## 1.0.0 - 2026-09-05

# What's new in Danser Enterprise Edition 1.0.0

### Highlights

- Gameplay now follows osu!lazer's model
  - Lazer-compatible behavior is the default, while osu!stable replays receive `CL` / Classic compatibility automatically
  - The old `LZ` workaround is no longer needed
- Hitsounds and storyboard audio now occur at their intended times, and video recordings stay synchronized through speed changes and audio stalls
- Song catalog updates no longer hold up launcher startup
- High-refresh gameplay is smoother, especially with custom FPS caps on Windows
- Solo knockout can now run with generated participants instead of replay files
- Lazer-compatible sliders now judge heads, ticks, repeats, and tails separately and can show skin-aware markers for supported misses

### Breaking Changes

- OpenGL 4.5 core is now required

### Added

- Added replay-free solo knockout with fully scored participants
  - New `-solo-knockout` flag to enable it, with `-tag` setting the participant count and `-cursors` the mirrored cursor views
- Added Lazer-compatible osu!standard judgments for slider parts
  - New `Objects.Sliders.ShowSliderJudgmentMarkers` displays supported misses with the `slidertickmiss` and `sliderendmiss` skin components
- Added more hit error bar customization and brought its behavior closer to osu!lazer
  - New `Gameplay.HitErrorMeter.ShowColorBar` (on by default), `ShowMovingAverage` (on by default), and `JudgmentLineThickness` (3 o!px by default) controls. `PointFadeOutTime` is now shown as "Timing-line fade-out time" and still defaults to 10 seconds
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
- `Gameplay.HitErrorMeter.ShowPositionalMisses` now defaults to false in new profiles
- New profiles use a short slider-body fade after the slider ends
  - Enable `Objects.Sliders.Snaking.OutFadeInstant` for an instant fade
- `Gameplay.HitErrorMeter.UnstableRateScale`, shown as `Numeric UR size`, now scales the numeric UR readout relative to the hit-error bar
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

# danser 0.12.0 snapshot changes carried forward

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
