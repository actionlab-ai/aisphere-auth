$ErrorActionPreference = "Stop"

if (-not $env:AISPHERE_SESSION_PROVIDER) { $env:AISPHERE_SESSION_PROVIDER = "redis" }
if (-not $env:AISPHERE_REDIS_ADDRS) { $env:AISPHERE_REDIS_ADDRS = "127.0.0.1:6379" }
if (-not $env:AISPHERE_REDIS_PREFIX) { $env:AISPHERE_REDIS_PREFIX = "aisphere" }
if (-not $env:AISPHERE_SERVICE_TOKEN_REQUIRED) { $env:AISPHERE_SERVICE_TOKEN_REQUIRED = "true" }
if (-not $env:AISPHERE_SERVICE_TOKEN) { $env:AISPHERE_SERVICE_TOKEN = "dev-service-token-change-me" }
if (-not $env:AISPHERE_SERVICE_TOKEN_HEADER) { $env:AISPHERE_SERVICE_TOKEN_HEADER = "X-Aisphere-Service-Token" }

go run ./cmd/server
