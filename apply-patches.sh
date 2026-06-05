#!/bin/sh
set -e
echo "Applying micro-vscode patches..."

if [ ! -f "go.mod" ]; then
    echo "Error: Run this from the micro repository root."
    exit 1
fi

echo "1. Applying diff patch..."
git apply micro-vscode.patch

echo "2. Copying new files..."
NEW_FILES="internal/action/filepicker.go
internal/action/palette.go
internal/action/panels.go
internal/config/workspace.go"

for f in $NEW_FILES; do
    src="micro-vscode-files/$f"
    if [ -f "$src" ]; then
        mkdir -p "$(dirname "$f")"
        cp "$src" "$f"
        echo "  $f"
    fi
done

echo "3. Building..."
make build

echo "Done. Run ./micro to start."
