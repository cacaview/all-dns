# CLAUDE.md

This file provides guidance to Claude Code when working in `web/`.

## Scope

These instructions apply to the Vue 3 frontend SPA under `web/`.

## Commands

Run commands from `web/` unless otherwise noted.

- Install dependencies: `npm install`
- Start dev server: `npm run dev`
- Build for production: `npm run build`
- Preview production build: `npm run preview`

## Frontend structure

Important directories:

- `public/` - static assets served as-is
- `src/api/` - backend API wrappers
- `src/components/` - reusable UI components
- `src/layouts/` - app shell and shared page layout
- `src/router/` - route definitions and guards
- `src/stores/` - Pinia stores for auth and domain/dashboard state
- `src/types/` - shared TypeScript shapes
- `src/utils/` - small frontend helpers
- `src/views/` - route-level pages

## Expectations

- Preserve the store-driven data flow.
- Keep route guards aligned with `auth.initialize()` before protected-route decisions.
- Keep backend calls centralized through `src/api/`.
- UI copy can remain Chinese-facing unless the user asks to change it.
- Do not treat `node_modules/` or `dist/` as source directories.
