# CLAUDE.md

This file provides guidance to Claude Code when working in `web/src/router/`.

## Scope

Vue Router configuration and navigation guards live here.

## Expectations

- Keep route definitions readable and centered in this package.
- Preserve the pattern where guards call `auth.initialize()` before deciding public/protected navigation.
- Keep redirects and access checks predictable for authenticated and unauthenticated users.
- Avoid burying authorization logic in individual pages when route-level protection is the better fit.
