# 0001 — Foundation stack and task runner

**Status:** Accepted
**Date:** 2026-11-13
**Phase:** 0

## Context

The repository started empty apart from two planning documents. The engineering plan recommends a Go backend with `net/http` and a React + TypeScript + Vite frontend, and calls for a `Makefile` as the task runner.

The development machine is Windows 11 with Node 24 and npm 11 installed, but no Go toolchain and no `make`.

## Decision

### Backend language: Go

Go is installed via `winget install GoLang.Go` and the backend is built as specified. The alternative — a single-language TypeScript monorepo — would have been faster to start but abandons the plan's stated stack for a convenience reason rather than a technical one.

Go's standard library covers the HTTP server, CSV parsing, and testing without dependencies, which matters for a project whose whole value proposition is predictable, inspectable behavior.

### Task runner: npm scripts as primary, Makefile as a mirror

`npm run <task>` is the entry point. It works on every platform the project supports and needs no additional tooling.

The `Makefile` delegates to the same npm scripts rather than duplicating command lines, so the two cannot drift. It exists for contributors who expect `make dev` and for CI parity, and it is not required to build or run the project.

### Frontend tooling

- Vite for the dev server and build.
- Tailwind CSS v4 with the `@tailwindcss/vite` plugin; design tokens live in CSS custom properties and are mapped into Tailwind's theme, so a token change is a one-line change.
- TanStack Query for server state. Added in Phase 0 because the shell already performs a real request and every later feature reads server data; the provider belongs in the app shell.
- Lucide for icons — one library, as the design plan requires.

### Direct API base URL, CORS enabled

The frontend calls `http://127.0.0.1:8787` directly, configured by `VITE_API_BASE_URL`, and the API allows the Vite origin via CORS.

A Vite dev proxy would also work and would remove the CORS surface. Direct calls were chosen because the failure mode is legible: if the API is down, the browser reports a connection error against a URL the user can see and check, rather than a 500 from the proxy.

## Consequences

- Contributors need Go 1.24+ and Node 20+; both are documented in the README.
- `make` remains optional, so a missing `make` never blocks a Windows contributor.
- Two entry points exist, but only one contains the commands.
- If the CORS policy becomes a source of bugs, switching to a Vite proxy is a contained change: one config block and one default value.
