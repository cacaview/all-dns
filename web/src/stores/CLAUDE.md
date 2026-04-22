# CLAUDE.md

This file provides guidance to Claude Code when working in `web/src/stores/`.

## Scope

Pinia stores for auth, dashboard, domains, backups, and reminders live here.

## Expectations

- Keep stores as the main home for frontend stateful workflows and coordinated data loading.
- Preserve token/session lifecycle behavior in `auth.ts`, including OAuth hash consumption, localStorage persistence, refresh, and logout.
- Preserve reminder normalization and dashboard/domain hydration behavior in `domains.ts` unless the user asks for a flow change.
- Keep UI-only formatting helpers small and colocated only when they clearly support store behavior.

## Key stores

- `domains.ts` — `setReminderHandled` is async with optimistic update; `fetchDomainRecords(domainId)` fetches DNS records; `domainLastPropagationStatus(domainId, status)` persists propagation check result.
