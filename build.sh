#!/usr/bin/env bash
set -e

export PATH=$PATH:/usr/local/go/bin
export GOTOOLCHAIN=local

echo "=========================================================="
echo "  Building Bauhaus Manga Downloader (Single Executable)"
echo "=========================================================="

# 1. Build Frontend
echo "[1/2] Building Bauhaus Frontend Assets..."
cd web
npm install
npm run build
cd ..

# 2. Build Go Binary
echo "[2/2] Compiling Single Go Binary with embedded assets..."
export GOTOOLCHAIN=local
go build -ldflags="-s -w" -o suencomic .
# also keep manga-downloader symlink/alias
ln -sf suencomic manga-downloader

echo "=========================================================="
echo "✓ Build successful! Executable created: ./suencomic"
echo "  Run with: ./suencomic -port 8090"
echo "=========================================================="
