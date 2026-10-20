# Apple Music ALAC / Dolby Atmos Downloader

[English](./README.md) | [简体中文](./README-CN.md) | [Cloud Server / Proxy Setup](./PROXY-SETUP.md)

> **Original script by Sorrow.** Modified with fixes and improvements.

This command-line tool downloads albums, songs, playlists, stations and music videos from Apple Music, preserves or embeds metadata and lyrics, and supports ALAC, AAC and Dolby Atmos. Use it only with content you are entitled to access and in accordance with Apple's terms and applicable law.

## Contents

- [Features](#features)
- [Supported formats](#supported-formats)
- [Requirements](#requirements)
- [Quick Start](#quick-start)
- [Graphical interface](#graphical-interface)
- [Configuration](#configuration)
- [Usage](#usage)
- [Upgrade](#upgrade)
- [For Developers](#for-developers)
- [Get media-user-token](#get-media-user-token)
- [Lyrics options](#lyrics-options)
- [Credits](#credits)
- [Cloud Server / Proxy Setup](./PROXY-SETUP.md)

## Features

1. Inline cover art and LRC lyrics.
2. Word-by-word and unsynchronized lyrics.
3. Artist album downloads.
4. Streaming download and decryption for large files.
5. Music-video downloads using in-process mp4ff decryption.
6. Interactive search and track selection.

## Supported formats

| Format | Description | Requires subscription |
|---|---|---|
| `alac` | `audio-alac-stereo` | Yes |
| `ec3` | `audio-atmos` / `audio-ec3` | Yes |
| `aac` | `audio-stereo` | Yes |
| `aac-lc` | `audio-stereo` | Yes |
| `aac-binaural` | `audio-stereo-binaural` | Yes |
| `aac-downmix` | `audio-stereo-downmix` | Yes |
| `MV` | Music video | Yes |

Stations require a valid `media-user-token` from an active subscription.

## Requirements

Install and prepare these before running the downloader:

1. **Apple Music account**: ALAC / AAC / Dolby Atmos all require an active subscription, and the account must be able to sign in to Apple Music normally.
2. **wrapper-lite**: the required backend decryption service. Any download fails while it is not running. See the [wrapper-lite repository](https://github.com/WorldObservationLog/wrapper/tree/lite).
   - **Get and start it**: the bundled `wrapper-lite-qemu` launcher is recommended (self-contained QEMU, no root and no native chroot needed); Docker works too, as does building from source with CMake and a C++ toolchain.
   - **First login**: the first run must log in once and cache the token locally (a 2FA code may be required) before the service can be started.
   - It listens on `http://127.0.0.1:12340` by default; put that address in `general.lite-server` in `config.yaml`.
   - If it runs on another device or in a container, make it listen on `0.0.0.0` and point `lite-server` at that host's reachable address (do not use `127.0.0.1`).
   - The regions returned by the service's `/status` must cover the storefront you want to download from.
3. **ffmpeg** (Optional): Required only for post-download conversion, animated artwork, or `ffmpeg`-dependent features. See [ffmpeg.org](https://ffmpeg.org/).
4. **Network**: the host must be able to reach `music.apple.com` and `amp-api.music.apple.com`. Restricted networks such as cloud servers are often blocked by Apple; see [Cloud Server / Proxy Setup](./PROXY-SETUP.md).
5. **Disk space**: lossless audio and music videos are large and downloads stream to disk, so leave enough room. The memory buffer cap is adjustable via `general.max-memory-limit`.

> **Note**: If you are using the precompiled release binaries, **Go is NOT required**. Go (1.23.1+) is only needed if you build from source (see [For Developers](#for-developers)).
> Android / Termux additionally needs `termux-setup-storage` for storage access; use `termux-wake-lock` before long or background downloads.

## Quick Start

Download the precompiled binary for your operating system and architecture from the [latest GitHub Releases](https://github.com/zhaarey/apple-music-downloader/releases/latest):

| Platform | Architecture | Precompiled Binary |
|---|---|---|
| **Windows** | x86_64 (64-bit) | `amdl_windows_amd64.exe` |
| **Windows** | ARM64 | `amdl_windows_arm64.exe` |
| **macOS** | Apple Silicon (M1/M2/M3/M4) | `amdl_darwin_arm64` |
| **macOS** | Intel (x86_64) | `amdl_darwin_amd64` |
| **Linux** | x86_64 (amd64) | `amdl_linux_amd64` |
| **Linux** | ARM64 (aarch64) | `amdl_linux_arm64` |
| **Android (Termux)** | ARM64 (aarch64) | `amdl_android_arm64` |

---

### Windows (PowerShell)

1. Open PowerShell and run the following command to download the executable:

```powershell
# Download precompiled binary (example for 64-bit Windows)
Invoke-WebRequest -Uri "https://github.com/zhaarey/apple-music-downloader/releases/latest/download/amdl_windows_amd64.exe" -OutFile "amdl.exe"
```

2. Run `amdl.exe` once to automatically generate the default `config.yaml`:

```powershell
.\amdl.exe
```

3. Open and edit `config.yaml` to set your `lite-server` address and destination paths.
4. Test running the downloader:

```powershell
.\amdl.exe --help
```

Example:

```powershell
.\amdl.exe "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

---

### macOS

1. Download the precompiled binary:

```bash
# For Apple Silicon (M-series):
curl -L -o amdl https://github.com/zhaarey/apple-music-downloader/releases/latest/download/amdl_darwin_arm64

# For Intel Macs:
# curl -L -o amdl https://github.com/zhaarey/apple-music-downloader/releases/latest/download/amdl_darwin_amd64

# Grant execution permission
chmod +x amdl
```

> **Tip (macOS Gatekeeper)**: If macOS blocks running `amdl` because it is from an unidentified developer, remove the quarantine attribute:
> ```bash
> xattr -d com.apple.quarantine amdl
> ```

2. Run `amdl` once to automatically generate the default `config.yaml`:

```bash
./amdl
```

3. Edit `config.yaml` with your settings.
4. Run the downloader:

```bash
./amdl "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

---

### Linux

1. Download the precompiled binary:

```bash
# For x86_64 / amd64:
curl -L -o amdl https://github.com/zhaarey/apple-music-downloader/releases/latest/download/amdl_linux_amd64

# For ARM64:
# curl -L -o amdl https://github.com/zhaarey/apple-music-downloader/releases/latest/download/amdl_linux_arm64

# Grant execution permission
chmod +x amdl
```

2. Run `amdl` once to automatically generate the default `config.yaml`:

```bash
./amdl
```

3. Edit `config.yaml` with your settings.
4. Run the downloader:

```bash
./amdl "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

---

### Android (Termux)

Install Termux from [F-Droid](https://f-droid.org/en/packages/com.termux/) or the [official GitHub releases](https://github.com/termux/termux-app/releases) (do not use the obsolete Play Store version).

1. Update packages and install curl (and optionally ffmpeg):

```bash
pkg update && pkg upgrade
pkg install -y curl ffmpeg
```

2. Grant access to shared Android storage (creates `~/storage/shared`):

```bash
termux-setup-storage
```

3. Download precompiled binary:

```bash
curl -L -o amdl https://github.com/zhaarey/apple-music-downloader/releases/latest/download/amdl_android_arm64
chmod +x amdl
```

4. Run `amdl` once to automatically generate the default `config.yaml`:

```bash
./amdl
```

5. Edit `config.yaml`. To save downloads into Android's shared Music directory, point the paths in `config.yaml` to the shared mount:

```yaml
paths:
  alac: "/sdcard/Music/amdl"
  atmos: "/sdcard/Music/amdl-atmos"
  aac: "/sdcard/Music/amdl-aac"
  mv: "/sdcard/Music/amdl-mv"
```

6. Run:

```bash
./amdl "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

> **Tips for Termux**:
> - For long downloads, prevent Android from sleeping with `termux-wake-lock` (release with `termux-wake-unlock` when done).
> - If wrapper-lite runs on another device on your network, point `lite-server` to that device's LAN IP, not `127.0.0.1`.
> - These instructions target current Android arm64 Termux environments; 32-bit Android is not a documented target.
> - If self-update on an older version fails with `lookup api.github.com on [::1]:53 ... connection refused`, re-download the binary once as in step 3; later versions update normally.

## Graphical interface

If you would rather not use the command line, launch the GUI:

```powershell
.\amdl.exe --gui
```

It opens a dedicated window (rendered by the Edge WebView2 runtime) and walks through:

1. **Paste a link** — albums, songs, playlists, stations, music videos and artist pages are supported.
   An album link carrying `?i=` downloads just that song.
2. **Metadata is fetched automatically** — cover art, title, artist, year, track count, record label,
   the Apple Digital Master flag, and the full track list with durations and explicit markers.
3. **Pick a quality** — only qualities the title actually offers are selectable:
   - Lossless (ALAC), labelled CD-quality or Hi-Res Lossless
   - High Quality (AAC), 256 kbps
   - Dolby Atmos, greyed out with a reason when unavailable
   - The destination folder for the selection is shown underneath
4. **Confirm** — a progress bar, processed counts and a live log are displayed.
5. **On completion** — "Download complete" appears with the success count and file list;
   **Open file location** reveals the files in the OS file manager.

> **Note**: the UI binds to loopback only (a random `127.0.0.1` port) and authenticates with a
> per-run random token, so other programs or pages cannot drive it. The terminal prints the URL,
> which also works in a normal browser.
> Downloads still require wrapper-lite to be running and logged in (see [Requirements](#requirements)).

## Configuration

When `amdl` is run for the first time without an existing configuration file, it automatically creates `config.yaml` in the current directory from an embedded default template.

At minimum, review and set:

```yaml
general:
  # wrapper-lite HTTP API endpoint.
  lite-server: "http://127.0.0.1:12340"

  # Required for stations. See "Get media-user-token" below.
  media-user-token: "your-media-user-token"

# Destination folders. Relative paths are resolved from the working directory.
paths:
  alac: "AM-Lossless"
  atmos: "AM-Atmos"
  aac: "AM-AAC"
  mv: "AM-MV"
```

If wrapper-lite runs on another machine or container, replace `127.0.0.1` with that host's reachable LAN or public address.

## Usage

Before running any command, make sure:

1. wrapper-lite is running.
2. `config.yaml` exists and has the correct `lite-server` value.

### Album

```bash
./amdl "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

On Windows:

```powershell
.\amdl.exe "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

### Single song

```bash
./amdl "https://music.apple.com/us/album/never-gonna-give-you-up-2022-remaster/1624945511?i=1624945512"
./amdl "https://music.apple.com/us/song/you-move-me-2022-remaster/1624945520"
```

### Artist albums

```bash
./amdl --all-album "https://music.apple.com/us/artist/taylor-swift/159260351"
```

### Playlist

```bash
./amdl "https://music.apple.com/us/playlist/taylor-swift-essentials/pl.3950454ced8c45a3b0cc693c2a7db97b"
```

### Interactive selection

```bash
./amdl --select "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

Enter track numbers separated by spaces.

### Interactive search

```bash
./amdl --search album "never gonna give you up"
./amdl --search song "you move me"
./amdl --search artist "taylor swift"
```

### Dolby Atmos

```bash
./amdl --atmos "https://music.apple.com/us/album/1989-taylors-version-deluxe/1713845538"
```

### AAC

```bash
./amdl --aac "https://music.apple.com/us/album/1989-taylors-version-deluxe/1713845538"
```

### Show quality information

```bash
./amdl --debug "https://music.apple.com/us/album/1989-taylors-version-deluxe/1713845538"
```

### Lossless edge cases

Without `--aac` / `--atmos` the downloader only considers ALAC streams and picks the **highest sample rate and bit depth that stays within `media.alac-max`** (given both 24-bit/192 kHz and 24-bit/96 kHz renditions, it takes 192 kHz).

Two optional switches change what happens when a track has no ALAC stream:

| Option | Default | Meaning |
|---|---|---|
| `media.lossless-only` | `false` | When `true`, a track with no lossless stream is **skipped and counted as Unavailable** instead of silently falling back to AAC. Explicit `--aac` / `--atmos` requests are unaffected. |
| `media.alac-verify` | `false` | When `true`, the downloaded ALAC stream is scanned on completion: malformed packet counts are reported and a file with no ALAC track is treated as an error. Only runs while `media.alac-fix` is `false` — both walk every packet, so they are never executed twice. |

```yaml
media:
  alac-max: 192000        # Sample rate ceiling; values above 192000 are clamped to 192000
  alac-fix: true          # Patch ALAC packets missing the TYPE_END terminator (also reports damage)
  alac-verify: false      # When alac-fix is off, scan without modifying
  lossless-only: false    # Skip tracks without a lossless stream instead of falling back to AAC
```

### Common options

```text
--alac-max <sample-rate>
--atmos-max <bitrate>
--aac-type <aac|aac-lc|aac-binaural|aac-downmix>
--mv-max <resolution>
--mv-audio-type <atmos|ac3|aac>
--lite-server <wrapper-lite-url>
```

## Get media-user-token

`media-user-token` is required for stations.

1. Open [Apple Music](https://music.apple.com) and sign in.
2. Open browser developer tools with `F12`.
3. Go to `Application` > `Storage` > `Cookies` > `https://music.apple.com`.
4. Find the cookie named `media-user-token` and copy its value.
5. Paste it into `general.media-user-token` in `config.yaml`.
6. Restart the downloader.

## Lyrics options

Configure lyrics settings under `metadata.lyrics` in `config.yaml`:

```yaml
metadata:
  lyrics:
    save-file: false                      # Save .lrc or .ttml file to disk
    embed: true                           # Embed lyrics directly into audio container tags
    type: "lyrics"                        # Lyrics type: "lyrics" (standard line-by-line) or "syllable-lyrics" (word-by-word)
    format: "lrc"                         # Lyrics file format: "lrc" or "ttml"
    extra: ""                             # Additional options: "" (default), "translation", or "pronunciation"
```

> **Tips**:
> - Set `general.language` in `config.yaml` (e.g. `"en-US"`, `"zh-Hans-CN"`, `"ja"`) to control the target language for translated lyrics.
> - `extra`: set to `"translation"` for translated lyrics or `"pronunciation"` for phonetic/romanized lyrics (when provided by Apple Music).

## Upgrade

### Built-in Self Update (Recommended)

Upgrade to the latest official release with automatic checksum verification and interactive configuration migration:

```bash
# Check and perform self-update with interactive config migration
./amdl --update

# Shorthand flag
./amdl -U

# Check for updates without downloading
./amdl --check-update

# Automated / non-interactive mode (automatically accept new option defaults)
./amdl -U -y
```

On Windows PowerShell:

```powershell
# Perform self-update
.\amdl.exe --update

# Shorthand flag
.\amdl.exe -U
```

> **Note**: Self-update automatically creates a timestamped `config.yaml.bak_...` backup and safely guides the migration of new settings without overwriting your credentials, custom paths, or comments. If a proxy is configured in `config.yaml`, the updater automatically routes through it.

### Manual Binary Update

Download the latest precompiled executable from [GitHub Releases](https://github.com/zhaarey/apple-music-downloader/releases/latest) and replace your current `amdl` / `amdl.exe` binary.

> For developers building from source, see [For Developers](#for-developers).

## For Developers

If you want to contribute, modify the code, or build the downloader from source:

### Prerequisites

1. **Go 1.23.1 or newer**: [go.dev/dl](https://go.dev/dl/).
2. **Git**: [git-scm.com](https://git-scm.com/).
3. **ffmpeg**: [ffmpeg.org](https://ffmpeg.org/) (optional for audio conversion or animated artwork).

### Clone and Build

**macOS / Linux**:

```bash
git clone https://github.com/zhaarey/apple-music-downloader.git
cd apple-music-downloader
cp config.yaml.example config.yaml
go build -o amdl .
./amdl --help
```

**Windows (PowerShell)**:

```powershell
git clone https://github.com/zhaarey/apple-music-downloader.git
cd apple-music-downloader
copy config.yaml.example config.yaml
go build -o amdl.exe .
.\amdl.exe --help
```

### Run in Development

```bash
go run . "https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511"
```

### Update Source Code

```bash
git pull
go build -o amdl .
```

On Windows (PowerShell):

```powershell
git pull
go build -o amdl.exe .
```

### Releasing

Releases are driven by the `VERSION` file. When a push to `main` carries a `VERSION` whose `v<VERSION>` tag does not exist yet, the Release workflow builds every platform and publishes a GitHub Release with that tag. To cut a release, bump `VERSION` (e.g. `0.1.0` → `0.1.1`) and merge it into `main`. A version with a suffix such as `0.2.0-beta.1` is published as a pre-release.

Release binaries self-update from the repository that built them, so a fork's releases update from the fork.

## Credits

- **Sorrow** created the original script.
- **WorldObservationLog** created [wrapper / wrapper-lite](https://github.com/WorldObservationLog/wrapper), used as the backend decryption service.
- **Sendy McSenderson** contributed the streaming download-and-decrypt implementation.
- [go-mp4tag](https://github.com/itouakirai/go-mp4tag) writes station MP4 metadata.
- [FFmpeg](https://ffmpeg.org/) supports optional conversion and animated-artwork features.
