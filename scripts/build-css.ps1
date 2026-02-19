# Regenerate Tailwind CSS
# Run this when you modify HTML templates to update the CSS

$ErrorActionPreference = "Stop"

Write-Host "Regenerating Tailwind CSS..." -ForegroundColor Cyan

# Get project root (parent of scripts directory)
$projectRoot = Split-Path $PSScriptRoot -Parent

# Check if tailwindcss.exe exists in project root
$tailwindExe = Join-Path $projectRoot "tailwindcss.exe"
if (-not (Test-Path $tailwindExe)) {
    Write-Host "Downloading Tailwind CSS CLI..." -ForegroundColor Yellow
    Invoke-WebRequest -Uri "https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.1/tailwindcss-windows-x64.exe" -OutFile $tailwindExe
}

# Run from project root so tailwind.config.js is found
Push-Location $projectRoot
try {
    & $tailwindExe -i ./web/static/css/tailwind.src.css -o ./web/static/css/tailwind.min.css --minify

    if ($LASTEXITCODE -eq 0) {
        $cssFile = Join-Path $projectRoot "web/static/css/tailwind.min.css"
        $size = [math]::Round((Get-Item $cssFile).Length / 1KB, 2)
        Write-Host "Generated: $cssFile ($size KB)" -ForegroundColor Green
    } else {
        Write-Host "Failed to generate CSS" -ForegroundColor Red
        exit 1
    }
} finally {
    Pop-Location
}
