#!/bin/bash
# build.sh - Build Hekate for all platforms

set -e  # Exit on error

echo "════════════════════════════════════════════════════════"
echo "  Building Hekate for All Platforms"
echo "════════════════════════════════════════════════════════"
echo ""

# Clean previous builds
echo "🧹 Cleaning previous builds..."
rm -f hekate-linux hekate-linux-arm64 hekate-windows.exe hekate-macos hekate-macos-arm64
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

# Build for Linux (ARM64)
echo "🐧 Building for Linux (ARM64)..."
GOOS=linux GOARCH=arm64 go build -o hekate-linux-arm64 main.go
chmod +x hekate-linux-arm64
echo "✅ hekate-linux-arm64"
file hekate-linux-arm64 | head -1
ls -lh hekate-linux-arm64 | awk '{print "   Size: " $5}'
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

# Build for macOS (Apple Silicon ARM64)
echo "🍎 Building for macOS (Apple Silicon ARM64)..."
GOOS=darwin GOARCH=arm64 go build -o hekate-macos-arm64 main.go
chmod +x hekate-macos-arm64
echo "✅ hekate-macos-arm64"
file hekate-macos-arm64 | head -1
ls -lh hekate-macos-arm64 | awk '{print "   Size: " $5}'
echo ""

# Summary
echo "════════════════════════════════════════════════════════"
echo "  ✅ Build Complete!"
echo "════════════════════════════════════════════════════════"
echo ""
echo "📦 Binaries created:"
echo "   Linux (x86_64):     hekate-linux"
echo "   Linux (ARM64):      hekate-linux-arm64"
echo "   Windows (x86_64):   hekate-windows.exe"
echo "   macOS (Intel):      hekate-macos"
echo "   macOS (Apple Silicon): hekate-macos-arm64"
echo ""
echo "▶️  To run:"
echo "   Linux (x86_64):   ./hekate-linux config.yaml"
echo "   Linux (ARM64):    ./hekate-linux-arm64 config.yaml"
echo "   Windows:          hekate-windows.exe config.yaml"
echo "   macOS (Intel):    ./hekate-macos config.yaml"
echo "   macOS (Apple Silicon): ./hekate-macos-arm64 config.yaml"
echo ""
echo "🔍 Verify no external dependencies:"
echo "   Linux:   ldd hekate-linux"
echo "   Windows: (check with depends.exe or similar)"
echo "   macOS:   otool -L hekate-macos"
echo "   macOS:   otool -L hekate-macos-arm64"
echo ""
echo "📝 To check architecture:"
echo "   Linux:   file hekate-*"
echo "   macOS:   file hekate-macos*"
