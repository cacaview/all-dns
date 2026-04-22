# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/provider/hetzner/`.

## Scope

Hetzner DNS adapter implementation lives here.

## Expectations

- Keep Hetzner-specific API details isolated to this package.
- Conform to the shared provider interface and descriptor registration pattern.
- Do not leak Hetzner-specific request/response logic into generic services or handlers.
