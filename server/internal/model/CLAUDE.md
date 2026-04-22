# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/model/`.

## Scope

GORM models and persisted backend data shapes live here.

## Expectations

- Keep model changes aligned with actual persistence needs.
- Prefer explicit fields over clever serialization unless the existing model already uses JSON columns.
- Be careful with changes that affect backups, domain metadata, account credential state, and auth/session data.
- If a model change affects query behavior or API output, review the corresponding service and handler layers too.

## Key models

- `User` — includes `PrimaryOrgID` for multi-tenant support.
- `Organization` + `OrgMember` — implement the multi-tenant isolation model; accounts and domains are scoped by `org_id`.
- `Account` — credentials are encrypted at rest; includes `org_id` and `user_id`.
- `ReminderAck` — tracks per-user, per-account reminder handled state (server-persisted, not browser-local).
