# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository overview

This is a small monorepo for a DNS management app with three runtime pieces wired together by `docker-compose.yml`:

- PostgreSQL database
- Go backend API in `server/`
- Vue 3 frontend in `web/`

There was no existing root `CLAUDE.md`, `README.md`, Cursor rules, or Copilot instructions at the time this file was created.

## Directory-local CLAUDE files

Source directories in this repo should also carry their own `CLAUDE.md` files.

- Keep root guidance focused on cross-repo conventions and architecture.
- Put directory-specific constraints next to the code they govern.
- Skip generated, dependency, and transient directories such as `web/node_modules`, `web/dist`, `server/tmp`, `server/uploads`, and `.git`.

## Common commands

### Full stack with Docker Compose

From the repo root:

- Start everything: `docker compose up --build`
- Start in background: `docker compose up -d --build`
- Stop services: `docker compose down`
- View logs: `docker compose logs -f server`
- Rebuild one service: `docker compose build server`

Services exposed by compose:

- Postgres: `localhost:5432`
- API: `http://localhost:8080`
- Web: `http://localhost:5173`

### Backend (`server/`)

- Run tests: `go test ./...`
- Run a single test: `go test ./path/to/package -run TestName`
- Run the API directly: `go run ./cmd/api`
- Build the API binary: `go build ./cmd/api`

For containerized local development, the backend Dockerfile runs Air with `server/.air.toml`, which rebuilds `./cmd/api` into `server/tmp/api`.

### Frontend (`web/`)

- Install deps: `npm install`
- Start dev server: `npm run dev`
- Production build: `npm run build`
- Preview production build: `npm run preview`

Notes:

- `npm run build` already runs `vue-tsc --noEmit` before `vite build`.
- There is currently no frontend test script or lint script in `web/package.json`.

## Configuration

Sample environment values live in `.env.example`.

Backend config is loaded from environment variables in `server/internal/config/config.go`.

Important variables:

- `APP_MASTER_KEY` is required and must be base64 that decodes to exactly 32 bytes.
- `JWT_SECRET` is required.
- `FRONTEND_URL` controls backend CORS.
- `DEV_LOGIN_ENABLED` enables the dev login endpoint.
- GitHub/GitLab OAuth is only enabled when the corresponding client ID, secret, and redirect URL are all set.
- Compose injects backend DB settings and frontend Vite API settings.

## High-level architecture

### Backend

The backend entrypoint is `server/cmd/api/main.go`.

Startup flow:

1. Load env config.
2. Connect to Postgres.
3. Run app migrations with `appdb.Migrate(...)`.
4. Build core services.
5. Register enabled OAuth providers.
6. Build the Gin router.
7. Start the HTTP server and the reminder background worker.

Backend stack:

- Gin for HTTP routing/middleware
- GORM for persistence
- PostgreSQL as the primary datastore
- JWT access/refresh tokens for auth
- GitHub/GitLab OAuth for login
- Provider adapters for DNS vendors
- DNS propagation checks and snapshot-based restore flows

Layering under `server/internal/` is important:

- `config/` - env/config loading
- `db/` - DB connection and migration setup
- `http/handler/` - request/response handlers
- `http/middleware/` - auth and RBAC middleware
- `service/` - business logic
- `provider/` - DNS provider abstraction and implementations
- `model/` - GORM models
- `oauth/` - OAuth provider integrations
- `notifier/` - reminder webhook delivery

The backend is intentionally service-driven: handlers should stay thin and delegate business rules to services.

### API shape

The API root is `/api/v1`.

Important route groups in `server/internal/http/router.go`:

- `GET /health`
- `/api/v1/auth/*` for OAuth, refresh, dev login, logout, and current-user lookup
- `/api/v1/dashboard/*` for summary data
- `/api/v1/accounts/*` for provider accounts and reminders
- `/api/v1/domains/*` for domain listing, records, backups, propagation checks, archive/star/tag actions, and profile updates

Protected routes use JWT auth middleware. Mutating account/domain routes are further restricted by RBAC to `admin` and `editor` roles.

### Provider model

The key abstraction is `server/internal/provider/provider.go`.

`DNSService` selects a provider adapter from the account's stored provider name, decrypts the stored provider config, and then uses a shared interface for:

- account validation
- domain sync
- record listing
- record upsert/delete

Current provider implementations are:

- `mock`
- `cloudflare`

If you add another DNS provider, follow the existing adapter pattern instead of branching provider-specific logic through handlers.

### Data and mutation workflow

`server/internal/service/dns_service.go` is the core business service.

Important behavior to preserve:

- Account credentials are encrypted at rest before storing them.
- Domain sync happens after successful account validation.
- Record mutations create a backup snapshot before writing.
- Record upserts and restore flows trigger propagation checks that are persisted back onto the domain.
- Domain/profile/tag/archive/star state is stored in the database; provider records remain the source of truth for live DNS records.

### Frontend

The frontend is a Vue 3 SPA in `web/` using:

- Vite
- TypeScript
- Vue Router
- Pinia
- Element Plus
- Axios

Important frontend structure:

- `src/api/` - HTTP client and API modules
- `src/stores/` - Pinia stores for auth and domain/dashboard state
- `src/views/` - route-level pages
- `src/components/` - domain/account UI pieces
- `src/layouts/` - app shell layout

Frontend entrypoints:

- App bootstrap: `web/src/main.ts`
- Router: `web/src/router/index.ts`
- Vite config: `web/vite.config.ts`

### Frontend auth flow

Auth behavior is centered in `web/src/stores/auth.ts`.

Important details:

- Access and refresh tokens are stored in `localStorage`.
- OAuth login returns to the frontend as a URL hash containing tokens.
- The auth store consumes the hash, stores tokens, and clears the hash from browser history.
- Route guards always call `auth.initialize()` before deciding whether a route is public or protected.

### Frontend data flow

There is no separate query/cache library. Data fetching is store-driven.

- `auth` store owns session bootstrap, refresh, and logout.
- `domains` store owns dashboard summary, accounts, reminders, domain list, and search state.
- Axios uses `VITE_API_BASE_URL` or `/api/v1` as the base URL.
- Vite dev server proxies `/api` to `VITE_BACKEND_ORIGIN` or `http://server:8080`.

## Repo-specific notes

- UI copy is largely Chinese-facing, even though most identifiers are English.
- There are currently no committed backend `*_test.go` files and no frontend `*.test.*`/`*.spec.*` files.
- The SQL migration file in `server/migrations/` exists, but the main runtime migration path is the Go migration code executed during backend startup.
- Avoid assuming archived-domain features are unused: the backend already exposes archive endpoints and archive-aware domain listing.
