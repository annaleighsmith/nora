#!/bin/bash
set -e

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY="nora"

echo "Building $BINARY..."
go build -o "$BINARY" .

echo "Installing to $INSTALL_DIR/$BINARY..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$BINARY" "$INSTALL_DIR/$BINARY"
else
    sudo mv "$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "Installed $(nora version 2>/dev/null || echo "$BINARY") to $INSTALL_DIR/$BINARY"
echo ""
echo "Run 'nora setup' to get started."
