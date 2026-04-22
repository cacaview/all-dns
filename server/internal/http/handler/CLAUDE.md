# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/http/handler/`.

## Scope

Handlers translate HTTP requests into service calls and shape HTTP responses.

## Expectations

- Keep handlers focused on parameter parsing, auth context lookup, request validation, and response formatting.
- Call services for domain rules and state changes.
- Return clear JSON responses that match existing endpoint conventions (`item`, `items`, `error`, plus any operation-specific fields).
- Do not duplicate business logic that already exists in services.

## New handlers added

- `NewUserHandler` (`user_handler.go`): admin-only user listing and role management.
- `NewProfileHandler` accepts a `storage.Storage` interface; attachments are uploaded through it rather than directly to the filesystem.

## New endpoints

- `PUT /api/v1/users/:id/role` — admin-only role update.
- `PUT /api/v1/accounts/:id/reminder-handled` — mark credential expiry reminder as handled (server-persisted).
