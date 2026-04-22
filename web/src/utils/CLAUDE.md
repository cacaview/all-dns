# CLAUDE.md

This file provides guidance to Claude Code when working in `web/src/utils/`.

## Scope

Small frontend helper utilities live here.

## Expectations

- Keep helpers narrow and dependency-light.
- Prefer utilities here only when the logic is shared or clearly not owned by a single component/store.
- Avoid turning this directory into a dumping ground for unrelated business logic.
