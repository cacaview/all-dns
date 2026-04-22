# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/`.

## Scope

`internal/` contains the backend implementation layers.

## Layer boundaries

- `config/` reads environment and normalizes configuration.
- `db/` owns database connection and migration bootstrapping.
- `http/` owns transport concerns only.
- `service/` owns backend business rules and orchestration.
- `provider/` owns external DNS provider abstractions and adapters.
- `model/` defines persisted shapes.
- `oauth/` integrates external OAuth providers.
- `notifier/` delivers reminder notifications.

## Expectations

- Preserve the service-driven architecture.
- Avoid circular dependencies between internal packages.
- Keep transport/request concerns out of `service/` where possible.
- Keep provider-specific logic inside `provider/` adapters, not spread across unrelated packages.
- Prefer extending an existing layer over introducing a new abstraction unless the current structure clearly cannot support the change.
