# CLAUDE.md

This file provides guidance to Claude Code when working in `web/src/views/`.

## Scope

Route-level pages for the frontend live here.

## Expectations

- Let views compose layouts, stores, and reusable components for each screen.
- Keep long-lived shared state in Pinia stores rather than duplicating it per page.
- Prefer calling backend APIs through `src/api/` or store actions, not inline raw Axios requests.
- Keep page copy and interaction patterns consistent with the current Chinese-language admin UI unless the user asks for a redesign.

## Admin-only pages

- `UsersView.vue` at `/users` — admin user management listing all users with role selectors. Only accessible to users with `role === 'admin'` via router guard.
