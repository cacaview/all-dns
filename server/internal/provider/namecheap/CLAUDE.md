# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/provider/namecheap/`.

## Scope

Namecheap DNS adapter implementation lives here.

## Expectations

- Keep Namecheap-specific API details isolated to this package.
- Conform to the shared provider interface and descriptor registration pattern.
- Do not leak Namecheap-specific request/response logic into generic services or handlers.
