# FleetCtrl Build Script for Windows (PowerShell)
param(
    [string]$Version = "",
    [switch]$WindowsOnly,
    [switch]$LinuxOnly,
    [switch]$All,
    [switch]$SkipCSS
)

$ErrorActionPreference = "Stop"

# Read version from appVersion.txt if not provided
if (-not $Version) {
    $versionFile = Join-Path $PSScriptRoot "..\appVersion.txt"
    if (Test-Path $versionFile) {
        $Version = (Get-Content $versionFile -Raw).Trim()
        Write-Host "Using version from appVersion.txt: $Version" -ForegroundColor Magenta
    } else {
        $Version = "dev"
        Write-Host "appVersion.txt not found, using default: $Version" -ForegroundColor Yellow
    }
}

Write-Host "Building FleetCtrl $Version..." -ForegroundColor Cyan

# Build CSS first (unless skipped)
if (-not $SkipCSS) {
    Write-Host "`nRebuilding Tailwind CSS..." -ForegroundColor Cyan
    $cssScript = Join-Path $PSScriptRoot "build-css.ps1"
    if (Test-Path $cssScript) {
        & $cssScript
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Failed to build CSS" -ForegroundColor Red
            exit 1
        }
    } else {
        Write-Host "Warning: build-css.ps1 not found, skipping CSS build" -ForegroundColor Yellow
    }
} else {
    Write-Host "`nSkipping CSS build (-SkipCSS flag)" -ForegroundColor Yellow
}

# Create dist directory
if (-not (Test-Path "../dist")) {
    New-Item -ItemType Directory -Path "../dist" | Out-Null
}

function Build-Platform {
    param(
        [string]$OS,
        [string]$Arch,
        [string]$Output
    )

    Write-Host "  Building for $OS/$Arch..." -ForegroundColor Yellow
    $env:GOOS = $OS
    $env:GOARCH = $Arch

    $buildTime = (Get-Date).ToUniversalTime().ToString('yyyy-MM-dd_HH:mm:ss')
    $buildDate = (Get-Date).ToUniversalTime().ToString('yyyyMMdd')
    $gitCommit = "unknown"
    try {
        $gitCommit = (git rev-parse --short HEAD 2>$null)
        if (-not $gitCommit) { $gitCommit = "unknown" }
    } catch {
        $gitCommit = "unknown"
    }

    $pkg = "fleetctrl/internal/version"
    $ldflags = "-X $pkg.Version=$Version -X $pkg.BuildDate=$buildDate -X $pkg.BuildTime=$buildTime -X $pkg.GitCommit=$gitCommit"

    & go build "-ldflags=$ldflags" -o $Output ../cmd/fleetctrl

    if ($LASTEXITCODE -ne 0) {
        Write-Host "  Failed to build for $OS/$Arch" -ForegroundColor Red
        exit 1
    }

    $size = (Get-Item $Output).Length / 1MB
    Write-Host "  Created: $Output ($([math]::Round($size, 2)) MB)" -ForegroundColor Green
}

function Generate-WindowsResources {
    Write-Host "`nGenerating Windows resources (UAC manifest)..." -ForegroundColor Cyan

    # Check if go-winres is installed
    $goWinRes = Get-Command go-winres -ErrorAction SilentlyContinue
    if (-not $goWinRes) {
        Write-Host "  Installing go-winres..." -ForegroundColor Yellow
        & go install github.com/tc-hib/go-winres@latest
        if ($LASTEXITCODE -ne 0) {
            Write-Host "  Warning: Failed to install go-winres, skipping manifest embedding" -ForegroundColor Yellow
            return $false
        }
    }

    # Generate .syso file
    Push-Location "../cmd/fleetctrl"
    & go-winres make --in winres/winres.json --out rsrc_windows_amd64.syso --arch amd64
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  Warning: Failed to generate Windows resources" -ForegroundColor Yellow
        Pop-Location
        return $false
    }
    Pop-Location

    Write-Host "  Windows resources generated successfully" -ForegroundColor Green
    return $true
}

# Determine what to build
# Default (no flags): build all platforms
# -WindowsOnly: build only Windows
# -LinuxOnly: build only Linux
# -All: build all platforms (same as default)
$buildAll = (-not $WindowsOnly -and -not $LinuxOnly) -or $All
$buildWindows = $WindowsOnly -or $buildAll
$buildLinux = $LinuxOnly -or $buildAll

if ($buildWindows) {
    # Generate Windows resources (UAC manifest) before building
    Generate-WindowsResources

    Write-Host "`nBuilding Windows binaries:" -ForegroundColor Cyan
    Build-Platform -OS "windows" -Arch "amd64" -Output "../dist/fleetctrl-windows-amd64.exe"
}

if ($buildLinux) {
    Write-Host "`nBuilding Linux binaries:" -ForegroundColor Cyan
    Build-Platform -OS "linux" -Arch "amd64" -Output "../dist/fleetctrl-linux-amd64"
    Build-Platform -OS "linux" -Arch "arm64" -Output "../dist/fleetctrl-linux-arm64"
}

# Reset environment
$env:GOOS = ""
$env:GOARCH = ""

Write-Host "`nBuild complete!" -ForegroundColor Green
Write-Host "Binaries are in ../dist/:" -ForegroundColor Cyan
Get-ChildItem ../dist | Format-Table Name, @{N='Size (MB)';E={[math]::Round($_.Length / 1MB, 2)}}
