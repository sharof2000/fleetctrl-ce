#!/bin/bash

# FleetCtrl Build Script

set -e

SKIP_CSS=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-css)
            SKIP_CSS=true
            shift
            ;;
        *)
            VERSION="$1"
            shift
            ;;
    esac
done

# Determine the script directory and project root
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Read version from appVersion.txt if not provided as argument
if [ -z "$VERSION" ]; then
    if [ -f "$PROJECT_ROOT/appVersion.txt" ]; then
        VERSION=$(cat "$PROJECT_ROOT/appVersion.txt" | tr -d '[:space:]')
        echo "Using version from appVersion.txt: $VERSION"
    else
        VERSION="dev"
        echo "appVersion.txt not found, using default: $VERSION"
    fi
fi

BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
BUILD_DATE=$(date -u '+%Y%m%d')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

PKG="fleetctrl/internal/version"
LDFLAGS="-X ${PKG}.Version=${VERSION} -X ${PKG}.BuildDate=${BUILD_DATE} -X ${PKG}.BuildTime=${BUILD_TIME} -X ${PKG}.GitCommit=${GIT_COMMIT}"

echo "Building FleetCtrl ${VERSION} (${GIT_COMMIT})..."

# Change to project root for all build operations
cd "$PROJECT_ROOT"

# Build CSS first (unless skipped)
if [ "$SKIP_CSS" = false ]; then
    echo ""
    echo "Rebuilding Tailwind CSS..."
    if [ -f "$SCRIPT_DIR/build-css.sh" ]; then
        bash "$SCRIPT_DIR/build-css.sh"
    else
        echo "Warning: build-css.sh not found, skipping CSS build"
    fi
else
    echo ""
    echo "Skipping CSS build (--skip-css flag)"
fi

# Create dist directory
mkdir -p dist

# Build for Linux AMD64
echo "Building for Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o dist/fleetctrl-linux-amd64 ./cmd/fleetctrl

# Build for Linux ARM64
echo "Building for Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o dist/fleetctrl-linux-arm64 ./cmd/fleetctrl

# Build for Windows AMD64
echo "Building for Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o dist/fleetctrl-windows-amd64.exe ./cmd/fleetctrl

echo "Build complete! Binaries are in ./dist/"
ls -la dist/
