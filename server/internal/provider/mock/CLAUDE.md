# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/provider/mock/`.

## Scope

In-memory/mock DNS provider behavior for local development lives here.

## Expectations

- Keep this adapter useful for local development and safe demos.
- Preserve compatibility with the shared provider interface.
- Avoid turning mock behavior into a special case elsewhere in the backend.
