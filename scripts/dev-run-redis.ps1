$ErrorActionPreference = "Stop"

if (-not $env:AISPHERE_SESSION_PROVIDER) { $env:AISPHERE_SESSION_PROVIDER = "redis" }
if (-not $env:AISPHERE_REDIS_ADDRS) { $env:AISPHERE_REDIS_ADDRS = "127.0.0.1:6379" }
if (-not $env:AISPHERE_REDIS_PREFIX) { $env:AISPHERE_REDIS_PREFIX = "aisphere" }

go run ./cmd/server
