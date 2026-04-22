# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/http/`.

## Scope

This layer owns Gin routing, HTTP handlers, request binding, auth middleware, and response shaping.

## Expectations

- Keep handlers thin and delegate business rules to `server/internal/service/`.
- Put auth and RBAC concerns in middleware or explicit route wiring.
- Keep API shape under `/api/v1` unless the user asks to change it.
- Preserve existing route grouping for auth, dashboard, accounts, domains, and backups.
- Avoid embedding provider-specific behavior directly in handlers.
- File storage is injected as a `storage.Storage` interface into `ProfileHandler`; S3 vs local is decided at startup in `main.go` based on `STORAGE_TYPE`.
