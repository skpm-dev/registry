# skpm Registry

**[skpm.org](https://skpm.org)** — the package manager for Skript.

> The HTTP API and GitHub-backed data store powering the skpm ecosystem.

Package metadata and script files live in this repository as version-controlled JSON, accessed via `raw.githubusercontent.com`. Download counts are stored in Postgres. The registry itself holds no persistent state beyond those two sources.

---

## API

**Base URL:** `https://registry.skpm.org`

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/packages` | Full package index |
| `GET` | `/packages/:name` | Metadata for one package (all versions) |
| `GET` | `/packages/:name/versions/:version/files/:file` | Download a script file (counts the download, redirects to raw GitHub) |
| `GET` | `/search?q=` | Search packages by name or description |
| `POST` | `/publish` | Publish a new package or version (requires `Authorization: Bearer <GitHub PAT>`) |

**Rate limits:** 10 req/min per IP on `POST /publish`; 120 req/min per IP globally.

---

## Integration API

Third-party services can publish packages and fetch file contents via a dedicated API. All integration endpoints require an API key.

To get an API key, open an issue on this repository.

### Authentication

Include your API key in every request:

```
Authorization: Bearer <api-key>
```

### Endpoints

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/v1/packages` | Publish a package (see below) |
| `GET` | `/v1/packages/:name/versions/:version/files/:file` | Fetch raw file content inline |

### Publish a package

`POST /v1/packages`

Requires two headers:
- `Authorization: Bearer <api-key>` — your integration API key
- `X-GitHub-Token: <github-pat>` — a GitHub PAT with `read:user` scope, used to verify your identity as the package author

```json
{
  "manifest": {
    "name": "my-package",
    "version": "1.0.0",
    "description": "A short description",
    "skript": "2.9",
    "minecraft": "1.21"
  },
  "files": {
    "my-package.sk": "# script content here"
  }
}
```

Returns `201` with a pull request URL on success. The package goes live once a maintainer merges the PR.

### Fetch file content

`GET /v1/packages/:name/versions/:version/files/:file`

Returns the raw script file content as `text/plain`. Also counts the download.

---

## Package format

Packages are stored as `packages/<name>.json` in this repository:

```json
{
  "name": "economy",
  "description": "A Vault-backed economy system",
  "author": "adammcgrogan",
  "latest": "1.0.1",
  "versions": {
    "1.0.1": {
      "skript": ">=2.8.0",
      "minecraft": ">=1.20",
      "addons": {},
      "dependencies": {},
      "files": [
        {
          "name": "economy.sk",
          "url": "https://registry.skpm.org/packages/economy/versions/1.0.1/files/economy.sk",
          "sha256": "sha256:a1b2c3..."
        }
      ]
    }
  }
}
```

Script files live at `files/<name>/<version>/<filename>`. A flat `index.json` at the repo root powers search and listing.

---

## Publish flow

1. The [skpm CLI](https://github.com/skpm-dev/cli) sends `POST /publish` with the manifest and file contents
2. The registry validates the request (name format, semver, ownership, duplicate version check)
3. A branch `publish/<name>-<version>` is created and script files + package metadata are committed to it
4. A pull request is opened — a maintainer reviews and merges it
5. The package is live once merged

Ownership is enforced server-side: the GitHub identity behind the bearer token must match the stored author for all versions after the first.

---

## Repository structure

```
registry/
├── index.json                   ← flat index of all packages (rebuilt post-merge)
├── packages/
│   ├── economy.json
│   └── join-message.json
└── files/
    ├── economy/
    │   └── 1.0.1/
    │       └── economy.sk
    └── join-message/
        └── 1.0.0/
            └── join-message.sk
```

---

## Related

- **[skpm-dev/cli](https://github.com/skpm-dev/cli)** — CLI tool for publishing packages
- **[skpm-dev/plugin](https://github.com/skpm-dev/plugin)** — Bukkit plugin for installing packages in-game
