# CLAUDE.md

This file provides guidance to Claude Code when working in `server/internal/provider/dnspod/`.

## Scope

Tencent Cloud DNSPod adapter implementation lives here.

## Expectations

- Keep DNSPod-specific API details isolated to this package.
- Conform to the shared provider interface and descriptor registration pattern.
- Do not leak DNSPod-specific request/response logic into generic services or handlers.
