# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/db/`.

## Scope

Database connection setup and migration bootstrapping live here.

## Expectations

- Keep connection creation and migration orchestration centralized here.
- Preserve startup migration behavior used by `cmd/api/main.go`.
- Keep DB wiring separate from business logic and request handling.
- If schema changes are needed, make sure runtime migration behavior and checked-in migration artifacts stay coherent.

## Migrations

AutoMigrate includes `Organization`, `OrgMember`, and `ReminderAck` in addition to the existing models. The `Account` model carries an `org_id` column for multi-tenant isolation.
