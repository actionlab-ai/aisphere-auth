#!/usr/bin/env bash
set -euo pipefail

export AISPHERE_SESSION_PROVIDER="${AISPHERE_SESSION_PROVIDER:-redis}"
export AISPHERE_REDIS_ADDRS="${AISPHERE_REDIS_ADDRS:-127.0.0.1:6379}"
export AISPHERE_REDIS_PREFIX="${AISPHERE_REDIS_PREFIX:-aisphere}"

go run ./cmd/server
