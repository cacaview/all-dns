# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/oauth/`.

## Scope

OAuth provider integrations for backend login live here.

## Expectations

- Keep provider-specific OAuth details inside this package.
- Preserve the pattern where providers are enabled only when config is complete.
- Keep callback/login URL generation and user info mapping provider-specific.
- Avoid leaking OAuth provider branching into generic auth handlers or unrelated services.
