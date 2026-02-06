#!/bin/bash
# Install git hooks from scripts/hooks to .git/hooks

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
HOOKS_DIR="$PROJECT_ROOT/.git/hooks"

echo "Installing git hooks..."

# Copy hooks
for hook in pre-commit pre-push; do
    if [ -f "$SCRIPT_DIR/hooks/$hook" ]; then
        echo "Installing $hook hook..."
        cp "$SCRIPT_DIR/hooks/$hook" "$HOOKS_DIR/$hook"
        chmod +x "$HOOKS_DIR/$hook"
        echo "✓ $hook installed"
    fi
done

echo ""
echo "✓ Git hooks installed successfully"
echo ""
echo "Hooks installed:"
echo "  - pre-commit: Runs linter (gofmt, go vet)"
echo "  - pre-push: Runs unit tests"
echo ""
echo "To bypass hooks (not recommended):"
echo "  git commit --no-verify"
echo "  git push --no-verify"
