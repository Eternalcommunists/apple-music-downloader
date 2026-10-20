<#
.SYNOPSIS
  Cross-platform release build for amdl (Windows PowerShell entry point).

.DESCRIPTION
  Produces the same artifacts as .github/workflows/release.yml, so a release can
  be cut from a local machine without GitHub Actions, or a version injection can
  be verified before pushing a tag.

  The version, commit and build date are injected through -ldflags exactly like
  the CI workflow, so `amdl --version` reports the same thing either way.

.PARAMETER All
  Build every supported target instead of just the native one.

.PARAMETER Targets
  Comma-separated os/arch list, e.g. "windows/amd64,linux/amd64".

.PARAMETER Version
  Override the version. Defaults to the contents of the VERSION file.

.PARAMETER Out
  Output directory. Defaults to "dist".

.PARAMETER Gui
  Build a windowed (GUI subsystem) binary, so double-clicking it never flashes a
  console window. Only meaningful for Windows targets.

.EXAMPLE
  .\scripts\build.ps1
  Build for the current OS/arch.

.EXAMPLE
  .\scripts\build.ps1 -All
  Build every supported target and write dist\checksums.txt.

.EXAMPLE
  .\scripts\build.ps1 -Targets windows/amd64 -Gui
  Build a windowed Windows binary.
#>

[CmdletBinding()]
param(
    [switch]$All,
    [string]$Targets = "",
    [string]$Version = "",
    [string]$Out = "dist",
    [string]$RepoOwner = "",
    [string]$RepoName = "",
    [switch]$Gui
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# Default target list mirrors release.yml. Android is separate because it needs
# cgo plus a matching NDK toolchain, which a local machine rarely has.
$DefaultTargets = @(
    "windows/amd64",
    "windows/arm64",
    "linux/amd64",
    "linux/arm64",
    "darwin/amd64",
    "darwin/arm64"
)
$AndroidTarget = "android/arm64"

# --- locate the module root (works from any cwd) -----------------------------

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = (Resolve-Path (Join-Path $ScriptDir "..")).Path
Push-Location $RootDir
try {
    if (-not (Test-Path "go.mod")) {
        throw "go.mod not found in $RootDir; run this from inside the repository"
    }

    # --- version metadata ---------------------------------------------------

    if ([string]::IsNullOrWhiteSpace($Version)) {
        $Version = (Get-Content "VERSION" -Raw).Trim()
    }
    # The tag is what self-update compares against, so it must carry the v
    # prefix. A user-supplied -Version may or may not include it.
    $Tag = "v" + $Version.TrimStart("v")

    $Commit = try { (git rev-parse HEAD 2>$null) } catch { "none" }
    if ([string]::IsNullOrWhiteSpace($Commit)) { $Commit = "none" }
    $BuildDate = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

    # Default the self-update repository to the configured origin so a fork
    # builds binaries that update from the fork, exactly as CI does.
    if ([string]::IsNullOrWhiteSpace($RepoOwner)) {
        $origin = try { (git config --get remote.origin.url 2>$null) } catch { "" }
        if ($origin -match "github\.com[:/]([^/]+)/([^/.]+)") {
            $RepoOwner = $Matches[1]
            $RepoName = $Matches[2]
        } else {
            $RepoOwner = "zhaarey"
            $RepoName = "apple-music-downloader"
        }
    }

    Write-Host "amdl release build"
    Write-Host "  version    : $Tag"
    Write-Host "  commit     : $Commit"
    Write-Host "  built at   : $BuildDate"
    Write-Host "  repository : $RepoOwner/$RepoName"
    Write-Host "  output     : $Out"
    Write-Host ""

    # --- target selection ---------------------------------------------------

    if ($All) {
        $Selected = $DefaultTargets + $AndroidTarget
    } elseif (-not [string]::IsNullOrWhiteSpace($Targets)) {
        $Selected = $Targets -split "," | ForEach-Object { $_.Trim() } | Where-Object { $_ }
    } else {
        # Native build only: derive it from `go env` so the default is useful.
        $nativeOs = (go env GOOS).Trim()
        $nativeArch = (go env GOARCH).Trim()
        $Selected = @("$nativeOs/$nativeArch")
    }

    # --- helpers ------------------------------------------------------------

    function Get-ArtifactName {
        param([string]$Goos, [string]$Goarch)
        if ($Goos -eq "windows") { return "amdl_${Goos}_${Goarch}.exe" }
        return "amdl_${Goos}_${Goarch}"
    }

    # --- build --------------------------------------------------------------

    New-Item -ItemType Directory -Force -Path $Out | Out-Null
    $Built = @()

    foreach ($target in $Selected) {
        $parts = $target -split "/"
        if ($parts.Count -ne 2 -or -not $parts[0] -or -not $parts[1]) {
            throw "bad target '$target' (expected os/arch)"
        }
        $goos = $parts[0]
        $goarch = $parts[1]
        $name = Get-ArtifactName -Goos $goos -Goarch $goarch
        $outFile = Join-Path $Out $name

        Write-Host "building $name ..."

        # cgo needs a C compiler for the *target*, which only Android sets up in
        # CI. Every other target builds with cgo off so no toolchain is needed.
        $cgo = "0"
        if ($goos -eq "android") {
            $cgo = "1"
            if (-not $env:CC) {
                $ndkCc = Get-Command "aarch64-linux-android24-clang" -ErrorAction SilentlyContinue
                if (-not $ndkCc) {
                    Write-Warning "  skipped: no aarch64-linux-android24-clang on PATH (set `$env:CC)"
                    continue
                }
                $env:CC = $ndkCc.Source
            }
        }

        # -trimpath keeps local absolute paths out of the binary, which matters
        # for a published artifact; -s -w strips symbols and DWARF data.
        $ldflags = "-s -w " +
            "-X 'amdl/internal/version.Version=$Tag' " +
            "-X 'amdl/internal/version.Commit=$Commit' " +
            "-X 'amdl/internal/version.BuildDate=$BuildDate' " +
            "-X 'amdl/internal/updater.RepoOwner=$RepoOwner' " +
            "-X 'amdl/internal/updater.RepoName=$RepoName'"

        # NOTE: the subcommand must be the first element — `& go @args` passes
        # the array verbatim, so omitting "build" makes go parse -trimpath as a
        # top-level flag and fail with "flag provided but not defined".
        $buildArgs = @("build", "-trimpath", "-ldflags=$ldflags", "-o", $outFile, ".")
        if ($Gui -and $goos -eq "windows") {
            # -H=windowsgui selects the GUI subsystem: no console window is
            # created, so double-clicking the release binary behaves like a
            # normal desktop app. It also means stdout is discarded, which the
            # launcher already accounts for by showing a dialog on failure.
            $buildArgs = @("build", "-trimpath", "-ldflags=$ldflags -H=windowsgui", "-o", $outFile, ".")
        }

        $savedGoos = $env:GOOS
        $savedGoarch = $env:GOARCH
        $savedCgo = $env:CGO_ENABLED
        try {
            $env:GOOS = $goos
            $env:GOARCH = $goarch
            $env:CGO_ENABLED = $cgo
            & go @buildArgs
            if ($LASTEXITCODE -ne 0) { throw "go build failed for $target (exit $LASTEXITCODE)" }
        } finally {
            $env:GOOS = $savedGoos
            $env:GOARCH = $savedGoarch
            $env:CGO_ENABLED = $savedCgo
        }

        $Built += $name
    }

    if ($Built.Count -eq 0) { throw "nothing was built" }

    # --- checksums ----------------------------------------------------------

    # checksums.txt uses the same "hash  name" shape as the release workflow, so
    # a locally produced file is interchangeable with the published one.
    $lines = foreach ($n in $Built) {
        $hash = (Get-FileHash -Algorithm SHA256 (Join-Path $Out $n)).Hash.ToLower()
        "$hash  $n"
    }
    Set-Content -Path (Join-Path $Out "checksums.txt") -Value $lines -Encoding ASCII

    Write-Host ""
    Write-Host "done. artifacts in ${Out}:"
    Get-ChildItem $Out | Select-Object Name, @{n = "SizeMB"; e = { [math]::Round($_.Length / 1MB, 2) } } |
        Format-Table -AutoSize | Out-String | Write-Host
}
finally {
    Pop-Location
}