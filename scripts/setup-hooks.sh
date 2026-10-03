#!/usr/bin/env bash
set -euo pipefail

echo "=========================================================="
echo "  Installing and Setting Up Git Pre-Commit Hooks          "
echo "=========================================================="

HOOK_FILE=".git/hooks/pre-commit"

mkdir -p .git/hooks

cat << 'EOF' > "$HOOK_FILE"
#!/usr/bin/env bash
set -e

echo "==> Running pre-commit security and quality checks..."

# 1. Gitleaks scan
if command -v gitleaks &> /dev/null; then
  echo "--> Running Gitleaks..."
  gitleaks detect --config=.gitleaks.toml --no-git --verbose
elif [ -f "$(go env GOPATH)/bin/gitleaks" ]; then
  echo "--> Running Gitleaks from GOPATH..."
  "$(go env GOPATH)/bin/gitleaks" detect --config=.gitleaks.toml --no-git --verbose
else
  echo "[WARN] Gitleaks not installed locally. Skipping local scan."
fi

# 2. GolangCI-Lint
if command -v golangci-lint &> /dev/null; then
  echo "--> Running GolangCI-Lint..."
  golangci-lint run
elif [ -f "$(go env GOPATH)/bin/golangci-lint" ]; then
  echo "--> Running GolangCI-Lint from GOPATH..."
  "$(go env GOPATH)/bin/golangci-lint" run
else
  echo "[WARN] GolangCI-Lint not installed locally. Skipping local lint."
fi

echo "==> Pre-commit checks completed successfully!"
EOF

chmod +x "$HOOK_FILE"
echo "Git pre-commit hook installed at $HOOK_FILE"
