# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/provider/gcp/`.

## Scope

Google Cloud DNS adapter implementation lives here.

## Expectations

- Keep Google Cloud DNS-specific API details isolated to this package.
- Conform to the shared provider interface and descriptor registration pattern.
- Do not leak Google Cloud-specific request/response logic into generic services or handlers.
