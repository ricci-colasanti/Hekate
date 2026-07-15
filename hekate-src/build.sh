#!/bin/bash
# build.sh - Build Hekate for all platforms

set -e  # Exit on error

echo "════════════════════════════════════════════════════════"
echo "  Building Hekate for All Platforms"
echo "════════════════════════════════════════════════════════"
echo ""

# Clean previous builds
echo "🧹 Cleaning previous builds..."
rm -f hekate-linux hekate-windows.exe hekate-macos
echo "✅ Clean complete"
echo ""

# Build for Linux (x86_64)
echo "🐧 Building for Linux (x86_64)..."
GOOS=linux GOARCH=amd64 go build -o hekate-linux main.go
chmod +x hekate-linux
echo "✅ hekate-linux"
file hekate-linux | head -1
ls -lh hekate-linux | awk '{print "   Size: " $5}'
echo ""

# Build for Windows (x86_64)
echo "🪟 Building for Windows (x86_64)..."
GOOS=windows GOARCH=amd64 go build -o hekate-windows.exe main.go
echo "✅ hekate-windows.exe"
ls -lh hekate-windows.exe | awk '{print "   Size: " $5}'
echo ""

# Build for macOS (Intel x86_64)
echo "🍎 Building for macOS (Intel x86_64)..."
GOOS=darwin GOARCH=amd64 go build -o hekate-macos main.go
chmod +x hekate-macos
echo "✅ hekate-macos"
file hekate-macos | head -1
ls -lh hekate-macos | awk '{print "   Size: " $5}'
echo ""

# Summary
echo "════════════════════════════════════════════════════════"
echo "  ✅ Build Complete!"
echo "════════════════════════════════════════════════════════"
echo ""
echo "📦 Binaries created:"
echo "   Linux:   hekate-linux"
echo "   Windows: hekate-windows.exe"
echo "   macOS:   hekate-macos"
echo ""
echo "▶️  To run:"
echo "   Linux:   ./hekate-linux config.yaml"
echo "   Windows: hekate-windows.exe config.yaml"
echo "   macOS:   ./hekate-macos config.yaml"
echo ""
echo "🔍 Verify no external dependencies:"
echo "   Linux:   ldd hekate-linux"
echo "   Windows: (check with depends.exe or similar)"
echo "   macOS:   otool -L hekate-macos"