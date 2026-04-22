# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/provider/cloudflare/`.

## Scope

Cloudflare DNS adapter implementation lives here.

## Expectations

- Keep Cloudflare-specific API details isolated to this package.
- Conform to the shared provider interface and descriptor registration pattern.
- Do not leak Cloudflare-specific request/response logic into generic services or handlers.
