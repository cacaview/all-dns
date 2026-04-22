# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/provider/digitalocean/`.

## Scope

DigitalOcean DNS adapter implementation lives here.

## Expectations

- Keep DigitalOcean-specific API details isolated to this package.
- Conform to the shared provider interface and descriptor registration pattern.
- Do not leak DigitalOcean-specific request/response logic into generic services or handlers.
