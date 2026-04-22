# CLAUDE.md

This file provides guidance to Claude Code when working in `web/src/types/`.

## Scope

Shared TypeScript interfaces and payload shapes live here.

## Expectations

- Keep these types aligned with backend API responses and shared frontend usage.
- Prefer updating shared interfaces here instead of duplicating inline shapes across views or stores.
- When API contracts change, review affected API modules, stores, and views together.
