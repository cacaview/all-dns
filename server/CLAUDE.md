# CLAUDE.md

This file provides guidance to Claude Code when working in `server/`.

## Scope

These instructions apply to the Go backend API, background reminder worker, migrations, and provider integrations under `server/`.

## Commands

Run commands from `server/` unless otherwise noted.

- Run all backend tests: `go test ./...`
- Run a single package test: `go test ./path/to/package -run TestName`
- Run the API locally: `go run ./cmd/api`
- Build the API: `go build ./cmd/api`

## Backend structure

Important directories:

- `cmd/api/` - process entrypoint and startup wiring
- `internal/config/` - environment/config loading
- `internal/db/` - DB connection and runtime migration setup
- `internal/http/` - Gin router, handlers, and middleware
- `internal/service/` - business logic; keep this as the main home for domain rules
- `internal/provider/` - DNS provider abstraction and provider adapters
- `internal/model/` - GORM models
- `internal/oauth/` - OAuth providers
- `internal/notifier/` - reminder webhook delivery
- `internal/storage/` - file storage abstraction (`Storage` interface, `LocalStorage`, `S3Storage`)
- `migrations/` - SQL migration artifacts; runtime startup still uses Go migration code

## Coding expectations

- Keep handlers thin; push business rules into `internal/service/`.
- Preserve encrypted-at-rest handling for provider credentials.
- Preserve validate-then-sync account behavior when touching account provisioning flows.
- Preserve backup snapshot creation before record mutations.
- Preserve propagation checks after record upserts and restore flows.
- Prefer extending provider adapters over branching provider-specific logic inside handlers or generic services.

## Provider work

Provider registration flows through `internal/provider/provider.go` and provider packages are wired into `internal/service/dns_service.go` via blank imports.

When adding a provider:

1. Create a dedicated adapter package under `internal/provider/<name>/`.
2. Register both the provider factory and descriptor in the provider package.
3. Keep provider-specific config shape inside the adapter package.
4. Avoid special-casing provider names in handlers.

## Files and directories to avoid treating as source

- `tmp/` is local runtime output.
- `uploads/` stores user-uploaded artifacts.
- Do not add durable instructions to transient output directories.
