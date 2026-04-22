# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/notifier/`.

## Scope

Reminder notification delivery integrations live here.

## Expectations

- Keep this package focused on outbound reminder delivery.
- Preserve simple integration boundaries so reminder scheduling/orchestration stays in services.
- Treat remote webhook failures as integration concerns, not reasons to move reminder business logic into this package.
