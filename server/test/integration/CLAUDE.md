# CLAUDE.md

This file provides guidance to Claude Code when working in `server/test/integration/`.

## Scope

Integration-style backend tests live here when they are added.

## Expectations

- Exercise real interactions between routing, services, persistence, and provider-facing flows where practical.
- Favor end-to-end behavior checks over unit-level duplication of service internals.
- Keep test setup explicit so auth, DB state, and provider behavior are easy to reason about.
- Do not weaken production code just to make tests easier to write.
