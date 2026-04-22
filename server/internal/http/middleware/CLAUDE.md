# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/http/middleware/`.

## Scope

HTTP middleware for authentication and RBAC lives here.

## Expectations

- Keep middleware focused on cross-cutting request concerns.
- Preserve JWT-based auth flow used by protected routes.
- Preserve RBAC restrictions for mutating account and domain routes.
- Keep user extraction/context propagation predictable for handlers.
