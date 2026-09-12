<p align="center">
  <img src="assets/textures/coinbig.png" width="200" height="200" style="max-width:100%; height:auto;" alt="danser coin logo"/>
</p>

<p align="center">
  <a href="https://github.com/InnovationReadyTupperware/danser-ee/releases/latest"><img src="https://img.shields.io/github/v/release/InnovationReadyTupperware/danser-ee?label=release" alt="Latest release"/></a>
  <a href="https://github.com/InnovationReadyTupperware/danser-ee/releases"><img src="https://img.shields.io/github/downloads/InnovationReadyTupperware/danser-ee/total?label=downloads" alt="Total downloads"/></a>
  <a href="https://discord.gg/UTPvbe8"><img src="https://img.shields.io/discord/713705871758065685.svg?label=danser-go&logo=discord&logoColor=ffffff&color=7389D8&labelColor=6A7EC2" alt="danser-go Discord"/></a>
</p>

<p align="center">
  <strong>Danser Enterprise Edition (danser-ee)</strong> is a maintained fork of <a href="https://github.com/Wieku/danser-go">danser-go</a>, a GUI/CLI visualization tool for osu!standard maps. It can also render osu!stable and osu!lazer replays and record output to MP4.
</p>

## Contents

- [See it in action](#see-it-in-action)
- [What's different from danser-go?](#whats-different-from-danser-go)
- [Why this fork exists](#why-this-fork-exists)
- [Platform support](#platform-support)
- [Download and run](#download-and-run)
- [CLI reference](#cli-reference)
- [Building from source](#building-from-source)

## See it in action

*(These videos are from danser-go and show the core danser experience, not danser-ee-exclusive features.)*

* [Omoi - Chiisana Koi no Uta (Synth Rock Cover) [Kroytz's EX EX] - TAG2 Mirror Collage](https://youtu.be/Vo0Pbpu113Y)
* [Sex Whales & Fraxo - Dead To Me (feat. Lox Chatterbox) [extrad1881 (ar 10)] Mirror Collage](https://youtu.be/KCHqrVGdXrk)
* [Nightcore - Flower Dance [Amachoco ARX.7] Mandala Mirror Collage](https://youtu.be/HBC89S-UwFc)
* [Flower Dance (osu! cursordance)](https://youtu.be/lcnnz3fN3bs)
* [osu! top 50 replays knockout | xi - FREEDOM DiVE [ENDLESS DiMENSiONS]](https://youtu.be/kzr_Sr0Shuc)
* [osu! top 50 knockout | YURRY CANNON - Suicide Parade [Sakase]](https://youtu.be/GS_yoq5MJMU)
* [osu! top 50 replays knockout | Kobaryo - Bookmaker [Corrupt The World]](https://youtu.be/SJqkP1IDUq0)

## What's different from danser-go?

If you already know `danser-go`, `danser-ee` should feel immediately familiar. It keeps the core danser experience while continuing development as its own fork.

### A few of the bigger changes:

- **Lazer gameplay is the default.** Lazer and replay-free playback use Lazer-compatible behavior directly, while osu!stable replays receive Classic (CL) mod automatically just like in osu!lazer. The old `LZ` mod compatibility workaround is gone.
- **Launcher startup and large song libraries are much more responsive.** Catalog updates no longer block normal launcher startup, and song search and scrolling no longer stutter on large libraries.
- **Several settings have been ported from osu!lazer.** These include the `Only fade approach circles` toggle for Hidden, plus `Combo color normalization` and `Hit animations` settings.
- **Current osu!standard star rating and performance calculations (July 2026 SR/PP rework).**
- **Audio timing and recording sync are substantially more accurate.**
- **High-FPS gameplay is much smoother.**

<details>
<summary><strong>Detailed comparison with the danser-go fork base</strong></summary>

The table compares current `danser-ee` behavior with the documented fork point (`upstream/dev` at `3eb75a34`). It is a summary of meaningful product differences, not a complete changelog.

| Area                             | danser-ee                                                                                                                        | danser-go (fork base)                                                                                                  |
|----------------------------------|----------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------|
| CLI no-argument behavior         | ✅ Helpful usage, no rickroll                                                                                                    | ❌ Surprise rickroll                                                                                                   |
| Gameplay behavior                | Lazer gameplay behavior by default. Replays recorded in osu!stable automatically receive the Classic mod, just like in osu!lazer | `LZ` mod used to opt into Lazer-oriented behavior                                                                      |
| Object stacking                  | Overlapping objects pile up in the same positions as in osu!lazer                                                                | Stable-derived stacking positions                                                                                      |
| Slider judgments                 | Heads, ticks, repeats, and tails are judged separately in non-CL gameplay                                                        | Approximate tick/tail handling                                                                                         |
| Slider end timing                | Preserves sub-millisecond slider end times, matching osu!lazer                                                                   | Slider end timing is rounded to integer milliseconds                                                                   |
| Cursor-dance spinners            | Lazer movement and completion targets while Stable replays retain Stable behavior                                                | Fixed autoplay-oriented spinner behavior                                                                               |
| Slider miss markers              | Missed slider ticks and tails can show osu!lazer-style skin-aware markers                                                        | No slider-body miss markers                                                                                            |
| Latest osu!standard SR/PP rework | ✅ July 2026 SR/PP rework                                                                                                        | October 2025 SR/PP rework                                                                                              |
| osu!lazer settings               | ✅ `Only fade approach circles` for Hidden mod, combo color normalization, and hit animations toggle                             | ❌ No equivalent settings; hit animations stay enabled unless an instafade skin is used                                |
| Song catalog startup             | ✅ You can use the launcher while your library is processed in the background                                                    | ❌ You have to wait for danser to process your library before you can use the main menu                                |
| Song search and scrolling        | ✅ Fast and fluid even on massive libraries                                                                                      | ❌❌❌ Search and scrolling can stutter severely, especially on large libraries                                        |
| Hit-error timing                 | Matches osu!lazer timing behavior across all playback, including osu!stable replays                                              | Stable-derived integer hit-error windows                                                                               |
| Unstable Rate (UR)               | Each timing hit is normalized by gameplay rate before UR is calculated                                                           | Speed adjustment is applied only to the final UR value                                                                 |
| Hit-error customization          | Toggleable color band and moving-average arrow, adjustable timing-line thickness and fade                                        | Fixed bar layout with fewer display options and no results-screen median                                               |
| High-refresh frame pacing        | Deadline-based timing with dedicated Windows pacing for high-refresh and custom FPS caps                                         | ❌ Custom FPS caps can incur full scheduler-quantum stalls on Windows                                                  |
| Hitsound and storyboard timing   | Sounds use their intended event times in realtime playback and recording                                                         | ❌ Playback is triggered by the frame that processes the event                                                         |
| GPU/MSAA startup failures        | Unsupported configurations fail with an actionable error before gameplay starts                                                  | ❌ Unsupported multisample/framebuffer configurations can hang during startup                                          |
| Slider snaking                   | Stays in sync with the current playback position after seeking                                                                   | Pre-scheduled snaking can become out of sync after seeking                                                             |
| Recording synchronization        | Audio/video stay synchronized through speed changes and audio stalls                                                             | Audio is timed separately from encoded video ("source-clocked"), which can cause desync during speed changes or stalls |
| Recording failures and output    | Encoder failures are checked; intermediates are retained for recovery; existing final output is not replaced                     | ❌ Encoder failure can be missed and recovery intermediates can be destroyed                                           |
| Recording color                  | ✅ BT.709 limited-range conversion with matching encoded metadata                                                                | ❌ BT.601 conversion tagged as BT.709                                                                                  |
| Recording motion blur            | Samples are centered on the encoded frame timeline without black startup history                                                 | ❌ Trailing shutter with black startup history                                                                         |
| Solo knockout                    | ✅ Replay-free knockout with generated, fully scored participants                                                                | -                                                                                                                      |

<small><em>Scope note: This comparison is against the documented fork base, not current upstream. It reflects the behavior observed there and the changes currently implemented in danser-ee. The approaches shown here are not necessarily the only or best possible solutions, and both projects may continue to evolve. The table is intended to document meaningful differences in good faith, not to claim final authority over how they should be solved.</em></small>

</details>

## Why this fork exists

`danser-ee` is a maintained fork of [danser-go](https://github.com/Wieku/danser-go). I started it after upstream development had slowed for over a year. I wanted to keep pushing danser forward at my own pace and bring it closer to the standard I always believed it could reach. I share the fork publicly because it may be useful to other danser users too.

The `ee` stands for **Enterprise Edition** - intentionally grandiose naming for what is, in practice, a pragmatic fork of `danser-go`.

## Platform support

| Operating system | Status         |
|------------------|----------------|
| Windows x64      | ✅             |
| Linux x64        | 🟡 Best effort |
| macOS            | ❌             |

**Linux:** Support is best effort and is not as thoroughly tested as Windows.

**macOS:** Not supported because danser relies heavily on OpenGL, which Apple has deprecated and no longer meaningfully advances on macOS.

## Download and run

1. **Download the package for your platform.**

   Open the [latest danser-ee release](https://github.com/InnovationReadyTupperware/danser-ee/releases/latest) and download one of the ready-to-run archives:

   - Windows: `danser-<version>-win.zip`
   - Linux: `danser-<version>-linux.zip`

   The `Source code` archives generated by GitHub are for building danser-ee yourself; they are not ready-to-run packages.

2. **Extract the whole ZIP to its own directory.**

   Keep the extracted contents together. The release package contains danser alongside the runtime files, assets, and bundled tools it needs, so copying out only the executable is not supported.

3. **Start danser.**

   - **Windows:** open `danser.exe`.
   - **Linux:** run `./danser` from the extracted directory.

4. **Prefer the command line?**

   Use `danser-cli` from the same extracted directory:

   **Windows**

   ```powershell
   .\danser-cli.exe <arguments>
   ```

   **Linux**

   ```bash
   ./danser-cli <arguments>
   ```

   For map selection, replay playback, recording, screenshots, mods, and the rest of the available options, head to the [CLI reference](#cli-reference).

## CLI reference

<details>
<summary><strong>Run arguments and examples</strong></summary>

* `-artist="NOMA"` or `-a="NOMA"`
* `-title="Brain Power"` or `-t="Brain Power"`
* `-difficulty="Overdrive"` or `-d="Overdrive"`
* `-creator="Skystar"` or `-c="Skystar"`
* `-md5=hash` - overrides all map selection arguments and attempts to find `.osu` file matching the specified MD5 hash
* `-beatmap-path="12345 Artist - Title/artist - title [hard].osu"` - starts the given map directly for a faster start
* `-id=433005` - overrides all map selection arguments and attempts to find `.osu` file with matching BeatmapID (not BeatmapSetID!)
* `-cursors=2` - number of cursors used in mirror collage
* `-tag=2` - number of generated cursors in TAG mode, or generated Danser participants in solo knockout
* `-speed=1.5` - music speed. Value of 1.5 is equal to osu!'s DoubleTime mod. Ignored if in `-play` mode with speed changing mods
* `-pitch=1.5` - music pitch. Value of 1.5 is equal to osu!'s Nightcore pitch. To recreate osu!'s Nightcore mod, use
  with speed 1.5
* `-settings=name` - settings filename - for example `settings/name.json` instead of `settings/default.json`
* `-debug` - shows additional info when running Danser, overrides `Graphics.DrawFPS` setting
* `-play` - play through the map in osu!standard mode
* `-skip` - skips map's intro like in osu!
* `-start=20.5` - start the map at a given time (in seconds)
* `-end=30.5` - end the map at a given time (in seconds)
* `-knockout` - knockout mode
* `-knockout2="[\"replay1.osr\",\"replay2.osr\"]"` - knockout mode, but instead of using danser's replays folder,
  sources replays from the given JSON array. `Knockout.MaxPlayers` and `Knockout.ExcludeMods` settings are ignored.
* `-solo-knockout` - map-driven knockout with generated Danser participants. Use `-tag` to choose the participant count
  and `-cursors` to choose the number of mirrored cursor views.
* `-record` - Records danser's output to a video file. Needs an
  accessible [FFmpeg](https://github.com/Wieku/danser-go/wiki/FFmpeg) installation.
* `-out=abcd` - overrides `-record` flag, records to a given filename instead of auto-generating it. Extension of the
  file is set in settings. When the `-ss` flag is used, this sets the output filename as well.
* `-replay="path_to_replay.osr"` or `-r="path_to_replay.osr"` - plays a given replay file. Be sure to replace `\`
  with `\\` or `/`. Overrides all map selection arguments
* `-mods=HDHR` - displays the map with given mods. `-mods=AT` will
  trigger cursordance with replay UI. If specified, it will override `-replay` mods
* `-mods2="[{\"acronym\":\"DT\",\"settings\":{\"speed_change\":1.2}},{\"acronym\":\"HD\"}]"` - displays the map with given mods. It's using lazer's mod structure to support mod settings. If specified, it will override `-replay` mods. As above, adding AT will
  trigger cursordance with replay UI
* `-skin` - overrides `Skin.CurrentSkin` in settings
* `-cs`, `-ar`, `-od`, `-hp` - overrides maps' difficulty settings (values outside of osu!'s normal limits accepted). Ignored if DA (Difficulty Adjust) mod is specified in `-mods2`
* `-nodbcheck` - skips updating the database with new, changed or deleted maps
* `-noupdatecheck` - skips checking GitHub for a newer version of danser-ee
* `-include-prerelease-updates` - includes alpha, beta, and release candidate versions in update checks
* `-ss=20.5` - creates a screenshot at the given time in .png format
* `-quickstart` - skips intro (`-skip` flag), sets `LeadInTime` and `LeadInHold` to 0.
* `-offset=20` - local audio offset in ms, applies to recordings unlike `Audio.Offset`. ~~Inverted compared to stable~~ not anymore.
* `-preciseprogress` - prints record progress in 1% increments.
* `-sPatch="{\"Cursor\":{\"CursorSize\":50}}"` - patches the currently loaded config with supplied JSON string. Patch is preserved during config file reloads. Useful for 3rd party devs to avoid having to parse and modify the settings files on small tweaks.

Examples which should give the same result:

```bash
<executable> -d="Overdrive" -tag=2 //Assuming that there is only ONE map with "Overdrive" as its difficulty name

<executable> -t="Brain Power" -d="Overdrive" -tag=2

<executable> -t "Brain Power" -d Overdrive -tag 2

<executable> -t="ain pow" -difficulty="rdrive" -tag=2

<executable> -md5=59f3708114c73b2334ad18f31ef49046 -tag=2

<executable> -id=933228 -tag=2
```

Settings and knockout usage are detailed in the upstream [danser-go wiki](https://github.com/Wieku/danser-go/wiki).

</details>

## Building from source

`danser-ee` uses CGO and depends on native runtime files and project assets. A successful Go build does not make an arbitrary output directory self-contained, so development builds should be run with the repository root as their working directory.

### Requirements

* [Go 1.27.1](https://go.dev/dl/)
* A compatible C/C++ toolchain for CGO
  * Windows: [WinLibs](https://winlibs.com/) MSVCRT+POSIX or another compatible MinGW-family toolchain. TDM-GCC is not supported.
  * Linux: gcc/g++.
* OpenGL support from a modern graphics driver. Recording requires buffer storage, direct-state access, image copy, and texture readback capabilities; preflight reports missing capabilities before encoder startup. Linux build environments may also need `libgl1-mesa-dev`.
* Linux builds may additionally need `xorg-dev`, `libgtk-3`, and `libgtk-3-dev`.

### Build and run

Clone the repository and work from its root directory:

```bash
git clone https://github.com/InnovationReadyTupperware/danser-ee.git
cd danser-ee
```

For local development, keep the repository root as the executable's working directory. This matters because danser resolves native libraries and other runtime assets from the development tree. For quick iteration, `go run . <arguments>` from the repository root is also appropriate.

On Windows:

```powershell
go build .
.\danser-ee.exe <arguments>
```

On Linux:

```bash
go build -o ./danser .
./danser <arguments>
```

If you run the project from an IDE, configure the run target to use the repository root as its working directory. Prefer this over launching a development binary from an IDE-specific output directory with no access to the repository's native libraries and assets.

To verify that all Go packages compile:

```bash
go build ./...
```

> [!NOTE]
> Use `dist-win.sh` or `dist-linux.sh` for release packages. A normal development
> build does not assemble the launcher and bundled runtime files.

<details>
<summary><strong>Frame pacing diagnostics</strong></summary>

Set `DANSER_FRAME_PROBE=1` before starting an interactive gameplay process to
retain up to 128 of the slowest samples for each instrumented render layer.
When gameplay closes, danser writes `FrameProbe:` entries to `danser.log`,
ordered from slowest to fastest and grouped by main loop, graphics pipeline,
and player rendering stages.

On PowerShell:

```powershell
$env:DANSER_FRAME_PROBE = '1'
.\danser-ee.exe <arguments>
```

</details>
