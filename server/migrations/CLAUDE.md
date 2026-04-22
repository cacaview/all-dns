# CLAUDE.md

This file provides guidance to Claude Code when working in `server/migrations/`.

## Scope

Checked-in SQL migration artifacts live here.

## Expectations

- Keep schema changes aligned with the runtime migration behavior used during backend startup.
- Treat these files as persistence history, not a place for application logic.
- When tables or columns change, review the affected models, services, and handlers for consistency.
- Prefer additive, explicit SQL that matches the current GORM-backed data model.
