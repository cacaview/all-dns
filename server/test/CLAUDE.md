# CLAUDE.md

This file provides guidance to Claude Code when working in `server/test/`.

## Scope

This directory is reserved for backend test assets and higher-level test suites.

## Expectations

- Keep tests focused on observable backend behavior.
- Prefer realistic integration coverage for database and API flows when tests are added here.
- Keep reusable fixtures and test helpers scoped to testing concerns only.
- Do not place production application logic in this tree.
