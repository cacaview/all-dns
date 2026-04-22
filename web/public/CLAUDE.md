# CLAUDE.md

This file provides guidance to Claude Code when working in `web/public/`.

## Scope

Static frontend assets that should be served as-is live here.

## Expectations

- Keep files here framework-agnostic and ready for direct serving by Vite.
- Do not put application logic or generated build output in this directory.
- Prefer `src/` for anything that should participate in the Vue build pipeline.
