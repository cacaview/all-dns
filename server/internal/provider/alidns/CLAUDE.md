# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/provider/alidns/`.

## Scope

Alibaba Cloud DNS adapter implementation lives here.

## Expectations

- Keep Alibaba Cloud DNS-specific API details isolated to this package.
- Conform to the shared provider interface and descriptor registration pattern.
- Do not leak Alibaba Cloud-specific request/response logic into generic services or handlers.
