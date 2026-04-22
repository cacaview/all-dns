# CLAUDE.md

This file provides guidance to Claude Code when working in `server/cmd/`.

## Scope

`cmd/` contains executable entrypoints. In this repo that is currently `cmd/api/`.

## Expectations

- Keep entrypoints focused on wiring, startup order, and shutdown behavior.
- Do not move business logic into `main.go`; keep it in `internal/service/` or other internal packages.
- Preserve startup order: load config, connect DB, run migrations, build services, build router, start server/worker.
- Preserve graceful shutdown behavior for the HTTP server and reminder worker.

## When editing here

- Prefer constructor wiring over inline logic.
- Keep logging and fatal startup checks explicit.
- If a new executable is added under `cmd/`, add a sibling `CLAUDE.md` if it needs more specific instructions.
