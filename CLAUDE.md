# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go run .           # start the server (requires REGISTRY_GITHUB_TOKEN env var)
go test ./...      # run all tests
go test ./internal/handlers/ -run TestName  # run a single test
go build .         # compile binary
```

Releases are cut by tagging via the Makefile:
```bash
make patch   # bump patch version, tag, push
make minor
make major
```

## Required environment variables

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `REGISTRY_GITHUB_TOKEN` | yes | — | Server-side GitHub token with write access to `skpm-dev/registry` |
| `REGISTRY_ADMIN_TOKEN` | no | — | Static bearer token for admin endpoints (yank, remove) |
| `REGISTRY_BASE_URL` | no | `https://registry.skpm.org` | Used when rewriting file download URLs |
| `PORT` | no | `8080` | HTTP listen port |
| `DB_PATH` | no | `skpm.db` | Path to the SQLite database file |

## Architecture

The server is simultaneously the **HTTP API** and the **data store** — package metadata (`packages/<name>.json`) and script files (`files/<name>/<version>/`) live in this Git repo itself, not in a database. The SQLite database (`skpm.db`) holds only download counts.

### Data flow

**Reading packages:** `store.GetPackage` and `store.GetIndex` fetch data at request time from `raw.githubusercontent.com/skpm-dev/registry/main/...`. There is no local cache; every read hits GitHub's CDN.

**Publishing:** `POST /publish` does not write to disk. Instead it uses the server's `REGISTRY_GITHUB_TOKEN` to:
1. Create a branch `publish/<name>-<version>` off `main`
2. Commit the `.sk` files, `packages/<name>.json`, and `index.json` to that branch
3. Open a pull request — a maintainer merges it to make the package live

**Download counting:** `GET /packages/{name}/versions/{version}/files/{filename}` increments a SQLite counter, then redirects to the raw GitHub URL. `GET /packages/{name}` rewrites all file URLs in the response to go through this endpoint (via `rewriteFileURLs`), so every download is counted regardless of client.

**Admin operations** (yank/remove) commit directly to `main` using `REGISTRY_GITHUB_TOKEN` — no PR.

### Package structure

`packages/<name>.json` is the source of truth for metadata. `index.json` is a flat list of `PackageSummary` objects used for listing and search. Both are updated atomically in the publish PR.

A yanked version sets `yanked: true` in the version entry and recalculates `latest` to the highest non-yanked semver. A removed package writes a tombstone (sets `removed: true`) and removes itself from `index.json` — callers get `410 Gone`.

### Internal packages

| Package | Responsibility |
|---|---|
| `internal/server` | Route registration, rate limiting middleware (10 req/min publish, 120/min global), CORS |
| `internal/handlers` | HTTP handlers — thin layer that validates, calls store/github, writes JSON |
| `internal/store` | Business logic: building package entries, updating the index, download counts, SQLite init |
| `internal/github` | GitHub API client — branch ops, file commits, PRs (`RepoClient`), plus user auth token extraction |
| `internal/models` | Shared types: `Package`, `VersionEntry`, `FileEntry`, `PackageSummary` |
| `internal/middleware` | Token-bucket rate limiter keyed by IP |
