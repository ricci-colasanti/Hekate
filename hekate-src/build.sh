#!/bin/bash
# ============================================================================
# build.sh - Build Hekate for all platforms
# ============================================================================
#
# This script builds Hekate for Linux, Windows, and macOS. It builds the
# entire Go package (all .go files in the current directory), producing
# standalone binaries with no external dependencies.
#
# USAGE:
#   ./build.sh              # Build all platforms
#   ./build.sh linux        # Build only Linux
#   ./build.sh windows      # Build only Windows
#   ./build.sh macos        # Build only macOS
#   ./build.sh clean        # Clean build artifacts
#
# ============================================================================

set -e  # Exit immediately on any error

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
BINARY_NAME="hekate"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE=$(date -u +"%Y-%m-%d_%H:%M:%S_UTC")
BUILD_TIME=$(date -u +%s)

# Output directory (keeps binaries organised)
OUTPUT_DIR="dist"

# Build flags
# -s -w : strip debug info and symbol table (smaller binaries)
# -X    : inject version/build info into the binary (if main has vars for it)
LDFLAGS="-s -w"
LDFLAGS="${LDFLAGS} -X main.Version=${VERSION}"
LDFLAGS="${LDFLAGS} -X main.BuildDate=${BUILD_DATE}"

# ---------------------------------------------------------------------------
# Colours (disabled if not a terminal)
# ---------------------------------------------------------------------------
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    BOLD='\033[1m'
    NC='\033[0m'  # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    BOLD=''
    NC=''
fi

# ---------------------------------------------------------------------------
# Helper functions
# ---------------------------------------------------------------------------

print_header() {
    echo ""
    echo "════════════════════════════════════════════════════════"
    echo "  ${BOLD}$1${NC}"
    echo "════════════════════════════════════════════════════════"
    echo ""
}

print_step() {
    echo "${BLUE}▶${NC} $1"
}

print_success() {
    echo "${GREEN}✅${NC} $1"
}

print_warning() {
    echo "${YELLOW}⚠️ ${NC} $1"
}

print_error() {
    echo "${RED}❌${NC} $1"
}

# Show human-readable file size
show_size() {
    local file="$1"
    if [ -f "$file" ]; then
        ls -lh "$file" | awk '{print "   Size: " $5}'
    fi
}

# Verify binary has no dynamic dependencies (Linux only)
verify_static() {
    local file="$1"
    if [ -f "$file" ] && command -v ldd >/dev/null 2>&1; then
        local deps
        deps=$(ldd "$file" 2>&1 || true)
        if echo "$deps" | grep -q "not a dynamic executable"; then
            echo "   ${GREEN}✓${NC} Statically linked (no external dependencies)"
        elif echo "$deps" | grep -q "not found"; then
            echo "   ${YELLOW}⚠${NC} Some dependencies missing"
        else
            echo "   ${YELLOW}⚠${NC} Dynamically linked"
        fi
    fi
}

# ---------------------------------------------------------------------------
# Build functions
# ---------------------------------------------------------------------------

build_linux() {
    print_step "Building for Linux (x86_64)..."
    GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" \
        -o "${OUTPUT_DIR}/hekate-linux-amd64" .
    chmod +x "${OUTPUT_DIR}/hekate-linux-amd64"
    print_success "${OUTPUT_DIR}/hekate-linux-amd64"
    show_size "${OUTPUT_DIR}/hekate-linux-amd64"
    verify_static "${OUTPUT_DIR}/hekate-linux-amd64"
}

build_linux_arm64() {
    print_step "Building for Linux (ARM64)..."
    GOOS=linux GOARCH=arm64 go build -ldflags "${LDFLAGS}" \
        -o "${OUTPUT_DIR}/hekate-linux-arm64" .
    chmod +x "${OUTPUT_DIR}/hekate-linux-arm64"
    print_success "${OUTPUT_DIR}/hekate-linux-arm64"
    show_size "${OUTPUT_DIR}/hekate-linux-arm64"
    verify_static "${OUTPUT_DIR}/hekate-linux-arm64"
}

build_windows() {
    print_step "Building for Windows (x86_64)..."
    GOOS=windows GOARCH=amd64 go build -ldflags "${LDFLAGS}" \
        -o "${OUTPUT_DIR}/hekate-windows-amd64.exe" .
    print_success "${OUTPUT_DIR}/hekate-windows-amd64.exe"
    show_size "${OUTPUT_DIR}/hekate-windows-amd64.exe"
}

build_macos_intel() {
    print_step "Building for macOS (Intel x86_64)..."
    GOOS=darwin GOARCH=amd64 go build -ldflags "${LDFLAGS}" \
        -o "${OUTPUT_DIR}/hekate-darwin-amd64" .
    chmod +x "${OUTPUT_DIR}/hekate-darwin-amd64"
    print_success "${OUTPUT_DIR}/hekate-darwin-amd64"
    show_size "${OUTPUT_DIR}/hekate-darwin-amd64"
}

build_macos_arm64() {
    print_step "Building for macOS (Apple Silicon ARM64)..."
    GOOS=darwin GOARCH=arm64 go build -ldflags "${LDFLAGS}" \
        -o "${OUTPUT_DIR}/hekate-darwin-arm64" .
    chmod +x "${OUTPUT_DIR}/hekate-darwin-arm64"
    print_success "${OUTPUT_DIR}/hekate-darwin-arm64"
    show_size "${OUTPUT_DIR}/hekate-darwin-arm64"
}

# ---------------------------------------------------------------------------
# Clean function
# ---------------------------------------------------------------------------

clean_build() {
    print_header "Cleaning build artifacts"
    print_step "Removing ${OUTPUT_DIR}/ directory..."
    rm -rf "${OUTPUT_DIR}"
    print_step "Removing old-style binaries (hekate-linux, etc.)..."
    rm -f hekate-linux hekate-windows.exe hekate-macos hekate-linux-amd64 \
          hekate-linux-arm64 hekate-windows-amd64.exe hekate-darwin-amd64 \
          hekate-darwin-arm64
    print_success "Clean complete"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

print_header "Building Hekate for All Platforms"

# Show version info
echo "  Version:     ${VERSION}"
echo "  Build date:  ${BUILD_DATE}"
echo "  Output dir:  ${OUTPUT_DIR}/"
echo ""

# Check Go is installed
if ! command -v go >/dev/null 2>&1; then
    print_error "Go is not installed or not in PATH"
    echo "   Install Go from: https://go.dev/dl/"
    exit 1
fi

# Check we have .go files in the current directory
if ! ls *.go >/dev/null 2>&1; then
    print_error "No .go files found in current directory"
    echo "   Run this script from the root of the Hekate project"
    exit 1
fi

# Check go.mod exists
if [ ! -f "go.mod" ]; then
    print_warning "go.mod not found - running 'go mod init hekate'"
    go mod init hekate
fi

# Ensure dependencies are present
print_step "Verifying Go dependencies..."
go mod tidy
print_success "Dependencies OK"
echo ""

# Create output directory
mkdir -p "${OUTPUT_DIR}"

# Parse command-line argument
case "${1:-all}" in
    clean)
        clean_build
        exit 0
        ;;
    linux)
        clean_build
        mkdir -p "${OUTPUT_DIR}"
        build_linux
        ;;
    linux-arm)
        clean_build
        mkdir -p "${OUTPUT_DIR}"
        build_linux_arm64
        ;;
    windows)
        clean_build
        mkdir -p "${OUTPUT_DIR}"
        build_windows
        ;;
    macos)
        clean_build
        mkdir -p "${OUTPUT_DIR}"
        build_macos_intel
        ;;
    macos-arm)
        clean_build
        mkdir -p "${OUTPUT_DIR}"
        build_macos_arm64
        ;;
    all|"")
        # Clean only the output dir, don't delete the whole dist
        print_step "Cleaning previous builds..."
        rm -rf "${OUTPUT_DIR}"
        mkdir -p "${OUTPUT_DIR}"
        print_success "Clean complete"
        echo ""
        
        # Build all platforms
        build_linux
        echo ""
        build_linux_arm64
        echo ""
        build_windows
        echo ""
        build_macos_intel
        echo ""
        build_macos_arm64
        ;;
    *)
        print_error "Unknown target: $1"
        echo ""
        echo "Usage: $0 [target]"
        echo ""
        echo "Targets:"
        echo "  (none)      Build all platforms"
        echo "  all         Build all platforms"
        echo "  linux       Build Linux x86_64 only"
        echo "  linux-arm   Build Linux ARM64 only"
        echo "  windows     Build Windows x86_64 only"
        echo "  macos       Build macOS Intel only"
        echo "  macos-arm   Build macOS Apple Silicon only"
        echo "  clean       Remove all build artifacts"
        exit 1
        ;;
esac

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

print_header "Build Complete"

echo "📦 Binaries created in ${OUTPUT_DIR}/:"
echo ""
if [ -d "${OUTPUT_DIR}" ]; then
    for binary in "${OUTPUT_DIR}"/hekate-*; do
        if [ -f "$binary" ]; then
            size=$(ls -lh "$binary" | awk '{print $5}')
            name=$(basename "$binary")
            printf "   %-30s %s\n" "$name" "$size"
        fi
    done
fi
echo ""

echo "▶️  To run:"
echo "   Linux (x86_64):     ./${OUTPUT_DIR}/hekate-linux-amd64 config.yaml"
echo "   Linux (ARM64):      ./${OUTPUT_DIR}/hekate-linux-arm64 config.yaml"
echo "   Windows:            ${OUTPUT_DIR}\\hekate-windows-amd64.exe config.yaml"
echo "   macOS (Intel):      ./${OUTPUT_DIR}/hekate-darwin-amd64 config.yaml"
echo "   macOS (Apple Silicon): ./${OUTPUT_DIR}/hekate-darwin-arm64 config.yaml"
echo ""

echo "🔍 Verify no external dependencies:"
echo "   Linux:   ldd ${OUTPUT_DIR}/hekate-linux-amd64"
echo "   macOS:   otool -L ${OUTPUT_DIR}/hekate-darwin-amd64"
echo "   Windows: Use Dependency Walker or dumpbin"
echo ""

print_success "Done!"
echo ""