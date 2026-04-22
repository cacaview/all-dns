# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/provider/aws/`.

## Scope

AWS Route53 adapter implementation lives here.

## Expectations

- Keep Route53-specific API details isolated to this package.
- Conform to the shared provider interface and descriptor registration pattern.
- Do not leak Route53-specific request/response logic into generic services or handlers.
