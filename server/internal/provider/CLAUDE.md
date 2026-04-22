# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/provider/`.

## Scope

This package tree owns the DNS provider abstraction, registration, descriptors, and concrete provider adapters.

## Expectations

- Keep the shared contract in `provider.go` stable and provider-agnostic.
- Prefer adding new provider packages over branching on provider names in shared code.
- Keep provider config parsing, validation, and export logic inside each adapter package.
- If UI-facing provider metadata changes, keep descriptors aligned with what the frontend account forms need.
