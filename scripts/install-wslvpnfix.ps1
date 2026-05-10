# scripted/written by Robert Bopko (github.com/zeroznet) with Boba Bott (Claude Opus 4.7)
#
# wsl-vpnfix Windows-side installer.
#
# Resolves the requested release tag from GitHub, downloads the rootfs
# tarball plus SHA256SUMS, verifies the tarball's SHA-256 against the
# manifest, and imports it as a WSL 2 distro. Designed to run from a
# `iwr ... | iex` one-liner; also runs cleanly from a saved file.
#
# Requires: PowerShell 5.1+ (Windows 10 22H2 ships 5.1, Windows 11 ships
# 7.x via the Store), `wsl.exe` on PATH, network access to github.com.
# Does NOT require Windows admin rights.

[CmdletBinding()]
param(
    [string]$Tag = 'latest',
    [string]$DistroName = 'wsl-vpnfix',
    [string]$InstallDir = "$env:LOCALAPPDATA\wsl-vpnfix",
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$Repo = 'zeroznet/wsl-vpnfix'
$ApiBase = "https://api.github.com/repos/$Repo"
$DlBase = "https://github.com/$Repo/releases/download"

function Write-Step { param($Msg) Write-Host "==> $Msg" -ForegroundColor Cyan }
function Write-Ok   { param($Msg) Write-Host "    $Msg" -ForegroundColor Green }
function Write-Warn { param($Msg) Write-Host "!!! $Msg" -ForegroundColor Yellow }
function Die        { param($Msg) Write-Host "*** $Msg" -ForegroundColor Red; exit 1 }

if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
    Die 'wsl.exe not found on PATH. Install WSL first: `wsl --install`.'
}

Write-Step "Resolving release tag '$Tag' from $Repo"
try {
    $headers = @{ 'User-Agent' = 'wsl-vpnfix-installer'; 'Accept' = 'application/vnd.github+json' }
    if ($Tag -eq 'latest') {
        $rel = Invoke-RestMethod -Headers $headers -Uri "$ApiBase/releases/latest"
    } else {
        $rel = Invoke-RestMethod -Headers $headers -Uri "$ApiBase/releases/tags/$Tag"
    }
} catch {
    Die "GitHub API request failed: $($_.Exception.Message)"
}

$ResolvedTag = $rel.tag_name
if (-not $ResolvedTag -or $ResolvedTag -notmatch '^v\d+\.\d+\.\d+$') {
    Die "release tag '$ResolvedTag' does not match vN.N.N"
}
$Version = $ResolvedTag.Substring(1)
$Tarball = "wsl-vpnfix-$Version.tar.gz"
$TarUrl  = "$DlBase/$ResolvedTag/$Tarball"
$SumsUrl = "$DlBase/$ResolvedTag/SHA256SUMS"
Write-Ok "tag=$ResolvedTag, tarball=$Tarball"

$Tmp = Join-Path ([System.IO.Path]::GetTempPath()) "wsl-vpnfix-$([guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Force -Path $Tmp | Out-Null
$TarPath  = Join-Path $Tmp $Tarball
$SumsPath = Join-Path $Tmp 'SHA256SUMS'

try {
    Write-Step "Downloading $Tarball"
    Invoke-WebRequest -Uri $TarUrl -OutFile $TarPath -UseBasicParsing
    $size = [math]::Round((Get-Item $TarPath).Length / 1MB, 2)
    Write-Ok "got $size MB"

    Write-Step 'Downloading SHA256SUMS'
    Invoke-WebRequest -Uri $SumsUrl -OutFile $SumsPath -UseBasicParsing

    Write-Step 'Verifying SHA-256'
    $expected = (Get-Content $SumsPath |
        Where-Object { $_ -match "\s$([regex]::Escape($Tarball))$" } |
        ForEach-Object { ($_ -split '\s+')[0] } |
        Select-Object -First 1)
    if (-not $expected) { Die "no entry for $Tarball in SHA256SUMS" }
    $actual = (Get-FileHash -Algorithm SHA256 -Path $TarPath).Hash.ToLowerInvariant()
    $expected = $expected.ToLowerInvariant()
    if ($actual -ne $expected) {
        Die "SHA-256 mismatch:`n  expected: $expected`n  actual:   $actual"
    }
    Write-Ok "sha256=$actual"

    $existing = (& wsl.exe --list --quiet) 2>$null |
        ForEach-Object { ($_ -replace "`0", '').Trim() } |
        Where-Object { $_ -eq $DistroName }
    if ($existing) {
        if ($Force) {
            Write-Warn "distro '$DistroName' exists, --Force given, unregistering"
        } else {
            $reply = Read-Host "Distro '$DistroName' already exists. Unregister and overwrite? [y/N]"
            if ($reply -notmatch '^[Yy]') { Die 'aborted by user' }
        }
        Write-Step "Terminating + unregistering existing '$DistroName'"
        & wsl.exe --terminate $DistroName 2>$null | Out-Null
        & wsl.exe --unregister $DistroName
        if ($LASTEXITCODE -ne 0) { Die "wsl --unregister failed (exit $LASTEXITCODE)" }
    }

    Write-Step "Importing into $InstallDir"
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    & wsl.exe --import $DistroName $InstallDir $TarPath --version 2
    if ($LASTEXITCODE -ne 0) { Die "wsl --import failed (exit $LASTEXITCODE)" }
    Write-Ok "imported as '$DistroName'"

} finally {
    Remove-Item -Recurse -Force -Path $Tmp -ErrorAction SilentlyContinue
}

Write-Host ''
Write-Host 'Installed.' -ForegroundColor Green
Write-Host ''
Write-Host 'Next:' -ForegroundColor Cyan
Write-Host "  wsl -d $DistroName            # start the appliance (keep window open)"
Write-Host "  wsl -d Ubuntu -- curl -sI https://1.1.1.1   # validate from a sibling distro"
Write-Host ''
Write-Host "Uninstall:  wsl --unregister $DistroName"
Write-Host ''
