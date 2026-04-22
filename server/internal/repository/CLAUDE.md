# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/repository/`.

## Scope

This directory is reserved for persistence-focused repository code if the backend grows beyond direct service/GORM queries.

## Expectations

- Prefer keeping query code in services unless extracting a repository clearly reduces duplication or complexity.
- If repository code is added, keep it focused on persistence concerns only.
- Do not move business rules here.
