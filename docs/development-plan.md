# Development Plan

## Milestone 0 - Bootstrap

- Go module
- Gin server
- healthz and readyz
- environment based config
- request id middleware

## Milestone 1 - Interfaces

- Principal model
- Authn service interface
- Authz service interface
- Session store interface and memory store
- Casdoor identity client interface
- Token issuer and verifier interfaces
- Public SDK principal and Gin middleware skeleton

## Milestone 2 - Casdoor login

- Implement Casdoor SDK adapter
- Add login state store
- Implement login and callback handlers
- Normalize Casdoor user info into Principal

## Milestone 3 - Redis session

- Implement Redis session store
- Create AI Sphere session cookie
- Implement auth me and logout

## Milestone 4 - Authz check

- Implement Casdoor enforce adapter
- Implement authz check and batch check handlers
- Add short TTL authz cache and audit log

## Milestone 5 - SkillHub integration

- Replace direct Casdoor token handling with aisphere-auth SDK
- Protect one admin endpoint first
- Expand to full permission matrix
