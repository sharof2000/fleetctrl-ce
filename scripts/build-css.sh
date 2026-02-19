#!/bin/bash

# Regenerate Tailwind CSS
# Run this when you modify HTML templates to update the CSS

set -e

echo "Regenerating Tailwind CSS..."

# Determine the script directory and project root
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

# Determine platform for downloading correct binary
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="x64" ;;
    aarch64|arm64) ARCH="arm64" ;;
esac

TAILWIND_BIN="tailwindcss"

# Check if tailwindcss exists
if [ ! -f "$TAILWIND_BIN" ]; then
    echo "Downloading Tailwind CSS CLI..."
    case "$OS" in
        linux)
            curl -sLO "https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.1/tailwindcss-linux-${ARCH}"
            mv "tailwindcss-linux-${ARCH}" "$TAILWIND_BIN"
            ;;
        darwin)
            curl -sLO "https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.1/tailwindcss-macos-${ARCH}"
            mv "tailwindcss-macos-${ARCH}" "$TAILWIND_BIN"
            ;;
        *)
            echo "Unsupported OS: $OS"
            exit 1
            ;;
    esac
    chmod +x "$TAILWIND_BIN"
fi

# Generate CSS
./$TAILWIND_BIN -i web/static/css/tailwind.src.css -o web/static/css/tailwind.min.css --minify

SIZE=$(du -h web/static/css/tailwind.min.css | cut -f1)
echo "Generated: web/static/css/tailwind.min.css ($SIZE)"
