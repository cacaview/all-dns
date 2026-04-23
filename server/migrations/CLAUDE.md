# CLAUDE.md

This file provides guidance to Claude Code when working in `server/migrations/`.

## Scope

Checked-in SQL migration artifacts live here.

## Expectations

- The root `migrations/` directory contains the legacy `0001_init.sql` (reference only — not executed at startup).
- The canonical migration path is `internal/db/versioned_migrate.go` using `embed.FS` from `internal/db/migrations/sql/`.
- New schema migrations go into `internal/db/migrations/sql/` as `000XXX_description.sql` in lexical version order.
- Do not edit already-applied migrations in a running system; create a new migration file instead.
