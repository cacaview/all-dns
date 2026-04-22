# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/config/`.

## Scope

Configuration loading and validation for the backend lives here.

## Expectations

- Keep this package environment-driven.
- Validate boundary requirements here, not deep inside services.
- Preserve required config invariants such as:
  - `APP_MASTER_KEY` must decode to exactly 32 bytes.
  - `JWT_SECRET` is required (no development default).
  - OAuth only enables when all required values are present.
  - `DEV_LOGIN_ENABLED` defaults to `"false"`.
- Prefer returning normalized config objects rather than scattering env access throughout the codebase.

## Config additions

- `PropagationResolvers` — comma-separated list of DNS resolvers for propagation checks; defaults to Cloudflare, Google, Quad9.
- `StorageType` — `"local"` or `"s3"`; controls which storage backend is constructed at startup.
- `S3Config` fields: `Bucket`, `Region`, `Endpoint`, `AccessKeyID`, `SecretAccessKey`, `PathPrefix` for S3-compatible object storage.
