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

# TODO: Remove this block before publishing — dev convenience only
# Reset prompts to defaults (removes cached copies so they regenerate)
PROMPTS_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/nora/prompts"
if [ -d "$PROMPTS_DIR" ]; then
    echo "Resetting prompts to defaults..."
    rm -f "$PROMPTS_DIR"/*.md
fi

echo ""
echo "Run 'nora setup' to get started."
