# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/service/`.

## Scope

Backend business logic and orchestration live here.

## Expectations

- Keep this layer as the main home for domain rules and workflows.
- Preserve account credential encryption, validate-then-sync behavior, backup creation before DNS mutations, and propagation checks after writes/restores.
- Keep provider selection behind the shared provider abstraction.
- Let handlers pass inputs in and responses out; do not move HTTP concerns into services.
- Avoid creating repository-style indirection unless it clearly reduces duplication or complexity.

## Multi-tenancy

All data access is scoped by `org_id` via `getUserOrgID()`. Accounts, domains, and reminders are isolated per organization.

## Key services

- `AuthService` — `ListUsers`, `UpdateUserRole` for RBAC user management; first user becomes admin and creates a default organization.
- `DNSService` — all queries (`ListAccounts`, `CreateAccount`, `ListDomainsWithOptions`, `GetDashboardSummary`, `ListReminders`) scope by `org_id`; exposes `SetReminderHandled` and `TriggerPropagationCheckWithOptions` with `WatchOptions` for continuous polling.
- `PropagationService` — `Watch()` supports continuous polling with configurable resolvers, interval, and max attempts.
- `ReminderService` — `GetReminderAcks(userID)` and `SetReminderHandled(userID, accountID, handled)` for server-persisted notification state via `ReminderAck`.
