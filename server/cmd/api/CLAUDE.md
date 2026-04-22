# CLAUDE.md

This file provides guidance to Claude Code when working in `server/cmd/api/`.

## Scope

This directory owns the backend process entrypoint in `main.go`.

## Responsibilities

`main.go` should only:

- load config
- connect to Postgres
- run migrations
- construct services and OAuth providers
- construct file storage (local or S3 depending on `STORAGE_TYPE`)
- build the router
- start the HTTP server
- start and stop the reminder worker with process lifecycle

## Keep out of this directory

- request handling details
- account/domain business rules
- provider-specific branching
- persistence query logic

Those belong in `internal/http/`, `internal/service/`, `internal/provider/`, or `internal/db/`.
