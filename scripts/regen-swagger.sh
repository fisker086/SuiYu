#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init \
  -g ./cmd/server/main.go \
  -o ./docs \
  --parseDependency \
  --parseInternal