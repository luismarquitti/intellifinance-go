#!/usr/bin/env bash
set -euo pipefail

echo "=========================================================="
echo "  Setting up IntelliFinance environment for Google Jules  "
echo "=========================================================="

# 1. Verify Go version
go version

# 2. Install golangci-lint if not present
if ! command -v golangci-lint &> /dev/null; then
  echo "Installing golangci-lint..."
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v1.57.2
fi

# 3. Install sqlc for type-safe database queries codegen
if ! command -v sqlc &> /dev/null; then
  echo "Installing sqlc..."
  go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.26.0
fi

# 4. Download Go dependencies
if [ -f "go.mod" ]; then
  echo "Downloading Go modules..."
  go mod download
fi

echo "==> Google Jules environment ready!"
