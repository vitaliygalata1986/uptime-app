#!/usr/bin/env bash
# Regenerate the contract, then build ReDoc only if swaggo succeeds.
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../../.." && pwd)"

cd -- "$repo_root/backend"
go generate .
npm --prefix .. run docs:redoc
