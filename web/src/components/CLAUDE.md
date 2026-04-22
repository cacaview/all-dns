# CLAUDE.md

This file provides guidance to Claude Code when working in `web/src/components/`.

## Scope

Reusable UI components for the DNS frontend live here.

## Expectations

- Keep components focused on presentation and local interaction logic.
- Prefer props and emitted events over embedding store-specific business rules directly in reusable components.
- Keep Chinese UI copy and Element Plus usage consistent with surrounding screens unless asked otherwise.
- If a component starts becoming route-specific, consider moving the composition up into the owning view.
