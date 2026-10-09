#!/usr/bin/env bash
set -euo pipefail
exec node scripts/release/build.mjs "${1:-0.1.0-dev}"
