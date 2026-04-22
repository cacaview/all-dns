# CLAUDE.md

This file provides guidance to Claude Code when working in `web/src/`.

## Scope

This directory contains the frontend application source.

## Expectations

- Keep app bootstrap light in `main.ts` and let routing, stores, and views own their respective concerns.
- Prefer API wrappers in `api/`, shared state in `stores/`, route-level composition in `views/`, and reusable UI in `components/`.
- Keep shared types in `types/` and lightweight helpers in `utils/`.
- Avoid bypassing stores with ad-hoc duplicated fetch logic unless a page-specific case clearly warrants it.

## Notable views

- `PropagationView.vue` — supports continuous propagation monitoring (持续监控) with start/stop toggle and loading state.
