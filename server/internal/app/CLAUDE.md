# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/app/`.

## Scope

This directory is reserved for backend app-level composition helpers if they are introduced.

## Expectations

- Keep code here focused on app assembly and cross-service wiring.
- Do not move transport handlers, provider adapters, or domain business rules here.
- Prefer existing packages under `internal/service/`, `internal/http/`, `internal/db/`, and `internal/provider/` unless an app-level composition concern clearly emerges.
