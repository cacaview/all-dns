# CLAUDE.md

This file provides guidance to Claude Code when working in `web/src/layouts/`.

## Scope

Shared application shell and page layout components live here.

## Expectations

- Keep layouts responsible for navigation chrome, shared framing, and high-level user/session affordances.
- Avoid pushing route-specific data fetching or page business logic into the layout.
- Preserve the current navigation model for dashboard, domains, propagation, notifications, and backups unless the user asks to change it.
