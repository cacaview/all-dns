# CLAUDE.md

This file provides guidance to Claude Code when working in `web/src/api/`.

## Scope

Frontend wrappers around backend HTTP endpoints live here.

## Expectations

- Keep request details centralized here instead of scattering raw Axios calls across views and components.
- Preserve the shared Axios client in `client.ts` as the place for base URL and auth header behavior.
- Keep functions close to backend route shapes and return typed frontend-friendly data.
- Do not move view state management into this directory.

## Key API modules

- `users.ts` — `listUsers()`, `updateUserRole(userId, role)` for admin user management.
- `domains.ts` — `triggerPropagationWatch(domainId, payload, opts)` with `PropagationWatchOptions` for continuous propagation monitoring.
- `accounts.ts` — `setReminderHandled(accountId, handled)` for server-persisted notification state.
